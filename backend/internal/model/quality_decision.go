package model

import "time"

// QualityDecision is the immutable sign-off tying a heat to laboratory evidence.
// A heat-release panel decision pairs two independently verified samples; the
// second sample code is recorded in PairedSampleCode (empty for legacy single
// evidence decisions created before the panel workflow).
type QualityDecision struct {
	BaseModel
	HeatCode         string    `json:"heatCode" gorm:"size:64;uniqueIndex;not null"`
	SampleCode       string    `json:"sampleCode" gorm:"size:64;index;not null"`
	PairedSampleCode string    `json:"pairedSampleCode" gorm:"size:64"`
	Reviewer         string    `json:"reviewer" gorm:"size:120;index;not null"`
	Reason           string    `json:"reason" gorm:"size:1000;not null"`
	Conditions       string    `json:"conditions" gorm:"size:1000"`
	DecidedAt        time.Time `json:"decidedAt" gorm:"index;not null"`
	Evidence         string    `json:"evidence" gorm:"size:2000;not null"`
}

func (item *QualityDecision) GetBase() *BaseModel { return &item.BaseModel }

func (item QualityDecision) TableName() string { return "quality_decisions" }

const QualityDecisionInitialStatus = "draft"
