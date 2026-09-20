package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/config"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDecisionValidatesOwnershipAndAtomicallyFinalizesHeat(t *testing.T) {
	db := workflowTestDB(t)
	ctx := context.Background()
	heatRepository := repository.NewHeatRepository(db)
	sampleRepository := repository.NewChemicalSampleRepository(db)
	decisionRepository := repository.NewQualityDecisionRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{AppName: "test", JWTSecret: strings.Repeat("a", 32)})
	decisions := NewQualityDecisionService(decisionRepository, heatRepository, sampleRepository, security)

	heat := workflowHeat("H-TEST-01")
	otherHeat := workflowHeat("H-TEST-02")
	if err := db.Create(&[]model.Heat{heat, otherHeat}).Error; err != nil {
		t.Fatalf("seed heats: %v", err)
	}
	var storedHeats []model.Heat
	if err := db.Order("id").Find(&storedHeats).Error; err != nil {
		t.Fatalf("read heats: %v", err)
	}
	sample := workflowSample("S-TEST-01", heat.Code)
	secondSample := workflowSample("S-TEST-01B", heat.Code)
	secondSample.CarbonPct = 3.32
	secondSample.SiliconPct = 2.03
	otherSample := workflowSample("S-TEST-02", otherHeat.Code)
	if err := db.Create(&[]model.ChemicalSample{sample, secondSample, otherSample}).Error; err != nil {
		t.Fatalf("seed samples: %v", err)
	}

	input := dto.CreateQualityDecision{
		Code: "QD-TEST-01", Name: "Acceptance decision", HeatCode: heat.Code, SampleCode: otherSample.Code,
		Reviewer: "spoofed-user", Reason: "chemistry reviewed", DecidedAt: time.Now().UTC(), Evidence: "LIMS-signed-result",
	}
	if _, err := decisions.Create(ctx, input, "reviewer", "req-mismatch"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected cross-heat sample rejection, got %v", err)
	}

	input.SampleCode = sample.Code
	created, err := decisions.Create(ctx, input, "reviewer", "req-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	if created.Reviewer != "reviewer" {
		t.Fatalf("reviewer was not derived from authenticated actor: %q", created.Reviewer)
	}
	finalized, err := decisions.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "accept", ExpectedVersion: created.Version, Reason: "all chemistry limits satisfied",
	}, "reviewer", "req-final")
	if err != nil {
		t.Fatalf("finalize decision: %v", err)
	}
	if finalized.Status != "accept" || finalized.Version != 2 {
		t.Fatalf("unexpected decision result: %#v", finalized)
	}
	storedHeat, err := heatRepository.GetByCode(ctx, heat.Code)
	if err != nil {
		t.Fatalf("load derived heat: %v", err)
	}
	if storedHeat.Status != "accepted" || storedHeat.Version != 2 {
		t.Fatalf("heat final state was not derived atomically: %#v", storedHeat)
	}
	var auditCount int64
	if err := db.Model(&model.AuditLog{}).Where("request_id = ?", "req-final").Count(&auditCount).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("expected decision and heat audits, got %d", auditCount)
	}
	if _, err := decisions.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "remelt", ExpectedVersion: 2, Reason: "attempt reversal",
	}, "reviewer", "req-reverse"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected terminal decision to be irreversible, got %v", err)
	}
}

func TestHeatCannotBypassQualityDecision(t *testing.T) {
	db := workflowTestDB(t)
	ctx := context.Background()
	furnaceRepository := repository.NewFurnaceRepository(db)
	heatRepository := repository.NewHeatRepository(db)
	sampleRepository := repository.NewChemicalSampleRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{AppName: "test", JWTSecret: strings.Repeat("a", 32)})
	heats := NewHeatService(heatRepository, furnaceRepository, sampleRepository, security)
	heat := workflowHeat("H-BYPASS")
	if err := db.Create(&heat).Error; err != nil {
		t.Fatalf("seed heat: %v", err)
	}
	if _, err := heats.Transition(ctx, heat.ID, dto.TransitionRequest{
		Status: "accepted", ExpectedVersion: 1, Reason: "manual bypass",
	}, "operator", "req-bypass"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected direct acceptance to fail, got %v", err)
	}
}

func TestAuditFailureRollsBackBusinessWrite(t *testing.T) {
	db := workflowTestDB(t)
	if err := db.Migrator().DropTable(&model.AuditLog{}); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{AppName: "test", JWTSecret: strings.Repeat("a", 32)})
	furnaces := NewFurnaceService(repository.NewFurnaceRepository(db), security)
	_, err := furnaces.Create(context.Background(), dto.CreateFurnace{
		Code: "F-ROLLBACK", Name: "Rollback furnace", PlantArea: "A", FurnaceType: "induction",
		CapacityTonnes: 10, SupportedAlloys: "HT250", MaxTemperatureC: 1600, Operator: "operator",
		LastInspectionAt: time.Now().UTC(), Evidence: "inspection-report",
	}, "admin", "req-rollback")
	if err == nil {
		t.Fatal("expected missing audit table to fail the transaction")
	}
	var count int64
	if err := db.Model(&model.Furnace{}).Where("code = ?", "F-ROLLBACK").Count(&count).Error; err != nil {
		t.Fatalf("count furnaces: %v", err)
	}
	if count != 0 {
		t.Fatalf("business row survived audit failure: %d", count)
	}
}

func workflowTestDB(t *testing.T) *gorm.DB {
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
	return db
}

func workflowHeat(code string) model.Heat {
	return model.Heat{
		BaseModel:   model.BaseModel{Code: code, Name: code, Status: "hold", Version: 1},
		FurnaceCode: "F-TEST", AlloyGrade: "HT250", Owner: "operator", ChargeWeightKg: 1000,
		TargetTemperatureC: 1500, CarbonMinPct: 3.1, CarbonMaxPct: 3.5, SiliconMinPct: 1.8,
		SiliconMaxPct: 2.3, SulfurMaxPct: 0.08, PhosphorusMaxPct: 0.12,
		StartedAt: time.Now().UTC().Add(-time.Hour), Evidence: "MES-record",
	}
}

func workflowSample(code, heatCode string) model.ChemicalSample {
	return model.ChemicalSample{
		BaseModel: model.BaseModel{Code: code, Name: code, Status: "verified", Version: 1},
		HeatCode:  heatCode, SamplePoint: "ladle", MethodVersion: "OES-1", Analyst: "operator",
		CarbonPct: 3.3, SiliconPct: 2.0, ManganesePct: 0.7, SulfurPct: 0.04, PhosphorusPct: 0.07,
		SampledAt: time.Now().UTC().Add(-30 * time.Minute), Evidence: "LIMS-result",
	}
}
