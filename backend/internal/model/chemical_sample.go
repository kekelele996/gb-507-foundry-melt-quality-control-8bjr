package model

import "time"

// ChemicalSample stores reproducible five-element chemistry evidence for one heat.
type ChemicalSample struct {
	BaseModel
	HeatCode       string    `json:"heatCode" gorm:"size:64;index;not null"`
	SamplePoint    string    `json:"samplePoint" gorm:"size:80;not null"`
	MethodVersion  string    `json:"methodVersion" gorm:"size:80;not null"`
	Analyst        string    `json:"analyst" gorm:"size:120;index;not null"`
	CarbonPct      float64   `json:"carbonPct" gorm:"not null"`
	SiliconPct     float64   `json:"siliconPct" gorm:"not null"`
	ManganesePct   float64   `json:"manganesePct" gorm:"not null"`
	SulfurPct      float64   `json:"sulfurPct" gorm:"not null"`
	PhosphorusPct  float64   `json:"phosphorusPct" gorm:"not null"`
	SampledAt      time.Time `json:"sampledAt" gorm:"index;not null"`
	Evidence       string    `json:"evidence" gorm:"size:2000"`
}

func (item *ChemicalSample) GetBase() *BaseModel { return &item.BaseModel }

func (item ChemicalSample) TableName() string { return "chemical_samples" }

const ChemicalSampleInitialStatus = "collected"

func (item ChemicalSample) IsPlausible() bool {
	return item.CarbonPct >= 0 && item.CarbonPct <= 6 && item.SiliconPct >= 0 && item.SiliconPct <= 6 &&
		item.ManganesePct >= 0 && item.ManganesePct <= 5 && item.SulfurPct >= 0 && item.SulfurPct <= 1 &&
		item.PhosphorusPct >= 0 && item.PhosphorusPct <= 1
}
