package dto

// CreateReleaseReview opens a joint-review session ("发起放行合议") for one
// heat. The two verified samples are paired by the service, never by the caller.
type CreateReleaseReview struct {
	Name        string `json:"name" binding:"required,min=2,max=160"`
	HeatCode    string `json:"heatCode" binding:"required,max=64"`
	Description string `json:"description" binding:"max:1000"`
}

// JudgeReleaseReview carries the reviewer verdict. accept requires a passing
// pairing; remelt/scrap require a filled reason.
type JudgeReleaseReview struct {
	Target          string `json:"target" binding:"required,oneof=accepted remelted scrapped"`
	ExpectedVersion uint   `json:"expectedVersion" binding:"required"`
	Reason          string `json:"reason" binding:"max=1000"`
}

// ReleaseSampleView is one leg of a paired sample on the quality board.
type ReleaseSampleView struct {
	ID            uint    `json:"id"`
	Code          string  `json:"code"`
	Status        string  `json:"status"`
	CarbonPct     float64 `json:"carbonPct"`
	SiliconPct    float64 `json:"siliconPct"`
	SulfurPct     float64 `json:"sulfurPct"`
	PhosphorusPct float64 `json:"phosphorusPct"`
	ManganesePct  float64 `json:"manganesePct"`
	SampledAt     string  `json:"sampledAt"`
}

// ReleasePairingView shows the joint-release pairing state for one heat,
// including the two readings, deltas and every blocking reason.
type ReleasePairingView struct {
	HeatCode         string             `json:"heatCode"`
	HeatName         string             `json:"heatName"`
	HeatStatus       string             `json:"heatStatus"`
	HeatVersion      uint               `json:"heatVersion"`
	AlloyGrade       string             `json:"alloyGrade"`
	CarbonRange      [2]float64         `json:"carbonRange"`
	SiliconRange     [2]float64         `json:"siliconRange"`
	SulfurMaxPct     float64            `json:"sulfurMaxPct"`
	PhosphorusMaxPct float64            `json:"phosphorusMaxPct"`
	PairingState     string             `json:"pairingState"` // incomplete / blocked / ready / locked-in
	VerifiedCount    int                `json:"verifiedCount"`
	FirstSample      *ReleaseSampleView `json:"firstSample"`
	SecondSample     *ReleaseSampleView `json:"secondSample"`
	CarbonDeltaPct   float64            `json:"carbonDeltaPct"`
	SiliconDeltaPct  float64            `json:"siliconDeltaPct"`
	CarbonDeltaOK    bool               `json:"carbonDeltaOk"`
	SiliconDeltaOK   bool               `json:"siliconDeltaOk"`
	Blockers         []string           `json:"blockers"`
	ReviewID         uint               `json:"reviewId"`
	ReviewCode       string             `json:"reviewCode"`
	ReviewStatus     string             `json:"reviewStatus"`
	ReviewVersion    uint               `json:"reviewVersion"`
	Reviewer         string             `json:"reviewer"`
	DecisionCode     string             `json:"decisionCode"`
	DecisionStatus   string             `json:"decisionStatus"`
}
