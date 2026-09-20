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

func TestEvaluatePairingRules(t *testing.T) {
	heat := workflowHeat("H-RULES")

	// Fewer than two verified samples.
	if result := EvaluatePairing(heat, []model.ChemicalSample{workflowSample("A", heat.Code)}); result.Eligible() {
		t.Fatal("single sample must not be eligible")
	}

	// Two in-range, agreeing samples pass.
	good := EvaluatePairing(heat, []model.ChemicalSample{
		workflowSampleWith("A", heat.Code, 3.30, 2.00, 0.04, 0.07),
		workflowSampleWith("B", heat.Code, 3.34, 2.04, 0.03, 0.06),
	})
	if !good.Eligible() {
		t.Fatalf("agreeing in-range pair must pass, blockers=%v", good.Blockers)
	}
	if absTestFloat(good.CarbonDelta-0.04) > 1e-9 || !good.CarbonDeltaOK || !good.SiliconDeltaOK {
		t.Fatalf("unexpected deltas: %#v", good)
	}

	// Carbon delta above 0.05 blocks.
	carbonDrift := EvaluatePairing(heat, []model.ChemicalSample{
		workflowSampleWith("A", heat.Code, 3.15, 2.00, 0.04, 0.07),
		workflowSampleWith("B", heat.Code, 3.24, 2.02, 0.04, 0.07),
	})
	if carbonDrift.Eligible() || !strings.Contains(strings.Join(carbonDrift.Blockers, "；"), "碳读数差值") {
		t.Fatalf("carbon drift must block: %v", carbonDrift.Blockers)
	}

	// Silicon delta above 0.05 blocks.
	siliconDrift := EvaluatePairing(heat, []model.ChemicalSample{
		workflowSampleWith("A", heat.Code, 3.30, 1.90, 0.04, 0.07),
		workflowSampleWith("B", heat.Code, 3.31, 1.98, 0.04, 0.07),
	})
	if siliconDrift.Eligible() || !strings.Contains(strings.Join(siliconDrift.Blockers, "；"), "硅读数差值") {
		t.Fatalf("silicon drift must block: %v", siliconDrift.Blockers)
	}

	// Out-of-grade sulfur blocks.
	sulfurHigh := EvaluatePairing(heat, []model.ChemicalSample{
		workflowSampleWith("A", heat.Code, 3.30, 2.00, 0.04, 0.07),
		workflowSampleWith("B", heat.Code, 3.31, 2.01, 0.09, 0.07),
	})
	if sulfurHigh.Eligible() || !strings.Contains(strings.Join(sulfurHigh.Blockers, "；"), "硫含量") {
		t.Fatalf("sulfur over grade must block: %v", sulfurHigh.Blockers)
	}
}

func absTestFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func releaseReviewFixture(t *testing.T) (*gorm.DB, ReleaseReviewService, repository.HeatRepository, repository.ChemicalSampleRepository) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1) // serialize to expose compare-and-set races deterministically
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}, &model.Furnace{}, &model.Heat{},
		&model.ChemicalSample{}, &model.QualityDecision{}, &model.HeatReleaseReview{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{AppName: "test", JWTSecret: strings.Repeat("a", 32)})
	heatRepository := repository.NewHeatRepository(db)
	sampleRepository := repository.NewChemicalSampleRepository(db)
	decisionRepository := repository.NewQualityDecisionRepository(db)
	reviewRepository := repository.NewHeatReleaseReviewRepository(db)
	service := NewReleaseReviewService(reviewRepository, heatRepository, sampleRepository, decisionRepository, security)
	return db, service, heatRepository, sampleRepository
}

func seedReviewHeat(t *testing.T, db *gorm.DB, code string, samples ...model.ChemicalSample) model.Heat {
	t.Helper()
	heat := workflowHeat(code)
	if err := db.Create(&heat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	if len(samples) > 0 {
		if err := db.Create(&samples).Error; err != nil {
			t.Fatalf("seed samples: %v", err)
		}
	}
	return heat
}

func TestReleaseReviewAcceptLocksSamplesAndFinalizesAtomically(t *testing.T) {
	db, reviews, heatRepository, sampleRepository := releaseReviewFixture(t)
	ctx := context.Background()
	heat := seedReviewHeat(t, db, "H-OK",
		workflowSampleWith("S-OK-1", "H-OK", 3.30, 2.00, 0.04, 0.07),
		workflowSampleWith("S-OK-2", "H-OK", 3.34, 2.04, 0.03, 0.06),
	)

	open, err := reviews.Open(ctx, dto.CreateReleaseReview{Name: "Joint review", HeatCode: heat.Code}, "reviewer", "req-open")
	if err != nil {
		t.Fatalf("open review: %v", err)
	}
	if open.Status != "open" || open.FirstSampleCode != "S-OK-1" || open.SecondSampleCode != "S-OK-2" {
		t.Fatalf("unexpected opened review: %#v", open)
	}

	judged, err := reviews.Judge(ctx, open.ID, dto.JudgeReleaseReview{
		Target: "accepted", ExpectedVersion: open.Version,
	}, "reviewer", "req-judge")
	if err != nil {
		t.Fatalf("judge accept: %v", err)
	}
	if judged.Status != "accepted" || judged.Version != 2 {
		t.Fatalf("unexpected judged review: %#v", judged)
	}
	storedHeat, err := heatRepository.GetByCode(ctx, heat.Code)
	if err != nil || storedHeat.Status != "accepted" {
		t.Fatalf("heat not accepted: heat=%#v err=%v", storedHeat, err)
	}
	for _, code := range []string{"S-OK-1", "S-OK-2"} {
		sample, err := sampleRepository.GetByCode(ctx, code)
		if err != nil || sample.Status != "locked" {
			t.Fatalf("sample %s not locked: %#v err=%v", code, sample, err)
		}
	}
	var decision model.QualityDecision
	if err := db.Where("heat_code = ?", heat.Code).First(&decision).Error; err != nil || decision.Status != "accept" {
		t.Fatalf("decision not finalized: %#v err=%v", decision, err)
	}
	var audits int64
	if err := db.Model(&model.AuditLog{}).Where("request_id = ?", "req-judge").Count(&audits).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if audits != 5 { // review + 2 samples + decision + heat
		t.Fatalf("expected 5 verdict audits in one request, got %d", audits)
	}

	// Second judgment must fail.
	if _, err := reviews.Judge(ctx, open.ID, dto.JudgeReleaseReview{
		Target: "remelted", ExpectedVersion: 2, Reason: "duplicate verdict",
	}, "reviewer", "req-dup"); !errors.Is(err, ErrReleaseConflict) {
		t.Fatalf("duplicate verdict must conflict, got %v", err)
	}
}

func TestReleaseReviewBlockedPairCannotAcceptButCanScrap(t *testing.T) {
	db, reviews, heatRepository, _ := releaseReviewFixture(t)
	ctx := context.Background()
	heat := seedReviewHeat(t, db, "H-BLOCKED",
		workflowSampleWith("S-B1", "H-BLOCKED", 3.15, 2.00, 0.04, 0.07),
		workflowSampleWith("S-B2", "H-BLOCKED", 3.30, 2.02, 0.04, 0.07),
	)
	open, err := reviews.Open(ctx, dto.CreateReleaseReview{Name: "Blocked review", HeatCode: heat.Code}, "reviewer", "req-open")
	if err != nil {
		t.Fatalf("open blocked review: %v", err)
	}
	if _, err := reviews.Judge(ctx, open.ID, dto.JudgeReleaseReview{
		Target: "accepted", ExpectedVersion: 1,
	}, "reviewer", "req-accept"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("accept on blocked pair must fail business rule, got %v", err)
	}
	if _, err := reviews.Judge(ctx, open.ID, dto.JudgeReleaseReview{
		Target: "scrapped", ExpectedVersion: 1,
	}, "reviewer", "req-scrap-noreason"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("scrap without reason must fail, got %v", err)
	}
	scrapped, err := reviews.Judge(ctx, open.ID, dto.JudgeReleaseReview{
		Target: "scrapped", ExpectedVersion: 1, Reason: "碳差值超标，成分不可放行",
	}, "reviewer", "req-scrap")
	if err != nil {
		t.Fatalf("scrap with reason: %v", err)
	}
	if scrapped.Status != "scrapped" {
		t.Fatalf("expected scrapped, got %s", scrapped.Status)
	}
	storedHeat, err := heatRepository.GetByCode(ctx, heat.Code)
	if err != nil || storedHeat.Status != "rejected" {
		t.Fatalf("heat must be rejected after scrap: %#v", storedHeat)
	}
	var decision model.QualityDecision
	if err := db.Where("heat_code = ?", heat.Code).First(&decision).Error; err != nil || decision.Status != "scrap" {
		t.Fatalf("scrap decision missing: %#v err=%v", decision, err)
	}
	// Samples stay verified when not accepted.
	var lockedCount int64
	if err := db.Model(&model.ChemicalSample{}).Where("heat_code = ? AND status = ?", heat.Code, "locked").Count(&lockedCount).Error; err != nil {
		t.Fatalf("count locked: %v", err)
	}
	if lockedCount != 0 {
		t.Fatalf("scrap must not lock samples, locked=%d", lockedCount)
	}
}

func TestReleaseReviewSingleSampleOnlyAllowsRemeltOrScrap(t *testing.T) {
	db, reviews, heatRepository, _ := releaseReviewFixture(t)
	ctx := context.Background()
	heat := seedReviewHeat(t, db, "H-SINGLE", workflowSample("S-ONLY", "H-SINGLE"))
	open, err := reviews.Open(ctx, dto.CreateReleaseReview{Name: "Single sample review", HeatCode: heat.Code}, "reviewer", "open")
	if err != nil {
		t.Fatalf("open with one sample: %v", err)
	}
	if open.SecondSampleCode != "" || open.PairingBlockers == "" {
		t.Fatalf("review should snapshot the missing leg, got %#v", open)
	}
	if _, err := reviews.Judge(ctx, open.ID, dto.JudgeReleaseReview{
		Target: "accepted", ExpectedVersion: 1,
	}, "reviewer", "accept"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("accept with one sample must fail, got %v", err)
	}
	remelted, err := reviews.Judge(ctx, open.ID, dto.JudgeReleaseReview{
		Target: "remelted", ExpectedVersion: 1, Reason: "缺第二份复核样本，返炉重新取样",
	}, "reviewer", "remelt")
	if err != nil {
		t.Fatalf("remelt with reason: %v", err)
	}
	if remelted.Status != "remelted" {
		t.Fatalf("expected remelted, got %s", remelted.Status)
	}
	storedHeat, err := heatRepository.GetByCode(ctx, heat.Code)
	if err != nil || storedHeat.Status != "rejected" {
		t.Fatalf("heat must be rejected after remelt: %#v", storedHeat)
	}
}

func TestReleaseReviewRejectsNonHoldHeatAndDuplicateOpen(t *testing.T) {
	db, reviews, heatRepository, _ := releaseReviewFixture(t)
	ctx := context.Background()
	heat := seedReviewHeat(t, db, "H-DUP",
		workflowSample("S-D1", "H-DUP"), workflowSampleWith("S-D2", "H-DUP", 3.31, 2.01, 0.04, 0.07))
	open, err := reviews.Open(ctx, dto.CreateReleaseReview{Name: "First review", HeatCode: heat.Code}, "reviewer", "r1")
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	if _, err := reviews.Open(ctx, dto.CreateReleaseReview{Name: "Second review", HeatCode: heat.Code}, "reviewer", "r2"); !errors.Is(err, ErrReleaseConflict) {
		t.Fatalf("duplicate open must conflict, got %v", err)
	}
	stored, err := heatRepository.GetByCode(ctx, heat.Code)
	if err != nil {
		t.Fatalf("load heat: %v", err)
	}
	stored.Status = "accepted"
	stored.Version++
	if err := heatRepository.Update(ctx, stored.ID, stored.Version-1, &stored); err != nil {
		t.Fatalf("move heat: %v", err)
	}
	if _, err := reviews.Judge(ctx, open.ID, dto.JudgeReleaseReview{
		Target: "accepted", ExpectedVersion: 1,
	}, "reviewer", "r3"); !errors.Is(err, ErrReleaseConflict) {
		t.Fatalf("judgment against non-hold heat must conflict, got %v", err)
	}
}

func TestConcurrentJudgmentSucceedsOnce(t *testing.T) {
	db, reviews, heatRepository, _ := releaseReviewFixture(t)
	ctx := context.Background()
	heat := seedReviewHeat(t, db, "H-RACE",
		workflowSample("S-R1", "H-RACE"), workflowSampleWith("S-R2", "H-RACE", 3.33, 2.02, 0.04, 0.07))
	open, err := reviews.Open(ctx, dto.CreateReleaseReview{Name: "Race review", HeatCode: heat.Code}, "reviewer", "open")
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	results := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := reviews.Judge(context.Background(), open.ID, dto.JudgeReleaseReview{
				Target: "accepted", ExpectedVersion: open.Version,
			}, "reviewer", fmt.Sprintf("race-%d", i))
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrReleaseConflict):
			conflicts++
		default:
			t.Fatalf("unexpected race error: %v", err)
		}
	}
	if successes != 1 || conflicts != workers-1 {
		t.Fatalf("expected exactly one success, got successes=%d conflicts=%d", successes, conflicts)
	}
	storedHeat, err := heatRepository.GetByCode(ctx, heat.Code)
	if err != nil || storedHeat.Status != "accepted" || storedHeat.Version != 2 {
		t.Fatalf("heat not finalized exactly once: %#v err=%v", storedHeat, err)
	}
	var decisions int64
	if err := db.Model(&model.QualityDecision{}).Where("heat_code = ?", heat.Code).Count(&decisions).Error; err != nil {
		t.Fatalf("count decisions: %v", err)
	}
	if decisions != 1 {
		t.Fatalf("expected exactly one decision, got %d", decisions)
	}
	var locked int64
	if err := db.Model(&model.ChemicalSample{}).Where("heat_code = ? AND status = ?", heat.Code, "locked").Count(&locked).Error; err != nil {
		t.Fatalf("count locked: %v", err)
	}
	if locked != 2 {
		t.Fatalf("expected two locked samples, got %d", locked)
	}
}

func TestVerdictAuditFailureRollsBackEverything(t *testing.T) {
	db, reviews, _, _ := releaseReviewFixture(t)
	heat := seedReviewHeat(t, db, "H-ROLL",
		workflowSample("S-RL1", "H-ROLL"), workflowSampleWith("S-RL2", "H-ROLL", 3.32, 2.03, 0.04, 0.07))
	open, err := reviews.Open(context.Background(), dto.CreateReleaseReview{Name: "Rollback review", HeatCode: heat.Code}, "reviewer", "open")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrator().DropTable(&model.AuditLog{}); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	if _, err := reviews.Judge(context.Background(), open.ID, dto.JudgeReleaseReview{
		Target: "accepted", ExpectedVersion: open.Version,
	}, "reviewer", "judge"); err == nil {
		t.Fatal("expected verdict to fail when audit write is impossible")
	}
	var heatStatus string
	if err := db.Model(&model.Heat{}).Where("code = ?", heat.Code).Select("status").Scan(&heatStatus).Error; err != nil {
		t.Fatalf("read heat: %v", err)
	}
	if heatStatus != "hold" {
		t.Fatalf("heat escaped transaction rollback: %s", heatStatus)
	}
	var reviewStatus string
	if err := db.Model(&model.HeatReleaseReview{}).Where("id = ?", open.ID).Select("status").Scan(&reviewStatus).Error; err != nil {
		t.Fatalf("read review: %v", err)
	}
	if reviewStatus != "open" {
		t.Fatalf("review escaped transaction rollback: %s", reviewStatus)
	}
	var locked int64
	if err := db.Model(&model.ChemicalSample{}).Where("heat_code = ? AND status = ?", heat.Code, "locked").Count(&locked).Error; err != nil {
		t.Fatalf("count locked: %v", err)
	}
	if locked != 0 {
		t.Fatalf("samples escaped transaction rollback, locked=%d", locked)
	}
}

func TestPairingBoardReportsStateAndBlockers(t *testing.T) {
	db, reviews, _, _ := releaseReviewFixture(t)
	ready := workflowHeat("H-BOARD-OK")
	blocked := workflowHeat("H-BOARD-X")
	if err := db.Create(&[]model.Heat{ready, blocked}).Error; err != nil {
		t.Fatalf("seed heats: %v", err)
	}
	if err := db.Create(&[]model.ChemicalSample{
		workflowSample("S-OK-A", ready.Code),
		workflowSampleWith("S-OK-B", ready.Code, 3.33, 2.02, 0.04, 0.07),
		workflowSampleWith("S-X-A", blocked.Code, 3.12, 2.00, 0.04, 0.07),
		workflowSampleWith("S-X-B", blocked.Code, 3.30, 2.03, 0.04, 0.07),
	}).Error; err != nil {
		t.Fatalf("seed samples: %v", err)
	}
	if _, err := reviews.Open(context.Background(), dto.CreateReleaseReview{Name: "ready", HeatCode: ready.Code}, "reviewer", "o1"); err != nil {
		t.Fatalf("open ready: %v", err)
	}
	board, err := reviews.PairingBoard(context.Background())
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	byHeat := map[string]dto.ReleasePairingView{}
	for _, view := range board {
		byHeat[view.HeatCode] = view
	}
	okView := byHeat[ready.Code]
	if okView.PairingState != "ready" || !okView.CarbonDeltaOK || okView.ReviewStatus != "open" || len(okView.Blockers) != 0 {
		t.Fatalf("unexpected ready view: %#v", okView)
	}
	xView := byHeat[blocked.Code]
	if xView.PairingState != "blocked" || len(xView.Blockers) == 0 || xView.FirstSample == nil || xView.SecondSample == nil {
		t.Fatalf("unexpected blocked view: %#v", xView)
	}
}
