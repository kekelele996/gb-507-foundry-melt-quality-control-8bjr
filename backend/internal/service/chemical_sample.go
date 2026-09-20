package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/constants"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/repository"
)

type ChemicalSampleService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.ChemicalSample], error)
	Get(context.Context, uint) (model.ChemicalSample, error)
	Create(context.Context, dto.CreateChemicalSample, string, string) (model.ChemicalSample, error)
	Update(context.Context, uint, dto.UpdateChemicalSample, string, string) (model.ChemicalSample, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.ChemicalSample, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type chemicalSampleService struct {
	repository repository.ChemicalSampleRepository
	heats      repository.HeatRepository
	security   SecurityService
}

func NewChemicalSampleService(repo repository.ChemicalSampleRepository, heats repository.HeatRepository,
	security SecurityService) ChemicalSampleService {
	return &chemicalSampleService{repository: repo, heats: heats, security: security}
}

func (s *chemicalSampleService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.ChemicalSample], error) {
	return s.repository.List(ctx, query)
}

func (s *chemicalSampleService) Get(ctx context.Context, id uint) (model.ChemicalSample, error) {
	return s.repository.Get(ctx, id)
}

func (s *chemicalSampleService) Create(ctx context.Context, input dto.CreateChemicalSample, actor, requestID string) (model.ChemicalSample, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.ChemicalSample, error) {
		if err := validateChemicalSample(input.Code, input.Name, input.HeatCode, input.SamplePoint, input.MethodVersion,
			actor, input.CarbonPct, input.SiliconPct, input.ManganesePct, input.SulfurPct, input.PhosphorusPct,
			input.SampledAt, input.Evidence); err != nil {
			return model.ChemicalSample{}, err
		}
		heat, err := s.resolveOpenHeat(txCtx, input.HeatCode)
		if err != nil {
			return model.ChemicalSample{}, err
		}
		item := model.ChemicalSample{
			BaseModel: model.BaseModel{
				Code: normalizeCode(input.Code), Name: strings.TrimSpace(input.Name), Status: model.ChemicalSampleInitialStatus,
				Version: 1, Description: strings.TrimSpace(input.Description),
			},
			HeatCode: heat.Code, SamplePoint: strings.TrimSpace(input.SamplePoint), MethodVersion: strings.TrimSpace(input.MethodVersion),
			Analyst: strings.TrimSpace(actor), CarbonPct: input.CarbonPct, SiliconPct: input.SiliconPct,
			ManganesePct: input.ManganesePct, SulfurPct: input.SulfurPct, PhosphorusPct: input.PhosphorusPct,
			SampledAt: input.SampledAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		}
		if err := s.repository.Create(txCtx, &item); err != nil {
			return model.ChemicalSample{}, fmt.Errorf("create chemical sample: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "create", "ChemicalSample", item.ID, "", item.Status,
			sampleAuditDetail(item)); err != nil {
			return model.ChemicalSample{}, fmt.Errorf("persist sample audit: %w", err)
		}
		return item, nil
	})
}

func (s *chemicalSampleService) Update(ctx context.Context, id uint, input dto.UpdateChemicalSample, actor, requestID string) (model.ChemicalSample, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.ChemicalSample, error) {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return model.ChemicalSample{}, err
		}
		if current.Status == "verified" || current.Status == "locked" || current.Status == "rejected" {
			return model.ChemicalSample{}, fmt.Errorf("%w: terminal laboratory evidence is immutable", ErrInvalidInput)
		}
		if normalizeCode(input.HeatCode) != current.HeatCode {
			return model.ChemicalSample{}, fmt.Errorf("%w: a sample cannot be reassigned to another heat", ErrInvalidInput)
		}
		if err := validateChemicalSample(current.Code, input.Name, input.HeatCode, input.SamplePoint, input.MethodVersion,
			actor, input.CarbonPct, input.SiliconPct, input.ManganesePct, input.SulfurPct, input.PhosphorusPct,
			input.SampledAt, input.Evidence); err != nil {
			return model.ChemicalSample{}, err
		}
		if _, err := s.resolveOpenHeat(txCtx, current.HeatCode); err != nil {
			return model.ChemicalSample{}, err
		}
		current.Name = strings.TrimSpace(input.Name)
		current.Description = strings.TrimSpace(input.Description)
		current.SamplePoint = strings.TrimSpace(input.SamplePoint)
		current.MethodVersion = strings.TrimSpace(input.MethodVersion)
		current.Analyst = strings.TrimSpace(actor)
		current.CarbonPct = input.CarbonPct
		current.SiliconPct = input.SiliconPct
		current.ManganesePct = input.ManganesePct
		current.SulfurPct = input.SulfurPct
		current.PhosphorusPct = input.PhosphorusPct
		current.SampledAt = input.SampledAt.UTC()
		current.Evidence = strings.TrimSpace(input.Evidence)
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = time.Now().UTC()
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return model.ChemicalSample{}, fmt.Errorf("update chemical sample: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "update", "ChemicalSample", id, current.Status, current.Status,
			sampleAuditDetail(current)); err != nil {
			return model.ChemicalSample{}, fmt.Errorf("persist sample audit: %w", err)
		}
		return s.repository.Get(txCtx, id)
	})
}

func (s *chemicalSampleService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.ChemicalSample, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.ChemicalSample, error) {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return model.ChemicalSample{}, err
		}
		target := strings.TrimSpace(input.Status)
		if !constants.CanTransition(constants.ChemicalSampleTransitions, current.Status, target) {
			return model.ChemicalSample{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
		}
		if _, err := s.resolveOpenHeat(txCtx, current.HeatCode); err != nil {
			return model.ChemicalSample{}, err
		}
		if target == "verified" && (!current.IsPlausible() || strings.TrimSpace(current.Evidence) == "") {
			return model.ChemicalSample{}, fmt.Errorf("%w: verification requires plausible measurements and traceable evidence", ErrInvalidInput)
		}
		before := current.Status
		current.Status = target
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = time.Now().UTC()
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return model.ChemicalSample{}, fmt.Errorf("transition chemical sample: %w", err)
		}
		detail := input.Reason + "; " + sampleAuditDetail(current)
		if err := s.security.Audit(txCtx, actor, requestID, "transition", "ChemicalSample", id, before, target, detail); err != nil {
			return model.ChemicalSample{}, fmt.Errorf("persist sample transition audit: %w", err)
		}
		return s.repository.Get(txCtx, id)
	})
}

func (s *chemicalSampleService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	return atomicError(ctx, s.security, func(txCtx context.Context) error {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return err
		}
		if current.Status != "collected" {
			return fmt.Errorf("%w: only an untested sample can be deleted", ErrInvalidInput)
		}
		if err := s.repository.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "delete", "ChemicalSample", id, current.Status, "deleted", "untested sample soft deleted")
	})
}

func (s *chemicalSampleService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func (s *chemicalSampleService) resolveOpenHeat(ctx context.Context, heatCode string) (model.Heat, error) {
	heat, err := s.heats.GetByCode(ctx, normalizeCode(heatCode))
	if err != nil {
		return model.Heat{}, fmt.Errorf("resolve sample heat: %w", err)
	}
	if heat.Status != "sampling" && heat.Status != "hold" {
		return model.Heat{}, fmt.Errorf("%w: samples are only accepted while a heat is sampling or on quality hold", ErrInvalidInput)
	}
	return heat, nil
}

func validateChemicalSample(code, name, heatCode, samplePoint, methodVersion, analyst string,
	carbon, silicon, manganese, sulfur, phosphorus float64, sampledAt time.Time, evidence string) error {
	for _, value := range []string{code, name, heatCode, samplePoint, methodVersion, analyst, evidence} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: sample identity, heat, point, method, analyst and evidence are required", ErrInvalidInput)
		}
	}
	probe := model.ChemicalSample{CarbonPct: carbon, SiliconPct: silicon, ManganesePct: manganese, SulfurPct: sulfur, PhosphorusPct: phosphorus}
	if !probe.IsPlausible() || sampledAt.IsZero() {
		return fmt.Errorf("%w: sample time or chemistry values are outside laboratory limits", ErrInvalidInput)
	}
	if sampledAt.After(time.Now().UTC().Add(5 * time.Minute)) {
		return fmt.Errorf("%w: sample time cannot be in the future", ErrInvalidInput)
	}
	return nil
}

func sampleAuditDetail(item model.ChemicalSample) string {
	return fmt.Sprintf("heat=%s method=%s analyst=%s C=%.3f Si=%.3f Mn=%.3f S=%.3f P=%.3f",
		item.HeatCode, item.MethodVersion, item.Analyst, item.CarbonPct, item.SiliconPct,
		item.ManganesePct, item.SulfurPct, item.PhosphorusPct)
}
