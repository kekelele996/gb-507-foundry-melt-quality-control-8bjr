package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/constants"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/repository"
	"gorm.io/gorm"
)

// Release pair statuses shown on the 质量判定 page.
const (
	PairStatusReady    = "paired-ready"
	PairStatusShort    = "pairing-short"
	PairStatusAbsent   = "pairing-absent"
	PairStatusAdjudged = "already-adjudged"
)

// releaseNoEvidenceMarker is recorded in the single-sample reference column
// when a heat is returned or scrapped without a verified sample pair; the
// column is NOT NULL for legacy schema compatibility.
const releaseNoEvidenceMarker = "N/A"

// ReleaseElementCheck is one element reading compared with the heat's frozen
// alloy specification.
type ReleaseElementCheck struct {
	Element string  `json:"element"`
	Value   float64 `json:"value"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Pass    bool    `json:"pass"`
}

// ReleaseSampleView is one verified sample with its element-by-element check.
type ReleaseSampleView struct {
	model.ChemicalSample
	ElementResults []ReleaseElementCheck `json:"elementResults"`
}

// ReleasePanel is the read model driving the heat-release panel. It is rebuilt
// from committed rows on every request, so refreshing the page always returns
// the same pairing, readings and blockers.
type ReleasePanel struct {
	Heat         model.Heat             `json:"heat"`
	Decision     *model.QualityDecision `json:"decision"`
	PairStatus   string                 `json:"pairStatus"`
	Samples      []ReleaseSampleView    `json:"samples"`
	CarbonDelta  float64                `json:"carbonDelta"`
	SiliconDelta float64                `json:"siliconDelta"`
	Blockers     []string               `json:"blockers"`
	CanAccept    bool                   `json:"canAccept"`
}

type ReleaseReviewService interface {
	ListPanelHeats(context.Context) ([]model.Heat, error)
	GetPanel(context.Context, string) (ReleasePanel, error)
	Adjudicate(context.Context, dto.ReleaseAdjudicationRequest, string, string) (model.QualityDecision, error)
}

type releaseReviewService struct {
	heats     repository.HeatRepository
	samples   repository.ChemicalSampleRepository
	decisions repository.QualityDecisionRepository
	security  SecurityService
}

func NewReleaseReviewService(heats repository.HeatRepository, samples repository.ChemicalSampleRepository,
	decisions repository.QualityDecisionRepository, security SecurityService) ReleaseReviewService {
	return &releaseReviewService{heats: heats, samples: samples, decisions: decisions, security: security}
}

func (s *releaseReviewService) ListPanelHeats(ctx context.Context) ([]model.Heat, error) {
	return s.heats.ListByStatuses(ctx, []string{
		string(constants.HeatStateHold), string(constants.HeatStateAccepted), string(constants.HeatStateRejected),
	})
}

func (s *releaseReviewService) GetPanel(ctx context.Context, heatCode string) (ReleasePanel, error) {
	heat, err := s.heats.GetByCode(ctx, normalizeCode(heatCode))
	if err != nil {
		return ReleasePanel{}, fmt.Errorf("resolve release heat: %w", err)
	}
	panel := ReleasePanel{Heat: heat, PairStatus: PairStatusAbsent, Samples: []ReleaseSampleView{}, Blockers: []string{}}

	if existing, findErr := s.decisions.GetByHeatCode(ctx, heat.Code); findErr == nil {
		panel.Decision = &existing
		panel.PairStatus = PairStatusAdjudged
		panel.Blockers = append(panel.Blockers, "该炉次已完成放行判定，决定不可撤销")
		// Still surface the paired evidence (now locked on accept) so the page
		// renders the same readings after refresh.
		released, listErr := s.samples.ListReleasedByHeat(ctx, heat.Code)
		if listErr != nil {
			return ReleasePanel{}, fmt.Errorf("list released samples: %w", listErr)
		}
		if existing.PairedSampleCode != "" {
			released = filterDecisionPair(released, existing.SampleCode, existing.PairedSampleCode)
		}
		for _, sample := range released {
			panel.Samples = append(panel.Samples, ReleaseSampleView{ChemicalSample: sample, ElementResults: elementChecks(heat, sample)})
		}
		if len(panel.Samples) >= 2 {
			panel.CarbonDelta = math.Abs(panel.Samples[0].CarbonPct - panel.Samples[1].CarbonPct)
			panel.SiliconDelta = math.Abs(panel.Samples[0].SiliconPct - panel.Samples[1].SiliconPct)
		}
		return panel, nil
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return ReleasePanel{}, fmt.Errorf("lookup release decision: %w", findErr)
	}

	if heat.Status != string(constants.HeatStateHold) {
		panel.Blockers = append(panel.Blockers, fmt.Sprintf("炉次当前状态为 %s，须处于质量待判（hold）才能合议", heat.Status))
	}

	verified, err := s.samples.ListVerifiedByHeat(ctx, heat.Code)
	if err != nil {
		return ReleasePanel{}, fmt.Errorf("list verified samples: %w", err)
	}
	for _, sample := range verified {
		panel.Samples = append(panel.Samples, ReleaseSampleView{ChemicalSample: sample, ElementResults: elementChecks(heat, sample)})
	}

	switch len(panel.Samples) {
	case 0:
		panel.Blockers = append(panel.Blockers, "缺少已复核（verified）样本，放行至少需要两份独立复核样本")
	case 1:
		panel.PairStatus = PairStatusShort
		panel.Blockers = append(panel.Blockers, "仅有一份已复核样本，放行须补充第二份独立复核样本")
	default:
		panel.PairStatus = PairStatusReady
		first, second := panel.Samples[0], panel.Samples[1]
		panel.CarbonDelta = math.Abs(first.CarbonPct - second.CarbonPct)
		panel.SiliconDelta = math.Abs(first.SiliconPct - second.SiliconPct)
		panel.Blockers = append(panel.Blockers, pairBlockers(heat, first.ChemicalSample, second.ChemicalSample)...)
	}
	panel.CanAccept = heat.Status == string(constants.HeatStateHold) && panel.PairStatus == PairStatusReady && len(panel.Blockers) == 0
	return panel, nil
}

func (s *releaseReviewService) Adjudicate(ctx context.Context, input dto.ReleaseAdjudicationRequest, actor, requestID string) (model.QualityDecision, error) {
	return atomicValue(ctx, s.security, func(txCtx context.Context) (model.QualityDecision, error) {
		heatCode := normalizeCode(input.HeatCode)
		decision := strings.TrimSpace(input.Decision)
		reason := strings.TrimSpace(input.Reason)
		evidence := strings.TrimSpace(input.Evidence)
		if heatCode == "" || reason == "" || evidence == "" {
			return model.QualityDecision{}, fmt.Errorf("%w: heat, reason and evidence are required", ErrInvalidInput)
		}

		heat, err := s.heats.LockByCode(txCtx, heatCode)
		if err != nil {
			return model.QualityDecision{}, fmt.Errorf("resolve release heat: %w", err)
		}
		// Duplicate guard inside the transaction; the unique heat_code index
		// is the final defense, so concurrent adjudications only succeed once.
		if existing, findErr := s.decisions.GetByHeatCode(txCtx, heat.Code); findErr == nil {
			return model.QualityDecision{}, fmt.Errorf("%w: heat %s already decided as %s by %s",
				ErrConflict, heat.Code, existing.Status, existing.Reviewer)
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return model.QualityDecision{}, fmt.Errorf("lookup release decision: %w", findErr)
		}

		verified, err := s.samples.ListVerifiedByHeat(txCtx, heat.Code)
		if err != nil {
			return model.QualityDecision{}, fmt.Errorf("list verified samples: %w", err)
		}
		panel := ReleasePanel{Heat: heat, Samples: make([]ReleaseSampleView, 0, len(verified)), Blockers: []string{}}
		for _, sample := range verified {
			panel.Samples = append(panel.Samples, ReleaseSampleView{ChemicalSample: sample, ElementResults: elementChecks(heat, sample)})
		}
		first, second, paired := releasePair(verified)
		panel.PairStatus = pairStatus(len(verified))
		if paired {
			panel.CarbonDelta = math.Abs(first.CarbonPct - second.CarbonPct)
			panel.SiliconDelta = math.Abs(first.SiliconPct - second.SiliconPct)
			panel.Blockers = pairBlockers(heat, first, second)
		} else {
			panel.Blockers = append(panel.Blockers, pairShortageBlocker(len(verified)))
		}
		if heat.Status != string(constants.HeatStateHold) {
			panel.Blockers = append(panel.Blockers, fmt.Sprintf("炉次当前状态为 %s，须处于质量待判（hold）才能合议", heat.Status))
		}
		panel.CanAccept = heat.Status == string(constants.HeatStateHold) && paired && len(panel.Blockers) == 0

		if decision == string(constants.DecisionTypeAccept) && !panel.CanAccept {
			return model.QualityDecision{}, fmt.Errorf("%w: 合格放行被阻塞：%s", ErrInvalidInput, strings.Join(panel.Blockers, "；"))
		}

		now := time.Now().UTC()
		item := model.QualityDecision{
			BaseModel: model.BaseModel{
				Code: releaseDecisionCode(heat.Code), Name: "炉次放行合议-" + heat.Code, Status: decision,
				Version: 1, Description: "炉次放行合议原子判定",
			},
			HeatCode:   heat.Code,
			SampleCode: releaseNoEvidenceMarker,
			Reviewer:   strings.TrimSpace(actor),
			Reason:     reason,
			DecidedAt:  now,
			Evidence:   evidence,
		}
		if paired {
			item.SampleCode = first.Code
			item.PairedSampleCode = second.Code
			if decision == string(constants.DecisionTypeAccept) {
				item.Conditions = fmt.Sprintf("配对 %s/%s；ΔC=%.3f%% ΔSi=%.3f%%，双样本四元素均在牌号范围内",
					first.Code, second.Code, panel.CarbonDelta, panel.SiliconDelta)
			}
		} else if len(verified) > 0 {
			// Return/scrap without a pair can still reference the single
			// available verified sample as supporting evidence.
			item.SampleCode = verified[0].Code
		}
		if err := s.decisions.Create(txCtx, &item); err != nil {
			if isDuplicateKey(err) {
				return model.QualityDecision{}, fmt.Errorf("%w: duplicate decision for heat %s", ErrConflict, heat.Code)
			}
			return model.QualityDecision{}, fmt.Errorf("create release decision: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "release", "QualityDecision", item.ID, "", decision,
			releaseDecisionAuditDetail(item, panel)); err != nil {
			return model.QualityDecision{}, fmt.Errorf("persist release decision audit: %w", err)
		}

		heatBefore := heat.Status
		if decision == string(constants.DecisionTypeAccept) {
			// Lock both paired samples atomically: they become immutable
			// laboratory evidence backing this acceptance.
			for _, sample := range []model.ChemicalSample{first, second} {
				if err := s.lockSample(txCtx, sample, actor, requestID, item.Code); err != nil {
					return model.QualityDecision{}, err
				}
			}
			heat.Status = string(constants.HeatStateAccepted)
		} else {
			heat.Status = string(constants.HeatStateRejected)
		}
		heat.Version++
		heat.UpdatedAt = now
		if err := s.heats.Update(txCtx, heat.ID, heat.Version-1, &heat); err != nil {
			if errors.Is(err, repository.ErrVersionConflict) {
				return model.QualityDecision{}, fmt.Errorf("%w: heat changed concurrently", ErrConflict)
			}
			return model.QualityDecision{}, fmt.Errorf("apply release decision to heat: %w", err)
		}
		if err := s.security.Audit(txCtx, actor, requestID, "release", "Heat", heat.ID, heatBefore, heat.Status,
			"heat state derived from release panel decision "+item.Code); err != nil {
			return model.QualityDecision{}, fmt.Errorf("persist release heat audit: %w", err)
		}
		return item, nil
	})
}

func (s *releaseReviewService) lockSample(ctx context.Context, sample model.ChemicalSample, actor, requestID, decisionCode string) error {
	before := sample.Status
	sample.Status = constants.ChemicalSampleStatusLocked
	sample.Version++
	sample.UpdatedAt = time.Now().UTC()
	if err := s.samples.Update(ctx, sample.ID, sample.Version-1, &sample); err != nil {
		if errors.Is(err, repository.ErrVersionConflict) {
			return fmt.Errorf("%w: sample %s changed concurrently", ErrConflict, sample.Code)
		}
		return fmt.Errorf("lock paired sample %s: %w", sample.Code, err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "lock", "ChemicalSample", sample.ID, before,
		constants.ChemicalSampleStatusLocked, "paired evidence locked by release decision "+decisionCode); err != nil {
		return fmt.Errorf("persist sample lock audit: %w", err)
	}
	return nil
}

func releasePair(verified []model.ChemicalSample) (model.ChemicalSample, model.ChemicalSample, bool) {
	if len(verified) < constants.ReleaseRequiredVerifiedPairs {
		return model.ChemicalSample{}, model.ChemicalSample{}, false
	}
	return verified[0], verified[1], true
}

// filterDecisionPair reduces released samples to the exact pair recorded on a
// committed decision, preserving the sampled_at ordering.
func filterDecisionPair(samples []model.ChemicalSample, firstCode, secondCode string) []model.ChemicalSample {
	pair := make([]model.ChemicalSample, 0, 2)
	for _, sample := range samples {
		if sample.Code == firstCode || sample.Code == secondCode {
			pair = append(pair, sample)
		}
	}
	return pair
}

func pairStatus(verifiedCount int) string {
	switch {
	case verifiedCount >= constants.ReleaseRequiredVerifiedPairs:
		return PairStatusReady
	case verifiedCount == 1:
		return PairStatusShort
	default:
		return PairStatusAbsent
	}
}

func pairShortageBlocker(verifiedCount int) string {
	if verifiedCount == 0 {
		return "缺少已复核（verified）样本，放行至少需要两份独立复核样本"
	}
	return "仅有一份已复核样本，放行须补充第二份独立复核样本"
}

// withinTolerance compares chemistry readings with a small epsilon so stored
// binary floating-point representations of the exact 0.05 boundary pass.
func withinTolerance(delta, tolerance float64) bool {
	return delta <= tolerance+1e-9
}

// pairBlockers evaluates the release gates for the two paired samples: all of
// carbon, silicon, sulfur and phosphorus must be inside the frozen alloy
// specification for both samples, and carbon/silicon readings must reproduce
// within 0.05 percentage points.
func pairBlockers(heat model.Heat, first, second model.ChemicalSample) []string {
	blockers := []string{}
	outOfSpec := 0
	for _, sample := range []model.ChemicalSample{first, second} {
		for _, result := range elementChecks(heat, sample) {
			if !result.Pass {
				outOfSpec++
			}
		}
	}
	if outOfSpec > 0 {
		blockers = append(blockers, fmt.Sprintf("配对样本中有 %d 项碳/硅/硫/磷读数超出牌号 %s 冻结范围", outOfSpec, heat.AlloyGrade))
	}
	if delta := math.Abs(first.CarbonPct - second.CarbonPct); !withinTolerance(delta, constants.ReleaseCarbonTolerancePct) {
		blockers = append(blockers, fmt.Sprintf("两份样本碳读数差值 %.3f%% 超过 %.2f%% 复现允差", delta, constants.ReleaseCarbonTolerancePct))
	}
	if delta := math.Abs(first.SiliconPct - second.SiliconPct); !withinTolerance(delta, constants.ReleaseSiliconTolerancePct) {
		blockers = append(blockers, fmt.Sprintf("两份样本硅读数差值 %.3f%% 超过 %.2f%% 复现允差", delta, constants.ReleaseSiliconTolerancePct))
	}
	return blockers
}

func elementChecks(heat model.Heat, sample model.ChemicalSample) []ReleaseElementCheck {
	return []ReleaseElementCheck{
		{Element: "C", Value: sample.CarbonPct, Min: heat.CarbonMinPct, Max: heat.CarbonMaxPct,
			Pass: sample.CarbonPct >= heat.CarbonMinPct && sample.CarbonPct <= heat.CarbonMaxPct},
		{Element: "Si", Value: sample.SiliconPct, Min: heat.SiliconMinPct, Max: heat.SiliconMaxPct,
			Pass: sample.SiliconPct >= heat.SiliconMinPct && sample.SiliconPct <= heat.SiliconMaxPct},
		{Element: "S", Value: sample.SulfurPct, Min: 0, Max: heat.SulfurMaxPct, Pass: sample.SulfurPct <= heat.SulfurMaxPct},
		{Element: "P", Value: sample.PhosphorusPct, Min: 0, Max: heat.PhosphorusMaxPct, Pass: sample.PhosphorusPct <= heat.PhosphorusMaxPct},
	}
}

func releaseDecisionCode(heatCode string) string {
	safe := strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToUpper(heatCode))
	buffer := make([]byte, 2)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("QDR-%s-%d", safe, time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("QDR-%s-%s-%X", safe, time.Now().UTC().Format("0102150405"), buffer)
}

func releaseDecisionAuditDetail(item model.QualityDecision, panel ReleasePanel) string {
	paired := item.PairedSampleCode
	if paired == "" {
		paired = releaseNoEvidenceMarker
	}
	return fmt.Sprintf("release-panel heat=%s samples=%s+%s reviewer=%s decision=%s dC=%.3f dSi=%.3f blockers=%d reason=%s",
		item.HeatCode, item.SampleCode, paired, item.Reviewer, item.Status,
		panel.CarbonDelta, panel.SiliconDelta, len(panel.Blockers), item.Reason)
}

// isDuplicateKey reports whether a write violated a unique index. PostgreSQL
// returns SQLSTATE 23505; MySQL returns 1062; SQLite returns UNIQUE constraint.
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "duplicate key") || strings.Contains(message, "23505")
}
