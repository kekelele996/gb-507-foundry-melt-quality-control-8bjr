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

type FurnaceService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.Furnace], error)
	Get(context.Context, uint) (model.Furnace, error)
	Create(context.Context, dto.CreateFurnace, string, string) (model.Furnace, error)
	Update(context.Context, uint, dto.UpdateFurnace, string, string) (model.Furnace, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.Furnace, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type furnaceService struct {
	repository repository.FurnaceRepository
	security   SecurityService
}

func NewFurnaceService(repo repository.FurnaceRepository, security SecurityService) FurnaceService {
	return &furnaceService{repository: repo, security: security}
}

func (s *furnaceService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.Furnace], error) {
	return s.repository.List(ctx, query)
}

func (s *furnaceService) Get(ctx context.Context, id uint) (model.Furnace, error) {
	return s.repository.Get(ctx, id)
}

func (s *furnaceService) Create(ctx context.Context, input dto.CreateFurnace, actor, requestID string) (model.Furnace, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.Furnace, error) {
		if err := validateFurnace(input.Code, input.Name, input.PlantArea, input.FurnaceType, input.SupportedAlloys,
			input.Operator, input.CapacityTonnes, input.MaxTemperatureC, input.LastInspectionAt); err != nil {
			return model.Furnace{}, err
		}
		item := model.Furnace{
			BaseModel: model.BaseModel{
				Code: normalizeCode(input.Code), Name: strings.TrimSpace(input.Name), Status: model.FurnaceInitialStatus,
				Version: 1, Description: strings.TrimSpace(input.Description),
			},
			PlantArea: strings.TrimSpace(input.PlantArea), FurnaceType: strings.TrimSpace(input.FurnaceType),
			CapacityTonnes: input.CapacityTonnes, SupportedAlloys: strings.TrimSpace(input.SupportedAlloys),
			MaxTemperatureC: input.MaxTemperatureC, Operator: strings.TrimSpace(input.Operator),
			LastInspectionAt: input.LastInspectionAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		}
		if err := s.repository.Create(txCtx, &item); err != nil {
			return model.Furnace{}, fmt.Errorf("create furnace: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "create", "Furnace", item.ID, "", item.Status,
			furnaceAuditDetail(item)); err != nil {
			return model.Furnace{}, fmt.Errorf("persist furnace audit: %w", err)
		}
		return item, nil
	})
}

func (s *furnaceService) Update(ctx context.Context, id uint, input dto.UpdateFurnace, actor, requestID string) (model.Furnace, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.Furnace, error) {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return model.Furnace{}, err
		}
		if current.Status == "locked" {
			return model.Furnace{}, fmt.Errorf("%w: locked furnace is immutable until maintenance reopens it", ErrInvalidInput)
		}
		if err := validateFurnace(current.Code, input.Name, input.PlantArea, input.FurnaceType, input.SupportedAlloys,
			input.Operator, input.CapacityTonnes, input.MaxTemperatureC, input.LastInspectionAt); err != nil {
			return model.Furnace{}, err
		}
		current.Name = strings.TrimSpace(input.Name)
		current.Description = strings.TrimSpace(input.Description)
		current.PlantArea = strings.TrimSpace(input.PlantArea)
		current.FurnaceType = strings.TrimSpace(input.FurnaceType)
		current.CapacityTonnes = input.CapacityTonnes
		current.SupportedAlloys = strings.TrimSpace(input.SupportedAlloys)
		current.MaxTemperatureC = input.MaxTemperatureC
		current.Operator = strings.TrimSpace(input.Operator)
		current.LastInspectionAt = input.LastInspectionAt.UTC()
		current.Evidence = strings.TrimSpace(input.Evidence)
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = time.Now().UTC()
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return model.Furnace{}, fmt.Errorf("update furnace: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "update", "Furnace", id, current.Status, current.Status,
			furnaceAuditDetail(current)); err != nil {
			return model.Furnace{}, fmt.Errorf("persist furnace audit: %w", err)
		}
		return s.repository.Get(txCtx, id)
	})
}

func (s *furnaceService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.Furnace, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.Furnace, error) {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return model.Furnace{}, err
		}
		target := strings.TrimSpace(input.Status)
		if !constants.CanTransition(constants.FurnaceTransitions, current.Status, target) {
			return model.Furnace{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
		}
		if (target == "available" || target == "charging") && strings.TrimSpace(current.Evidence) == "" {
			return model.Furnace{}, fmt.Errorf("%w: reopening a furnace requires inspection evidence", ErrInvalidInput)
		}
		before := current.Status
		current.Status = target
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = time.Now().UTC()
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return model.Furnace{}, fmt.Errorf("transition furnace: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "transition", "Furnace", id, before, target, input.Reason); err != nil {
			return model.Furnace{}, fmt.Errorf("persist furnace transition audit: %w", err)
		}
		return s.repository.Get(txCtx, id)
	})
}

func (s *furnaceService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	return atomicError(ctx, s.security, func(txCtx context.Context) error {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return err
		}
		if current.Status != "maintenance" && current.Status != "locked" {
			return fmt.Errorf("%w: only an isolated furnace can be deleted", ErrInvalidInput)
		}
		if err := s.repository.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "delete", "Furnace", id, current.Status, "deleted", "furnace soft deleted")
	})
}

func (s *furnaceService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateFurnace(code, name, area, furnaceType, alloys, operator string, capacity, maxTemperature float64, inspectedAt time.Time) error {
	for _, value := range []string{code, name, area, furnaceType, alloys, operator} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: furnace identity, area, type, alloys and operator are required", ErrInvalidInput)
		}
	}
	if capacity <= 0 || capacity > 500 || maxTemperature < 500 || maxTemperature > 2200 || inspectedAt.IsZero() {
		return fmt.Errorf("%w: furnace capacity, temperature or inspection time is invalid", ErrInvalidInput)
	}
	if inspectedAt.After(time.Now().UTC().Add(5 * time.Minute)) {
		return fmt.Errorf("%w: furnace inspection time cannot be in the future", ErrInvalidInput)
	}
	return nil
}

func furnaceAuditDetail(item model.Furnace) string {
	return fmt.Sprintf("area=%s type=%s capacity=%.1ft max=%.0fC operator=%s",
		item.PlantArea, item.FurnaceType, item.CapacityTonnes, item.MaxTemperatureC, item.Operator)
}
