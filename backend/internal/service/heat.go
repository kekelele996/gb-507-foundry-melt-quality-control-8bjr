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

type HeatService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.Heat], error)
	Get(context.Context, uint) (model.Heat, error)
	Create(context.Context, dto.CreateHeat, string, string) (model.Heat, error)
	Update(context.Context, uint, dto.UpdateHeat, string, string) (model.Heat, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.Heat, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type heatService struct {
	repository repository.HeatRepository
	furnaces   repository.FurnaceRepository
	samples    repository.ChemicalSampleRepository
	security   SecurityService
}

func NewHeatService(repo repository.HeatRepository, furnaces repository.FurnaceRepository,
	samples repository.ChemicalSampleRepository, security SecurityService) HeatService {
	return &heatService{repository: repo, furnaces: furnaces, samples: samples, security: security}
}

func (s *heatService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.Heat], error) {
	return s.repository.List(ctx, query)
}

func (s *heatService) Get(ctx context.Context, id uint) (model.Heat, error) {
	return s.repository.Get(ctx, id)
}

func (s *heatService) Create(ctx context.Context, input dto.CreateHeat, actor, requestID string) (model.Heat, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.Heat, error) {
		if err := validateHeat(input.Code, input.Name, input.FurnaceCode, input.AlloyGrade, input.Owner,
			input.ChargeWeightKg, input.TargetTemperatureC, input.CarbonMinPct, input.CarbonMaxPct,
			input.SiliconMinPct, input.SiliconMaxPct, input.SulfurMaxPct, input.PhosphorusMaxPct, input.StartedAt); err != nil {
			return model.Heat{}, err
		}
		furnace, err := s.resolveFurnace(txCtx, input.FurnaceCode, input.AlloyGrade, input.ChargeWeightKg, input.TargetTemperatureC)
		if err != nil {
			return model.Heat{}, err
		}
		item := model.Heat{
			BaseModel: model.BaseModel{
				Code: normalizeCode(input.Code), Name: strings.TrimSpace(input.Name), Status: model.HeatInitialStatus,
				Version: 1, Description: strings.TrimSpace(input.Description),
			},
			FurnaceCode: furnace.Code, AlloyGrade: strings.ToUpper(strings.TrimSpace(input.AlloyGrade)),
			Owner: strings.TrimSpace(input.Owner), ChargeWeightKg: input.ChargeWeightKg,
			TargetTemperatureC: input.TargetTemperatureC, CarbonMinPct: input.CarbonMinPct,
			CarbonMaxPct: input.CarbonMaxPct, SiliconMinPct: input.SiliconMinPct, SiliconMaxPct: input.SiliconMaxPct,
			SulfurMaxPct: input.SulfurMaxPct, PhosphorusMaxPct: input.PhosphorusMaxPct,
			StartedAt: input.StartedAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		}
		if err := s.repository.Create(txCtx, &item); err != nil {
			return model.Heat{}, fmt.Errorf("create heat: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "create", "Heat", item.ID, "", item.Status,
			heatAuditDetail(item)); err != nil {
			return model.Heat{}, fmt.Errorf("persist heat audit: %w", err)
		}
		return item, nil
	})
}

func (s *heatService) Update(ctx context.Context, id uint, input dto.UpdateHeat, actor, requestID string) (model.Heat, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.Heat, error) {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return model.Heat{}, err
		}
		if current.Status != "charged" {
			return model.Heat{}, fmt.Errorf("%w: chemistry specification is immutable after melting starts", ErrInvalidInput)
		}
		if err := validateHeat(current.Code, input.Name, input.FurnaceCode, input.AlloyGrade, input.Owner,
			input.ChargeWeightKg, input.TargetTemperatureC, input.CarbonMinPct, input.CarbonMaxPct,
			input.SiliconMinPct, input.SiliconMaxPct, input.SulfurMaxPct, input.PhosphorusMaxPct, input.StartedAt); err != nil {
			return model.Heat{}, err
		}
		furnace, err := s.resolveFurnace(txCtx, input.FurnaceCode, input.AlloyGrade, input.ChargeWeightKg, input.TargetTemperatureC)
		if err != nil {
			return model.Heat{}, err
		}
		current.Name = strings.TrimSpace(input.Name)
		current.Description = strings.TrimSpace(input.Description)
		current.FurnaceCode = furnace.Code
		current.AlloyGrade = strings.ToUpper(strings.TrimSpace(input.AlloyGrade))
		current.Owner = strings.TrimSpace(input.Owner)
		current.ChargeWeightKg = input.ChargeWeightKg
		current.TargetTemperatureC = input.TargetTemperatureC
		current.CarbonMinPct = input.CarbonMinPct
		current.CarbonMaxPct = input.CarbonMaxPct
		current.SiliconMinPct = input.SiliconMinPct
		current.SiliconMaxPct = input.SiliconMaxPct
		current.SulfurMaxPct = input.SulfurMaxPct
		current.PhosphorusMaxPct = input.PhosphorusMaxPct
		current.StartedAt = input.StartedAt.UTC()
		current.Evidence = strings.TrimSpace(input.Evidence)
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = time.Now().UTC()
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return model.Heat{}, fmt.Errorf("update heat: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "update", "Heat", id, current.Status, current.Status,
			heatAuditDetail(current)); err != nil {
			return model.Heat{}, fmt.Errorf("persist heat audit: %w", err)
		}
		return s.repository.Get(txCtx, id)
	})
}

func (s *heatService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.Heat, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.Heat, error) {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return model.Heat{}, err
		}
		target := strings.TrimSpace(input.Status)
		if !constants.CanTransition(constants.HeatTransitions, current.Status, target) {
			return model.Heat{}, fmt.Errorf("%w: %s -> %s; final heat states are derived from a quality decision", ErrInvalidTransition, current.Status, target)
		}
		if target == "hold" {
			hasTerminal, sampleErr := s.samples.HasTerminalForHeat(txCtx, current.Code)
			if sampleErr != nil {
				return model.Heat{}, fmt.Errorf("check heat samples: %w", sampleErr)
			}
			if !hasTerminal {
				return model.Heat{}, fmt.Errorf("%w: a terminal chemical sample is required before quality hold", ErrInvalidInput)
			}
		}
		before := current.Status
		current.Status = target
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = time.Now().UTC()
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return model.Heat{}, fmt.Errorf("transition heat: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "transition", "Heat", id, before, target, input.Reason); err != nil {
			return model.Heat{}, fmt.Errorf("persist heat transition audit: %w", err)
		}
		return s.repository.Get(txCtx, id)
	})
}

func (s *heatService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	return atomicError(ctx, s.security, func(txCtx context.Context) error {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return err
		}
		if current.Status != "charged" {
			return fmt.Errorf("%w: only a charged heat can be deleted", ErrInvalidInput)
		}
		if err := s.repository.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "delete", "Heat", id, current.Status, "deleted", "charged heat soft deleted")
	})
}

func (s *heatService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func (s *heatService) resolveFurnace(ctx context.Context, furnaceCode, alloy string, weightKg, temperature float64) (model.Furnace, error) {
	furnace, err := s.furnaces.GetByCode(ctx, normalizeCode(furnaceCode))
	if err != nil {
		return model.Furnace{}, fmt.Errorf("resolve heat furnace: %w", err)
	}
	if furnace.Status == "maintenance" || furnace.Status == "locked" {
		return model.Furnace{}, fmt.Errorf("%w: furnace is unavailable for charging", ErrInvalidInput)
	}
	if weightKg > furnace.CapacityTonnes*1000 {
		return model.Furnace{}, fmt.Errorf("%w: charge weight exceeds furnace capacity", ErrInvalidInput)
	}
	if temperature > furnace.MaxTemperatureC {
		return model.Furnace{}, fmt.Errorf("%w: target temperature exceeds furnace capability", ErrInvalidInput)
	}
	if !alloySupported(furnace.SupportedAlloys, alloy) {
		return model.Furnace{}, fmt.Errorf("%w: alloy grade is not supported by the furnace", ErrInvalidInput)
	}
	return furnace, nil
}

func validateHeat(code, name, furnaceCode, alloy, owner string, weight, temperature, carbonMin, carbonMax,
	siliconMin, siliconMax, sulfurMax, phosphorusMax float64, startedAt time.Time) error {
	for _, value := range []string{code, name, furnaceCode, alloy, owner} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: heat identity, furnace, alloy and owner are required", ErrInvalidInput)
		}
	}
	if weight <= 0 || temperature < 500 || temperature > 2200 || startedAt.IsZero() ||
		carbonMin < 0 || carbonMax <= carbonMin || carbonMax > 6 || siliconMin < 0 ||
		siliconMax <= siliconMin || siliconMax > 6 || sulfurMax <= 0 || sulfurMax > 1 ||
		phosphorusMax <= 0 || phosphorusMax > 1 {
		return fmt.Errorf("%w: heat capacity, temperature or chemistry limits are invalid", ErrInvalidInput)
	}
	if startedAt.After(time.Now().UTC().Add(5 * time.Minute)) {
		return fmt.Errorf("%w: heat start time cannot be in the future", ErrInvalidInput)
	}
	return nil
}

func alloySupported(supported, requested string) bool {
	target := strings.ToUpper(strings.TrimSpace(requested))
	for _, alloy := range strings.FieldsFunc(supported, func(r rune) bool { return r == ',' || r == ';' || r == '/' }) {
		if strings.ToUpper(strings.TrimSpace(alloy)) == target {
			return true
		}
	}
	return false
}

func heatAuditDetail(item model.Heat) string {
	return fmt.Sprintf("furnace=%s alloy=%s charge=%.0fkg target=%.0fC C=%.3f-%.3f Si=%.3f-%.3f S<=%.3f P<=%.3f",
		item.FurnaceCode, item.AlloyGrade, item.ChargeWeightKg, item.TargetTemperatureC,
		item.CarbonMinPct, item.CarbonMaxPct, item.SiliconMinPct, item.SiliconMaxPct,
		item.SulfurMaxPct, item.PhosphorusMaxPct)
}
