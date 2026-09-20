package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/constants"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/repository"
	"gorm.io/gorm"
)

var ErrReleaseConflict = errors.New("heat release review was already judged")

type ReleaseReviewService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.HeatReleaseReview], error)
	Get(context.Context, uint) (model.HeatReleaseReview, error)
	Open(context.Context, dto.CreateReleaseReview, string, string) (model.HeatReleaseReview, error)
	Judge(context.Context, uint, dto.JudgeReleaseReview, string, string) (model.HeatReleaseReview, error)
	DeleteOpen(context.Context, uint, string, string) error
	PairingBoard(ctx context.Context) ([]dto.ReleasePairingView, error)
}

type releaseReviewService struct {
	reviews   repository.HeatReleaseReviewRepository
	heats     repository.HeatRepository
	samples   repository.ChemicalSampleRepository
	decisions repository.QualityDecisionRepository
	security  SecurityService
}

func NewReleaseReviewService(reviews repository.HeatReleaseReviewRepository, heats repository.HeatRepository,
	samples repository.ChemicalSampleRepository, decisions repository.QualityDecisionRepository,
	security SecurityService) ReleaseReviewService {
	return &releaseReviewService{reviews: reviews, heats: heats, samples: samples, decisions: decisions, security: security}
}

func (s *releaseReviewService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.HeatReleaseReview], error) {
	return s.reviews.List(ctx, query)
}

func (s *releaseReviewService) Get(ctx context.Context, id uint) (model.HeatReleaseReview, error) {
	return s.reviews.Get(ctx, id)
}

// Open pairs the two most recent verified samples of a heat on quality hold and
// creates the single open review for that heat. A review may be opened even
// while the pairing is blocked, so reviewers can record a remelt/scrap verdict;
// the blocking reasons are snapshotted on the review for audit stability.
func (s *releaseReviewService) Open(ctx context.Context, input dto.CreateReleaseReview, actor, requestID string) (model.HeatReleaseReview, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) < 2 {
		return model.HeatReleaseReview{}, fmt.Errorf("%w: review name is required", ErrInvalidInput)
	}
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.HeatReleaseReview, error) {
		heat, err := s.heats.GetByCode(txCtx, normalizeCode(input.HeatCode))
		if err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("resolve review heat: %w", err)
		}
		if heat.Status != string(constants.HeatStateHold) {
			return model.HeatReleaseReview{}, fmt.Errorf("%w: only a heat on quality hold can enter joint release review", ErrInvalidInput)
		}
		if _, err := s.reviews.FindForHeat(txCtx, heat.Code); err == nil {
			return model.HeatReleaseReview{}, fmt.Errorf("%w: heat already has a joint release review", ErrReleaseConflict)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.HeatReleaseReview{}, fmt.Errorf("check existing review: %w", err)
		}
		verified, err := s.samples.ListVerifiedForHeat(txCtx, heat.Code)
		if err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("load verified samples: %w", err)
		}
		evaluation := EvaluatePairing(heat, verified)
		review := model.HeatReleaseReview{
			BaseModel: model.BaseModel{
				Code: reviewCode(heat.Code), Name: name, Status: model.HeatReleaseReviewInitialStatus,
				Version: 1, Description: strings.TrimSpace(input.Description),
			},
			HeatCode:        heat.Code,
			AlloyGrade:      heat.AlloyGrade,
			Reviewer:        strings.TrimSpace(actor),
			PairingBlockers: strings.Join(evaluation.Blockers, "；"),
		}
		if evaluation.First != nil {
			review.FirstSampleID = evaluation.First.ID
			review.FirstSampleCode = evaluation.First.Code
			review.FirstCarbonPct = evaluation.First.CarbonPct
			review.FirstSiliconPct = evaluation.First.SiliconPct
		}
		if evaluation.Second != nil {
			review.SecondSampleID = evaluation.Second.ID
			review.SecondSampleCode = evaluation.Second.Code
			review.SecondCarbonPct = evaluation.Second.CarbonPct
			review.SecondSiliconPct = evaluation.Second.SiliconPct
			review.CarbonDeltaPct = evaluation.CarbonDelta
			review.SiliconDeltaPct = evaluation.SiliconDelta
		}
		if err := s.reviews.Create(txCtx, &review); err != nil {
			if repository.IsDuplicateKeyError(err) {
				return model.HeatReleaseReview{}, fmt.Errorf("%w: heat already has a joint release review", ErrReleaseConflict)
			}
			return model.HeatReleaseReview{}, fmt.Errorf("create release review: %w", err)
		}
		detail := fmt.Sprintf("open joint release review heat=%s pairing=%s blockers=%d", heat.Code, pairingStateLabel(evaluation), len(evaluation.Blockers))
		if err := s.security.Audit(txCtx, actor, requestID, "release-open", "HeatReleaseReview", review.ID, "", review.Status, detail); err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("persist review open audit: %w", err)
		}
		return review, nil
	})
}

// Judge applies the reviewer verdict and performs the whole joint-release
// outcome in one transaction: review verdict, heat acceptance/rejection,
// locking both samples (accept only), the terminal quality decision and every
// audit row. Any failure rolls everything back. The compare-and-set claim plus
// the heat unique decision index ensure repeated or concurrent judgments win at
// most once.
func (s *releaseReviewService) Judge(ctx context.Context, id uint, input dto.JudgeReleaseReview, actor, requestID string) (model.HeatReleaseReview, error) {
	reason := strings.TrimSpace(input.Reason)
	target := strings.TrimSpace(input.Target)
	if !constants.CanTransition(constants.ReleaseReviewTransitions, "open", target) {
		return model.HeatReleaseReview{}, fmt.Errorf("%w: unsupported review verdict %q", ErrInvalidInput, target)
	}
	if target != string(constants.ReleaseReviewAccepted) && len([]rune(reason)) < 3 {
		return model.HeatReleaseReview{}, fmt.Errorf("%w: 返炉或报废必须填写原因", ErrInvalidInput)
	}
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.HeatReleaseReview, error) {
		review, err := s.reviews.Get(txCtx, id)
		if err != nil {
			return model.HeatReleaseReview{}, err
		}
		heat, err := s.heats.GetByCode(txCtx, review.HeatCode)
		if err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("resolve review heat: %w", err)
		}
		if heat.Status != string(constants.HeatStateHold) {
			return model.HeatReleaseReview{}, fmt.Errorf("%w: heat is no longer on quality hold; verdict already applied", ErrReleaseConflict)
		}
		// Re-evaluate against current data so a review cannot accept after the
		// laboratory evidence changed underneath it.
		verified, err := s.samples.ListVerifiedForHeat(txCtx, heat.Code)
		if err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("load verified samples: %w", err)
		}
		evaluation := EvaluatePairing(heat, verified)
		// Acceptance requires two verified samples; when the pairing is incomplete
		// or out of spec the reviewer can still record remelt/scrap with a reason.
		if target == string(constants.ReleaseReviewAccepted) {
			if !evaluation.Eligible() || evaluation.First == nil || evaluation.Second == nil {
				return model.HeatReleaseReview{}, fmt.Errorf("%w: %s", ErrInvalidInput, strings.Join(evaluation.Blockers, "；"))
			}
		}
		// Remelt/scrap may proceed with an incomplete pairing, but the verdict
		// always applies to the review that was opened.
		if evaluation.First == nil {
			return model.HeatReleaseReview{}, fmt.Errorf("%w: joint review needs at least one verified sample before a verdict", ErrInvalidInput)
		}
		if evaluation.First.ID != review.FirstSampleID ||
			(evaluation.Second == nil && review.SecondSampleID != 0) ||
			(evaluation.Second != nil && evaluation.Second.ID != review.SecondSampleID) {
			return model.HeatReleaseReview{}, fmt.Errorf("%w: 配对样本已变化，请刷新后重新合议", ErrReleaseConflict)
		}

		// 1. Claim the review (compare-and-set). Concurrent verdicts lose here.
		if err := s.reviews.ClaimOpen(txCtx, review.ID, input.ExpectedVersion, target); err != nil {
			if errors.Is(err, repository.ErrVersionConflict) {
				return model.HeatReleaseReview{}, fmt.Errorf("%w: 合议已被判定或记录已变更", ErrReleaseConflict)
			}
			return model.HeatReleaseReview{}, fmt.Errorf("claim release review: %w", err)
		}
		now := time.Now().UTC()
		review.Status = target
		review.Version = input.ExpectedVersion + 1
		review.VerdictReason = reason
		review.DecidedAt = now
		review.Reviewer = strings.TrimSpace(actor)
		if err := s.finalizeReviewRow(txCtx, &review, actor, reason, now); err != nil {
			return model.HeatReleaseReview{}, err
		}
		if err := s.security.Audit(txCtx, actor, requestID, "release-judge", "HeatReleaseReview", review.ID,
			"open", target, reviewAuditDetail(review)); err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("persist review verdict audit: %w", err)
		}

		// 2. Lock the two samples only when accepting.
		if target == string(constants.ReleaseReviewAccepted) && evaluation.First != nil && evaluation.Second != nil {
			if err := s.samples.LockVerified(txCtx, evaluation.First.ID); err != nil {
				return model.HeatReleaseReview{}, fmt.Errorf("lock first paired sample: %w", err)
			}
			if err := s.samples.LockVerified(txCtx, evaluation.Second.ID); err != nil {
				return model.HeatReleaseReview{}, fmt.Errorf("lock second paired sample: %w", err)
			}
			if err := s.lockSampleAudit(txCtx, actor, requestID, *evaluation.First); err != nil {
				return model.HeatReleaseReview{}, err
			}
			if err := s.lockSampleAudit(txCtx, actor, requestID, *evaluation.Second); err != nil {
				return model.HeatReleaseReview{}, err
			}
		}

		// 3. Persist the terminal quality decision (existing draft is finalized).
		decision, err := s.persistVerdictDecision(txCtx, heat, review, evaluation, actor, reason, now)
		if err != nil {
			return model.HeatReleaseReview{}, err
		}
		if err := s.security.Audit(txCtx, actor, requestID, "decision", "QualityDecision", decision.ID,
			decisionBeforeStatus(decision, target), decision.Status,
			fmt.Sprintf("joint release review %s -> %s", review.Code, decision.Status)); err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("persist decision audit: %w", err)
		}

		// 4. Derive the terminal heat state.
		heatBefore := heat.Status
		if target == string(constants.ReleaseReviewAccepted) {
			heat.Status = string(constants.HeatStateAccepted)
		} else {
			heat.Status = string(constants.HeatStateRejected)
		}
		heat.Version++
		heat.UpdatedAt = now
		if err := s.heats.Update(txCtx, heat.ID, heat.Version-1, &heat); err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("apply verdict to heat: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "decision", "Heat", heat.ID, heatBefore, heat.Status,
			fmt.Sprintf("heat state derived from joint release review %s", review.Code)); err != nil {
			return model.HeatReleaseReview{}, fmt.Errorf("persist derived heat audit: %w", err)
		}
		return s.reviews.Get(txCtx, id)
	})
}

func (s *releaseReviewService) DeleteOpen(ctx context.Context, id uint, actor, requestID string) error {
	return atomicError(ctx, s.security, func(txCtx context.Context) error {
		review, err := s.reviews.Get(txCtx, id)
		if err != nil {
			return err
		}
		if review.Status != string(constants.ReleaseReviewOpen) {
			return fmt.Errorf("%w: only an open review can be withdrawn", ErrInvalidInput)
		}
		if err := s.reviews.Delete(txCtx, id); err != nil {
			return err
		}
		return s.security.Audit(txCtx, actor, requestID, "release-withdraw", "HeatReleaseReview", id, "open", "deleted", "open joint release review withdrawn")
	})
}

// PairingBoard assembles the read model consumed by the quality page: pairing
// state, both readings, C/Si deltas, blocking reasons and review/decision state.
func (s *releaseReviewService) PairingBoard(ctx context.Context) ([]dto.ReleasePairingView, error) {
	heats, err := s.heats.ListByStatuses(ctx, []string{
		string(constants.HeatStateHold), string(constants.HeatStateAccepted), string(constants.HeatStateRejected),
	}, 100)
	if err != nil {
		return nil, fmt.Errorf("load board heats: %w", err)
	}
	views := make([]dto.ReleasePairingView, 0, len(heats))
	for _, heat := range heats {
		view, err := s.buildPairingView(ctx, heat)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// buildPairingView computes one board row. After a terminal verdict the paired
// samples are shown from the review snapshot / locked rows, so refresh always
// shows the same readings that were judged.
func (s *releaseReviewService) buildPairingView(ctx context.Context, heat model.Heat) (dto.ReleasePairingView, error) {
	samples, err := s.samples.ListForHeat(ctx, heat.Code)
	if err != nil {
		return dto.ReleasePairingView{}, fmt.Errorf("load board samples: %w", err)
	}
	byID := make(map[uint]model.ChemicalSample, len(samples))
	verified := make([]model.ChemicalSample, 0)
	locked := make([]model.ChemicalSample, 0)
	for _, sample := range samples {
		byID[sample.ID] = sample
		switch sample.Status {
		case "verified":
			verified = append(verified, sample)
		case "locked":
			locked = append(locked, sample)
		}
	}

	view := dto.ReleasePairingView{
		HeatCode: heat.Code, HeatName: heat.Name, HeatStatus: heat.Status, HeatVersion: heat.Version,
		AlloyGrade: heat.AlloyGrade, CarbonRange: [2]float64{heat.CarbonMinPct, heat.CarbonMaxPct},
		SiliconRange: [2]float64{heat.SiliconMinPct, heat.SiliconMaxPct}, SulfurMaxPct: heat.SulfurMaxPct,
		PhosphorusMaxPct: heat.PhosphorusMaxPct, VerifiedCount: len(verified),
		Blockers: make([]string, 0),
	}

	review, reviewErr := s.reviews.FindForHeat(ctx, heat.Code)
	switch {
	case reviewErr == nil:
		view.ReviewID = review.ID
		view.ReviewCode = review.Code
		view.ReviewStatus = review.Status
		view.ReviewVersion = review.Version
		view.Reviewer = review.Reviewer
	case errors.Is(reviewErr, gorm.ErrRecordNotFound):
		// no review yet
	default:
		return dto.ReleasePairingView{}, fmt.Errorf("load board review: %w", reviewErr)
	}
	if decision, err := s.decisions.FindForHeat(ctx, heat.Code); err == nil {
		view.DecisionCode = decision.Code
		view.DecisionStatus = decision.Status
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.ReleasePairingView{}, fmt.Errorf("load board decision: %w", err)
	}

	if reviewErr == nil && review.Status != string(constants.ReleaseReviewOpen) {
		// Terminal review: prefer the exact locked samples captured at verdict.
		first, firstOK := byID[review.FirstSampleID]
		second, secondOK := byID[review.SecondSampleID]
		if firstOK {
			view.FirstSample = sampleView(first)
		}
		if secondOK {
			view.SecondSample = sampleView(second)
		}
		view.CarbonDeltaPct = review.CarbonDeltaPct
		view.SiliconDeltaPct = review.SiliconDeltaPct
		view.CarbonDeltaOK = review.CarbonDeltaPct <= model.ChemistryAgreementTolerance+1e-9
		view.SiliconDeltaOK = review.SiliconDeltaPct <= model.ChemistryAgreementTolerance+1e-9
		if review.Status == string(constants.ReleaseReviewAccepted) {
			view.PairingState = "locked-in"
		} else {
			view.PairingState = "closed"
			if review.PairingBlockers != "" {
				view.Blockers = strings.Split(review.PairingBlockers, "；")
			}
		}
		return view, nil
	}

	if heat.Status != string(constants.HeatStateHold) {
		// A terminal heat finalized without a joint review (legacy path): show
		// the most recent locked samples when available so the board still
		// displays the pair and readings consistently.
		view.PairingState = "closed"
		if len(locked) >= 2 {
			view.FirstSample = sampleView(locked[1])
			view.SecondSample = sampleView(locked[0])
			view.CarbonDeltaPct = absFloat(locked[1].CarbonPct - locked[0].CarbonPct)
			view.SiliconDeltaPct = absFloat(locked[1].SiliconPct - locked[0].SiliconPct)
			view.CarbonDeltaOK = view.CarbonDeltaPct <= model.ChemistryAgreementTolerance+1e-9
			view.SiliconDeltaOK = view.SiliconDeltaPct <= model.ChemistryAgreementTolerance+1e-9
			if heat.Status == string(constants.HeatStateAccepted) {
				view.PairingState = "locked-in"
			}
		}
		return view, nil
	}

	// Open or unreviewed heat: evaluate the current verified pairing.
	evaluation := EvaluatePairing(heat, verified)
	view.CarbonDeltaPct = evaluation.CarbonDelta
	view.SiliconDeltaPct = evaluation.SiliconDelta
	view.CarbonDeltaOK = evaluation.CarbonDeltaOK
	view.SiliconDeltaOK = evaluation.SiliconDeltaOK
	view.Blockers = evaluation.Blockers
	if evaluation.First != nil {
		view.FirstSample = sampleView(*evaluation.First)
	}
	if evaluation.Second != nil {
		view.SecondSample = sampleView(*evaluation.Second)
	}
	if heat.Status == string(constants.HeatStateHold) {
		switch {
		case evaluation.Eligible():
			view.PairingState = "ready"
		case len(verified) < 2:
			view.PairingState = "incomplete"
		default:
			view.PairingState = "blocked"
		}
	}
	return view, nil
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func (s *releaseReviewService) finalizeReviewRow(ctx context.Context, review *model.HeatReleaseReview, actor, reason string, now time.Time) error {
	// ClaimOpen already changed status/version atomically; persist the verdict
	// metadata with a conditional update keyed on the claimed version.
	if err := s.reviews.WriteVerdictMeta(ctx, review.ID, review.Version, actor, reason, now); err != nil {
		return fmt.Errorf("persist review verdict: %w", err)
	}
	return nil
}

func (s *releaseReviewService) lockSampleAudit(ctx context.Context, actor, requestID string, sample model.ChemicalSample) error {
	if err := s.security.Audit(ctx, actor, requestID, "transition", "ChemicalSample", sample.ID,
		"verified", "locked", "sample locked by accepted joint release review "+sampleAuditDetail(sample)); err != nil {
		return fmt.Errorf("persist sample lock audit: %w", err)
	}
	return nil
}

func (s *releaseReviewService) persistVerdictDecision(ctx context.Context, heat model.Heat, review model.HeatReleaseReview,
	evaluation PairingEvaluation, actor, reason string, now time.Time) (model.QualityDecision, error) {
	target := string(constants.DecisionTypeAccept)
	if review.Status == string(constants.ReleaseReviewRemelted) {
		target = string(constants.DecisionTypeRemelt)
	} else if review.Status == string(constants.ReleaseReviewScrapped) {
		target = string(constants.DecisionTypeScrap)
	}
	evidence := "JOINT-REVIEW-" + review.Code
	detail := decisionReason(review, evaluation, reason)
	existing, err := s.decisions.FindForHeat(ctx, heat.Code)
	switch {
	case err == nil:
		if existing.Status != "draft" {
			return model.QualityDecision{}, fmt.Errorf("%w: heat already has final quality decision %s", ErrReleaseConflict, existing.Code)
		}
		if err := s.decisions.FinalizeDraft(ctx, existing.ID, target, strings.TrimSpace(actor), detail, evidence); err != nil {
			if errors.Is(err, repository.ErrVersionConflict) {
				return model.QualityDecision{}, fmt.Errorf("%w: quality decision was finalized concurrently", ErrReleaseConflict)
			}
			return model.QualityDecision{}, fmt.Errorf("finalize draft decision: %w", err)
		}
		existing.Status = target
		existing.Reviewer = strings.TrimSpace(actor)
		existing.Reason = detail
		existing.Evidence = evidence
		return existing, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		decision := model.QualityDecision{
			BaseModel: model.BaseModel{
				Code: decisionCode(heat.Code), Name: "炉次放行合议决定 " + heat.Code, Status: target, Version: 1,
				Description: "由炉次放行合议一次性生成",
			},
			HeatCode: heat.Code, SampleCode: joinedSampleCodes(review), Reviewer: strings.TrimSpace(actor), Reason: detail, DecidedAt: now, Evidence: evidence,
		}
		if err := s.decisions.Create(ctx, &decision); err != nil {
			if repository.IsDuplicateKeyError(err) {
				return model.QualityDecision{}, fmt.Errorf("%w: heat already has final quality decision", ErrReleaseConflict)
			}
			return model.QualityDecision{}, fmt.Errorf("create verdict decision: %w", err)
		}
		return decision, nil
	default:
		return model.QualityDecision{}, fmt.Errorf("resolve heat decision: %w", err)
	}
}

func sampleView(sample model.ChemicalSample) *dto.ReleaseSampleView {
	return &dto.ReleaseSampleView{
		ID: sample.ID, Code: sample.Code, Status: sample.Status, CarbonPct: sample.CarbonPct,
		SiliconPct: sample.SiliconPct, SulfurPct: sample.SulfurPct, PhosphorusPct: sample.PhosphorusPct,
		ManganesePct: sample.ManganesePct, SampledAt: sample.SampledAt.UTC().Format(time.RFC3339),
	}
}

func reviewCode(heatCode string) string {
	return "RL-" + strings.TrimPrefix(heatCode, "H-")
}

func joinedSampleCodes(review model.HeatReleaseReview) string {
	if review.SecondSampleCode == "" {
		return review.FirstSampleCode
	}
	return review.FirstSampleCode + "+" + review.SecondSampleCode
}

func decisionCode(heatCode string) string {
	return "QD-RL-" + strings.TrimPrefix(heatCode, "H-")
}

func decisionBeforeStatus(decision model.QualityDecision, target string) string {
	if strings.HasPrefix(decision.Code, "QD-RL-") {
		return ""
	}
	return "draft"
}

func decisionReason(review model.HeatReleaseReview, evaluation PairingEvaluation, reason string) string {
	if review.Status == string(constants.ReleaseReviewAccepted) {
		return fmt.Sprintf("放行合议通过：两份已复核样本碳硅硫磷均在牌号范围内，ΔC=%.3f、ΔSi=%.3f（容差%.2f）",
			evaluation.CarbonDelta, evaluation.SiliconDelta, model.ChemistryAgreementTolerance)
	}
	verdict := "返炉"
	if review.Status == string(constants.ReleaseReviewScrapped) {
		verdict = "报废"
	}
	blockers := review.PairingBlockers
	if blockers == "" {
		blockers = strings.Join(evaluation.Blockers, "；")
	}
	return fmt.Sprintf("放行合议判定%s。原因：%s。阻塞项：%s", verdict, reason, blockers)
}

func pairingStateLabel(evaluation PairingEvaluation) string {
	if evaluation.Eligible() {
		return "ready"
	}
	return "blocked"
}

func reviewAuditDetail(review model.HeatReleaseReview) string {
	return fmt.Sprintf("heat=%s samples=%s+%s ΔC=%.3f ΔSi=%.3f reviewer=%s reason=%s",
		review.HeatCode, review.FirstSampleCode, review.SecondSampleCode,
		review.CarbonDeltaPct, review.SiliconDeltaPct, review.Reviewer, review.VerdictReason)
}
