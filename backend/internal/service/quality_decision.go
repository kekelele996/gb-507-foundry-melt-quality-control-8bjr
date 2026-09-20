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

type QualityDecisionService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.QualityDecision], error)
	Get(context.Context, uint) (model.QualityDecision, error)
	Create(context.Context, dto.CreateQualityDecision, string, string) (model.QualityDecision, error)
	Update(context.Context, uint, dto.UpdateQualityDecision, string, string) (model.QualityDecision, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.QualityDecision, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type qualityDecisionService struct {
	repository repository.QualityDecisionRepository
	heats      repository.HeatRepository
	samples    repository.ChemicalSampleRepository
	security   SecurityService
}

func NewQualityDecisionService(repo repository.QualityDecisionRepository, heats repository.HeatRepository,
	samples repository.ChemicalSampleRepository, security SecurityService) QualityDecisionService {
	return &qualityDecisionService{repository: repo, heats: heats, samples: samples, security: security}
}

func (s *qualityDecisionService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.QualityDecision], error) {
	return s.repository.List(ctx, query)
}

func (s *qualityDecisionService) Get(ctx context.Context, id uint) (model.QualityDecision, error) {
	return s.repository.Get(ctx, id)
}

func (s *qualityDecisionService) Create(ctx context.Context, input dto.CreateQualityDecision, actor, requestID string) (model.QualityDecision, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.QualityDecision, error) {
		if err := validateQualityDecision(input.Code, input.Name, input.HeatCode, input.SampleCode, actor,
			input.Reason, input.DecidedAt, input.Evidence); err != nil {
			return model.QualityDecision{}, err
		}
		heat, sample, err := s.resolveContext(txCtx, input.HeatCode, input.SampleCode)
		if err != nil {
			return model.QualityDecision{}, err
		}
		if exists, duplicateErr := s.repository.HasForHeat(txCtx, heat.Code, 0); duplicateErr != nil {
			return model.QualityDecision{}, duplicateErr
		} else if exists {
			return model.QualityDecision{}, fmt.Errorf("%w: heat already has a final quality decision", ErrInvalidInput)
		}
		item := model.QualityDecision{
			BaseModel: model.BaseModel{
				Code: normalizeCode(input.Code), Name: strings.TrimSpace(input.Name), Status: model.QualityDecisionInitialStatus,
				Version: 1, Description: strings.TrimSpace(input.Description),
			},
			HeatCode: heat.Code, SampleCode: sample.Code, Reviewer: strings.TrimSpace(actor),
			Reason: strings.TrimSpace(input.Reason), Conditions: strings.TrimSpace(input.Conditions),
			DecidedAt: input.DecidedAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		}
		if err := s.repository.Create(txCtx, &item); err != nil {
			return model.QualityDecision{}, fmt.Errorf("create quality decision: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "create", "QualityDecision", item.ID, "", item.Status,
			decisionAuditDetail(item)); err != nil {
			return model.QualityDecision{}, fmt.Errorf("persist decision audit: %w", err)
		}
		return item, nil
	})
}

func (s *qualityDecisionService) Update(ctx context.Context, id uint, input dto.UpdateQualityDecision, actor, requestID string) (model.QualityDecision, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.QualityDecision, error) {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return model.QualityDecision{}, err
		}
		if current.Status != "draft" {
			return model.QualityDecision{}, fmt.Errorf("%w: signed quality decision is immutable", ErrInvalidInput)
		}
		if err := validateQualityDecision(current.Code, input.Name, input.HeatCode, input.SampleCode, actor,
			input.Reason, input.DecidedAt, input.Evidence); err != nil {
			return model.QualityDecision{}, err
		}
		heat, sample, err := s.resolveContext(txCtx, input.HeatCode, input.SampleCode)
		if err != nil {
			return model.QualityDecision{}, err
		}
		current.Name = strings.TrimSpace(input.Name)
		current.Description = strings.TrimSpace(input.Description)
		current.HeatCode = heat.Code
		current.SampleCode = sample.Code
		current.Reviewer = strings.TrimSpace(actor)
		current.Reason = strings.TrimSpace(input.Reason)
		current.Conditions = strings.TrimSpace(input.Conditions)
		current.DecidedAt = input.DecidedAt.UTC()
		current.Evidence = strings.TrimSpace(input.Evidence)
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = time.Now().UTC()
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return model.QualityDecision{}, fmt.Errorf("update quality decision: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "update", "QualityDecision", id, current.Status, current.Status,
			decisionAuditDetail(current)); err != nil {
			return model.QualityDecision{}, fmt.Errorf("persist decision audit: %w", err)
		}
		return s.repository.Get(txCtx, id)
	})
}

func (s *qualityDecisionService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.QualityDecision, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.QualityDecision, error) {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return model.QualityDecision{}, err
		}
		target := strings.TrimSpace(input.Status)
		if !constants.CanTransition(constants.QualityDecisionTransitions, current.Status, target) {
			return model.QualityDecision{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
		}
		heat, _, err := s.resolveContext(txCtx, current.HeatCode, current.SampleCode)
		if err != nil {
			return model.QualityDecision{}, err
		}
		if target == string(constants.DecisionTypeAccept) {
			// Acceptance always runs the heat-release panel gates, regardless
			// of whether the decision was created through the panel endpoint or
			// the legacy draft workflow: two independently verified samples,
			// all four graded elements in range, C/Si deltas within 0.05.
			verified, listErr := s.samples.ListVerifiedByHeat(txCtx, heat.Code)
			if listErr != nil {
				return model.QualityDecision{}, fmt.Errorf("list release samples: %w", listErr)
			}
			first, second, paired := releasePair(verified)
			if !paired {
				return model.QualityDecision{}, fmt.Errorf("%w: acceptance requires two verified samples; %s",
					ErrInvalidInput, pairShortageBlocker(len(verified)))
			}
			if blockers := pairBlockers(heat, first, second); len(blockers) > 0 {
				return model.QualityDecision{}, fmt.Errorf("%w: %s", ErrInvalidInput, strings.Join(blockers, "；"))
			}
			if current.SampleCode != first.Code && current.SampleCode != second.Code {
				return model.QualityDecision{}, fmt.Errorf("%w: decision reference sample is not part of the verified pair", ErrInvalidInput)
			}
			current.PairedSampleCode = second.Code
			current.Conditions = strings.TrimSpace(strings.TrimSpace(current.Conditions) + "; " +
				fmt.Sprintf("配对 %s/%s 双样本四元素在牌号范围内", first.Code, second.Code))
		}
		if exists, duplicateErr := s.repository.HasForHeat(txCtx, heat.Code, current.ID); duplicateErr != nil {
			return model.QualityDecision{}, duplicateErr
		} else if exists {
			return model.QualityDecision{}, fmt.Errorf("%w: heat already has a final quality decision", ErrInvalidInput)
		}
		before := current.Status
		current.Status = target
		current.Reviewer = strings.TrimSpace(actor)
		current.DecidedAt = time.Now().UTC()
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = current.DecidedAt
		if err := s.repository.Update(txCtx, id, input.ExpectedVersion, &current); err != nil {
			return model.QualityDecision{}, fmt.Errorf("transition quality decision: %w", err)
		}
		detail := input.Reason + "; " + decisionAuditDetail(current)
		if err := s.security.Audit(txCtx, actor, requestID, "transition", "QualityDecision", id, before, target, detail); err != nil {
			return model.QualityDecision{}, fmt.Errorf("persist decision transition audit: %w", err)
		}

		heatBefore := heat.Status
		if target == string(constants.DecisionTypeAccept) {
			heat.Status = string(constants.HeatStateAccepted)
		} else {
			heat.Status = string(constants.HeatStateRejected)
		}
		heat.Version++
		heat.UpdatedAt = time.Now().UTC()
		if err := s.heats.Update(txCtx, heat.ID, heat.Version-1, &heat); err != nil {
			return model.QualityDecision{}, fmt.Errorf("apply decision to heat: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "decision", "Heat", heat.ID, heatBefore, heat.Status,
			"heat state derived from quality decision "+current.Code); err != nil {
			return model.QualityDecision{}, fmt.Errorf("persist derived heat audit: %w", err)
		}
		return s.repository.Get(txCtx, id)
	})
}

func (s *qualityDecisionService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	return atomicError(ctx, s.security, func(txCtx context.Context) error {
		current, err := s.repository.Get(txCtx, id)
		if err != nil {
			return err
		}
		if current.Status != "draft" {
			return fmt.Errorf("%w: only a draft decision can be deleted", ErrInvalidInput)
		}
		if err := s.repository.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "delete", "QualityDecision", id, current.Status, "deleted", "draft decision soft deleted")
	})
}

func (s *qualityDecisionService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func (s *qualityDecisionService) resolveContext(ctx context.Context, heatCode, sampleCode string) (model.Heat, model.ChemicalSample, error) {
	heat, err := s.heats.GetByCode(ctx, normalizeCode(heatCode))
	if err != nil {
		return model.Heat{}, model.ChemicalSample{}, fmt.Errorf("resolve decision heat: %w", err)
	}
	if heat.Status != string(constants.HeatStateHold) {
		return model.Heat{}, model.ChemicalSample{}, fmt.Errorf("%w: heat must be on quality hold before a decision", ErrInvalidInput)
	}
	sample, err := s.samples.GetByCode(ctx, normalizeCode(sampleCode))
	if err != nil {
		return model.Heat{}, model.ChemicalSample{}, fmt.Errorf("resolve decision sample: %w", err)
	}
	if sample.HeatCode != heat.Code {
		return model.Heat{}, model.ChemicalSample{}, fmt.Errorf("%w: chemical sample belongs to another heat", ErrInvalidInput)
	}
	if sample.Status != "verified" && sample.Status != "rejected" {
		return model.Heat{}, model.ChemicalSample{}, fmt.Errorf("%w: chemical sample must be terminal before a decision", ErrInvalidInput)
	}
	return heat, sample, nil
}

func validateQualityDecision(code, name, heatCode, sampleCode, reviewer, reason string, decidedAt time.Time, evidence string) error {
	for _, value := range []string{code, name, heatCode, sampleCode, reviewer, reason, evidence} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: decision identity, heat, sample, reviewer, reason and evidence are required", ErrInvalidInput)
		}
	}
	if decidedAt.IsZero() || decidedAt.After(time.Now().UTC().Add(5*time.Minute)) {
		return fmt.Errorf("%w: decision time is invalid", ErrInvalidInput)
	}
	return nil
}

func decisionAuditDetail(item model.QualityDecision) string {
	return fmt.Sprintf("heat=%s sample=%s reviewer=%s reason=%s", item.HeatCode, item.SampleCode, item.Reviewer, item.Reason)
}
