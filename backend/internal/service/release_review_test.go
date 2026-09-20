package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/config"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func releaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}, &model.Furnace{}, &model.Heat{},
		&model.ChemicalSample{}, &model.QualityDecision{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	// Serialize writers like PostgreSQL row locks would.
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	return db
}

func newReleaseStack(t *testing.T) (*gorm.DB, ReleaseReviewService, repository.HeatRepository, repository.ChemicalSampleRepository, repository.QualityDecisionRepository) {
	t.Helper()
	db := releaseTestDB(t)
	heatRepository := repository.NewHeatRepository(db)
	sampleRepository := repository.NewChemicalSampleRepository(db)
	decisionRepository := repository.NewQualityDecisionRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{AppName: "test", JWTSecret: strings.Repeat("b", 32)})
	return db, NewReleaseReviewService(heatRepository, sampleRepository, decisionRepository, security), heatRepository, sampleRepository, decisionRepository
}

func releaseSample(code, heatCode string, carbon, silicon, sulfur, phosphorus float64, status string) model.ChemicalSample {
	sample := workflowSample(code, heatCode)
	sample.CarbonPct = carbon
	sample.SiliconPct = silicon
	sample.SulfurPct = sulfur
	sample.PhosphorusPct = phosphorus
	sample.Status = status
	return sample
}

func TestReleasePanelAcceptanceLocksSamplesHeatDecisionAndAuditsAtomically(t *testing.T) {
	db, reviews, heatRepository, sampleRepository, decisionRepository := newReleaseStack(t)
	ctx := context.Background()
	heat := workflowHeat("H-REL-01")
	if err := db.Create(&heat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	samples := []model.ChemicalSample{
		releaseSample("SR-01A", heat.Code, 3.30, 2.00, 0.04, 0.07, "verified"),
		releaseSample("SR-01B", heat.Code, 3.34, 2.03, 0.045, 0.075, "verified"),
	}
	if err := db.Create(&samples).Error; err != nil {
		t.Fatalf("seed samples: %v", err)
	}

	panel, err := reviews.GetPanel(ctx, heat.Code)
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if !panel.CanAccept || panel.PairStatus != PairStatusReady {
		t.Fatalf("expected ready pair, blockers=%v", panel.Blockers)
	}
	if !withinTolerance(panel.CarbonDelta, 0.04) || !withinTolerance(panel.SiliconDelta, 0.03) {
		t.Fatalf("unexpected deltas: dC=%v dSi=%v", panel.CarbonDelta, panel.SiliconDelta)
	}

	decision, err := reviews.Adjudicate(ctx, dto.ReleaseAdjudicationRequest{
		HeatCode: heat.Code, Decision: "accept", Reason: "双样本复核合格准予放行", Evidence: "QMS-signed-release",
	}, "reviewer", "req-release")
	if err != nil {
		t.Fatalf("adjudicate accept: %v", err)
	}
	if decision.Status != "accept" || decision.PairedSampleCode == "" ||
		(decision.SampleCode != "SR-01A" && decision.PairedSampleCode != "SR-01A") {
		t.Fatalf("unexpected decision: %#v", decision)
	}

	storedHeat, err := heatRepository.GetByCode(ctx, heat.Code)
	if err != nil || storedHeat.Status != "accepted" || storedHeat.Version != 2 {
		t.Fatalf("heat not accepted atomically: %#v err=%v", storedHeat, err)
	}
	for _, code := range []string{"SR-01A", "SR-01B"} {
		locked, err := sampleRepository.GetByCode(ctx, code)
		if err != nil || locked.Status != "locked" {
			t.Fatalf("sample %s not locked: %#v err=%v", code, locked, err)
		}
	}
	final, err := decisionRepository.GetByHeatCode(ctx, heat.Code)
	if err != nil || final.Status != "accept" {
		t.Fatalf("decision not persisted: %#v err=%v", final, err)
	}
	var audits []model.AuditLog
	if err := db.Where("request_id = ?", "req-release").Order("id").Find(&audits).Error; err != nil {
		t.Fatalf("load audits: %v", err)
	}
	wantActions := map[string]int{"release": 2, "lock": 2}
	gotActions := map[string]int{}
	for _, entry := range audits {
		gotActions[entry.Action]++
	}
	if len(audits) != 4 || gotActions["release"] != wantActions["release"] || gotActions["lock"] != wantActions["lock"] {
		t.Fatalf("expected 2 release + 2 lock audits, got %#v", gotActions)
	}

	// Post-decision panel reflects the already-adjudged state consistently.
	again, err := reviews.GetPanel(ctx, heat.Code)
	if err != nil || again.PairStatus != PairStatusAdjudged || again.Decision == nil || again.CanAccept {
		t.Fatalf("panel not adjudged after verdict: %#v err=%v", again, err)
	}
}

func TestReleasePanelBlocksOnSampleShortage(t *testing.T) {
	db, reviews, _, _, _ := newReleaseStack(t)
	ctx := context.Background()
	heat := workflowHeat("H-REL-02")
	if err := db.Create(&heat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	shortSample := releaseSample("SR-02A", heat.Code, 3.3, 2.0, 0.04, 0.07, "verified")
	if err := db.Create(&shortSample).Error; err != nil {
		t.Fatalf("seed sample: %v", err)
	}
	panel, err := reviews.GetPanel(ctx, heat.Code)
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if panel.CanAccept || panel.PairStatus != PairStatusShort || len(panel.Blockers) == 0 {
		t.Fatalf("expected shortage block: %#v", panel)
	}
	if _, err := reviews.Adjudicate(ctx, dto.ReleaseAdjudicationRequest{
		HeatCode: heat.Code, Decision: "accept", Reason: "should be blocked", Evidence: "EVID-1",
	}, "reviewer", "req-block-short"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected blocked acceptance, got %v", err)
	}
	// Return/scrap is allowed with a documented reason even without a pair.
	decision, err := reviews.Adjudicate(ctx, dto.ReleaseAdjudicationRequest{
		HeatCode: heat.Code, Decision: "remelt", Reason: "第二份复核样本缺失，先返炉重炼", Evidence: "EVID-2",
	}, "reviewer", "req-remelt")
	if err != nil {
		t.Fatalf("remelt without pair should succeed with reason: %v", err)
	}
	if decision.SampleCode != "SR-02A" || decision.PairedSampleCode != "" {
		t.Fatalf("unexpected remelt evidence: %#v", decision)
	}
	stored, err := reviews.GetPanel(ctx, heat.Code)
	if err != nil || stored.PairStatus != PairStatusAdjudged {
		t.Fatalf("panel should show adjudged: %#v err=%v", stored, err)
	}
}

func TestReleasePanelBlocksOnChemistryAndDeltas(t *testing.T) {
	db, reviews, _, _, _ := newReleaseStack(t)
	ctx := context.Background()

	outOfSpec := workflowHeat("H-REL-03")
	if err := db.Create(&outOfSpec).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	bad := []model.ChemicalSample{
		releaseSample("SR-03A", outOfSpec.Code, 3.30, 2.00, 0.04, 0.07, "verified"),
		releaseSample("SR-03B", outOfSpec.Code, 3.31, 2.01, 0.095, 0.07, "verified"), // sulfur above 0.08
	}
	if err := db.Create(&bad).Error; err != nil {
		t.Fatalf("seed samples: %v", err)
	}
	panel, err := reviews.GetPanel(ctx, outOfSpec.Code)
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if panel.CanAccept || !strings.Contains(strings.Join(panel.Blockers, ";"), "超出牌号") {
		t.Fatalf("expected out-of-spec blocker, %#v", panel.Blockers)
	}

	deltaHeat := workflowHeat("H-REL-04")
	if err := db.Create(&deltaHeat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	diff := []model.ChemicalSample{
		releaseSample("SR-04A", deltaHeat.Code, 3.30, 2.00, 0.04, 0.07, "verified"),
		releaseSample("SR-04B", deltaHeat.Code, 3.36, 1.94, 0.04, 0.07, "verified"), // ΔC=0.06, ΔSi=0.06
	}
	if err := db.Create(&diff).Error; err != nil {
		t.Fatalf("seed samples: %v", err)
	}
	panel, err = reviews.GetPanel(ctx, deltaHeat.Code)
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if panel.CanAccept || len(panel.Blockers) != 2 {
		t.Fatalf("expected two delta blockers, %#v", panel.Blockers)
	}
	if _, err := reviews.Adjudicate(ctx, dto.ReleaseAdjudicationRequest{
		HeatCode: deltaHeat.Code, Decision: "accept", Reason: "强行放行", Evidence: "EVID-3",
	}, "reviewer", "req-block-delta"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected delta block, got %v", err)
	}
}

func TestReleaseCarbonToleranceBoundary(t *testing.T) {
	db, reviews, _, _, _ := newReleaseStack(t)
	ctx := context.Background()
	heat := workflowHeat("H-REL-05")
	if err := db.Create(&heat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	samples := []model.ChemicalSample{
		releaseSample("SR-05A", heat.Code, 3.30, 2.00, 0.04, 0.07, "verified"),
		releaseSample("SR-05B", heat.Code, 3.35, 2.05, 0.04, 0.07, "verified"), // exactly at 0.05
	}
	if err := db.Create(&samples).Error; err != nil {
		t.Fatalf("seed samples: %v", err)
	}
	panel, err := reviews.GetPanel(ctx, heat.Code)
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if !panel.CanAccept || len(panel.Blockers) != 0 {
		t.Fatalf("0.05 boundary must pass, blockers=%v", panel.Blockers)
	}
}

func TestReleaseDuplicateAndConcurrentAdjudicationsSucceedOnce(t *testing.T) {
	db, reviews, heatRepository, sampleRepository, _ := newReleaseStack(t)
	ctx := context.Background()
	heat := workflowHeat("H-REL-06")
	if err := db.Create(&heat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	samples := []model.ChemicalSample{
		releaseSample("SR-06A", heat.Code, 3.30, 2.00, 0.04, 0.07, "verified"),
		releaseSample("SR-06B", heat.Code, 3.32, 2.02, 0.04, 0.07, "verified"),
	}
	if err := db.Create(&samples).Error; err != nil {
		t.Fatalf("seed samples: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, results[index] = reviews.Adjudicate(ctx, dto.ReleaseAdjudicationRequest{
				HeatCode: heat.Code, Decision: "accept", Reason: "并发合议判定", Evidence: "EVID-CONC",
			}, fmt.Sprintf("reviewer-%d", index), fmt.Sprintf("req-conc-%d", index))
		}(i)
	}
	wg.Wait()

	successes, conflicts := 0, 0
	for _, result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected adjudication error: %v", result)
		}
	}
	if successes != 1 || conflicts != 4 {
		t.Fatalf("expected exactly 1 success and 4 conflicts, got %d/%d", successes, conflicts)
	}

	var decisionCount int64
	if err := db.Model(&model.QualityDecision{}).Where("heat_code = ?", heat.Code).Count(&decisionCount).Error; err != nil {
		t.Fatalf("count decisions: %v", err)
	}
	if decisionCount != 1 {
		t.Fatalf("expected one decision row, got %d", decisionCount)
	}
	storedHeat, err := heatRepository.GetByCode(ctx, heat.Code)
	if err != nil || storedHeat.Status != "accepted" || storedHeat.Version != 2 {
		t.Fatalf("heat over-updated by concurrent requests: %#v", storedHeat)
	}
	for _, code := range []string{"SR-06A", "SR-06B"} {
		locked, err := sampleRepository.GetByCode(ctx, code)
		if err != nil || locked.Status != "locked" || locked.Version != 2 {
			t.Fatalf("sample lock raced: %#v err=%v", locked, err)
		}
	}

	// A direct repeat after the winner commits is a clean duplicate rejection.
	if _, err := reviews.Adjudicate(ctx, dto.ReleaseAdjudicationRequest{
		HeatCode: heat.Code, Decision: "scrap", Reason: "重复判定尝试", Evidence: "EVID-DUP",
	}, "reviewer", "req-dup"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected duplicate conflict, got %v", err)
	}
}

func TestReleaseAuditFailureRollsBackEntireAdjudication(t *testing.T) {
	db, reviews, _, _, decisionRepository := newReleaseStack(t)
	ctx := context.Background()
	heat := workflowHeat("H-REL-07")
	if err := db.Create(&heat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	samples := []model.ChemicalSample{
		releaseSample("SR-07A", heat.Code, 3.30, 2.00, 0.04, 0.07, "verified"),
		releaseSample("SR-07B", heat.Code, 3.33, 2.02, 0.04, 0.07, "verified"),
	}
	if err := db.Create(&samples).Error; err != nil {
		t.Fatalf("seed samples: %v", err)
	}
	if err := db.Migrator().DropTable(&model.AuditLog{}); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	if _, err := reviews.Adjudicate(ctx, dto.ReleaseAdjudicationRequest{
		HeatCode: heat.Code, Decision: "accept", Reason: "审计失败时必须整体回滚", Evidence: "EVID-ROLLBACK",
	}, "reviewer", "req-release-rollback"); err == nil {
		t.Fatal("expected adjudication to fail when audit table is missing")
	}
	storedHeat, err := db_modelHeat(t, db, heat.Code)
	if err != nil {
		t.Fatalf("reload heat: %v", err)
	}
	if storedHeat.Status != "hold" || storedHeat.Version != 1 {
		t.Fatalf("heat changed despite rollback: %#v", storedHeat)
	}
	for _, code := range []string{"SR-07A", "SR-07B"} {
		var sample model.ChemicalSample
		if err := db.Where("code = ?", code).First(&sample).Error; err != nil {
			t.Fatalf("reload sample: %v", err)
		}
		if sample.Status != "verified" {
			t.Fatalf("sample locked despite rollback: %#v", sample)
		}
	}
	page, err := decisionRepository.List(ctx, dto.PageQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("decision survived audit failure: %d rows", page.Total)
	}
}

func db_modelHeat(t *testing.T, db *gorm.DB, code string) (model.Heat, error) {
	t.Helper()
	var heat model.Heat
	err := db.Where("code = ?", code).First(&heat).Error
	return heat, err
}

func TestReleaseRejectsReasonlessScrap(t *testing.T) {
	db, reviews, _, _, _ := newReleaseStack(t)
	ctx := context.Background()
	heat := workflowHeat("H-REL-08")
	if err := db.Create(&heat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	if _, err := reviews.Adjudicate(ctx, dto.ReleaseAdjudicationRequest{
		HeatCode: heat.Code, Decision: "scrap", Reason: " ", Evidence: "EVID-NO-REASON",
	}, "reviewer", "req-no-reason"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected reason validation, got %v", err)
	}
}
