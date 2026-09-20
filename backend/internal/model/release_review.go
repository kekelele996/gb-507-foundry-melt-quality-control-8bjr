package model

import "time"

// HeatReleaseReview is the joint-review aggregate ("炉次放行合议"). It pairs two
// independently verified laboratory samples for one heat and records the reviewer
// verdict. The unique heat index guarantees at most one review per heat, which
// makes repeated or concurrent judgments succeed at most once.
type HeatReleaseReview struct {
	BaseModel
	HeatCode         string    `json:"heatCode" gorm:"size:64;uniqueIndex;not null"`
	FirstSampleID    uint      `json:"firstSampleId" gorm:"index;not null"`
	SecondSampleID   uint      `json:"secondSampleId" gorm:"index;not null"`
	FirstSampleCode  string    `json:"firstSampleCode" gorm:"size:64;index;not null"`
	SecondSampleCode string    `json:"secondSampleCode" gorm:"size:64;index;not null"`
	AlloyGrade       string    `json:"alloyGrade" gorm:"size:80;not null"`
	Reviewer         string    `json:"reviewer" gorm:"size:120;index;not null"`
	VerdictReason    string    `json:"verdictReason" gorm:"size:1000"`
	DecidedAt        time.Time `json:"decidedAt" gorm:"index"`
	// Readings are snapshotted at review creation so the pairing board stays
	// consistent even if unrelated laboratory records change later.
	FirstCarbonPct   float64 `json:"firstCarbonPct" gorm:"not null"`
	SecondCarbonPct  float64 `json:"secondCarbonPct" gorm:"not null"`
	FirstSiliconPct  float64 `json:"firstSiliconPct" gorm:"not null"`
	SecondSiliconPct float64 `json:"secondSiliconPct" gorm:"not null"`
	CarbonDeltaPct   float64 `json:"carbonDeltaPct" gorm:"not null"`
	SiliconDeltaPct  float64 `json:"siliconDeltaPct" gorm:"not null"`
	PairingBlockers  string  `json:"pairingBlockers" gorm:"size:1000;not null"`
}

func (item *HeatReleaseReview) GetBase() *BaseModel { return &item.BaseModel }

func (item HeatReleaseReview) TableName() string { return "heat_release_reviews" }

const HeatReleaseReviewInitialStatus = "open"

// ChemistryAgreementTolerance is the maximum reading difference between the two
// paired samples for carbon and silicon ("各不超过零点零五").
const ChemistryAgreementTolerance = 0.05
