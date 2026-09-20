package service

import (
	"fmt"
	"math"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
)

// PairingEvaluation is the rule result for a two-sample joint release. Samples
// are returned in sampling order; when fewer than two verified samples exist
// the missing leg is nil and Blockers explains why acceptance is impossible.
type PairingEvaluation struct {
	First          *model.ChemicalSample
	Second         *model.ChemicalSample
	CarbonDelta    float64
	SiliconDelta   float64
	CarbonDeltaOK  bool
	SiliconDeltaOK bool
	Blockers       []string
}

func (e PairingEvaluation) Eligible() bool { return len(e.Blockers) == 0 }

// EvaluatePairing enforces the joint-release rule in one place:
// two verified samples, every C/Si/S/P reading inside the frozen heat grade
// range, and the two carbon readings as well as the two silicon readings within
// 0.05 percentage points of each other.
func EvaluatePairing(heat model.Heat, verified []model.ChemicalSample) PairingEvaluation {
	result := PairingEvaluation{Blockers: make([]string, 0)}
	if len(verified) == 0 {
		result.Blockers = append(result.Blockers, "尚无已复核样本，合议需要两份已复核样本")
		return result
	}
	if len(verified) == 1 {
		result.First = &verified[0]
		result.Blockers = append(result.Blockers, "仅有一份已复核样本，尚缺第二份已复核样本")
	} else {
		first := verified[1]
		second := verified[0]
		result.First = &first
		result.Second = &second
	}
	for leg, sample := range map[string]*model.ChemicalSample{"第一份": result.First, "第二份": result.Second} {
		if sample == nil {
			continue
		}
		if !sample.IsPlausible() {
			result.Blockers = append(result.Blockers, fmt.Sprintf("%s样本 %s 读数超出实验室合理区间", leg, sample.Code))
		}
		if sample.CarbonPct < heat.CarbonMinPct || sample.CarbonPct > heat.CarbonMaxPct {
			result.Blockers = append(result.Blockers, fmt.Sprintf("%s样本 %s 碳含量 %.3f%% 不在牌号范围 %.3f-%.3f%%", leg, sample.Code, sample.CarbonPct, heat.CarbonMinPct, heat.CarbonMaxPct))
		}
		if sample.SiliconPct < heat.SiliconMinPct || sample.SiliconPct > heat.SiliconMaxPct {
			result.Blockers = append(result.Blockers, fmt.Sprintf("%s样本 %s 硅含量 %.3f%% 不在牌号范围 %.3f-%.3f%%", leg, sample.Code, sample.SiliconPct, heat.SiliconMinPct, heat.SiliconMaxPct))
		}
		if sample.SulfurPct > heat.SulfurMaxPct {
			result.Blockers = append(result.Blockers, fmt.Sprintf("%s样本 %s 硫含量 %.3f%% 超过牌号上限 %.3f%%", leg, sample.Code, sample.SulfurPct, heat.SulfurMaxPct))
		}
		if sample.PhosphorusPct > heat.PhosphorusMaxPct {
			result.Blockers = append(result.Blockers, fmt.Sprintf("%s样本 %s 磷含量 %.3f%% 超过牌号上限 %.3f%%", leg, sample.Code, sample.PhosphorusPct, heat.PhosphorusMaxPct))
		}
	}
	if result.First != nil && result.Second != nil {
		result.CarbonDelta = math.Abs(result.First.CarbonPct - result.Second.CarbonPct)
		result.SiliconDelta = math.Abs(result.First.SiliconPct - result.Second.SiliconPct)
		result.CarbonDeltaOK = result.CarbonDelta <= model.ChemistryAgreementTolerance+1e-9
		result.SiliconDeltaOK = result.SiliconDelta <= model.ChemistryAgreementTolerance+1e-9
		if !result.CarbonDeltaOK {
			result.Blockers = append(result.Blockers, fmt.Sprintf("两份样本碳读数差值 %.3f%% 超过 %.2f%% 的合议容差", result.CarbonDelta, model.ChemistryAgreementTolerance))
		}
		if !result.SiliconDeltaOK {
			result.Blockers = append(result.Blockers, fmt.Sprintf("两份样本硅读数差值 %.3f%% 超过 %.2f%% 的合议容差", result.SiliconDelta, model.ChemistryAgreementTolerance))
		}
	}
	return result
}
