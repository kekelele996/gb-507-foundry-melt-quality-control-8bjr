package dto

import "time"

// CreateChemicalSample is the public write contract for 化验样本. Status is deliberately
// omitted so callers cannot bypass the service state machine.
type CreateChemicalSample struct {
	Code          string    `json:"code" binding:"required,min=2,max=64"`
	Name          string    `json:"name" binding:"required,min=2,max=160"`
	Description   string    `json:"description" binding:"max=1000"`
	HeatCode      string    `json:"heatCode" binding:"required,max=64"`
	SamplePoint   string    `json:"samplePoint" binding:"required,max=80"`
	MethodVersion string    `json:"methodVersion" binding:"required,max=80"`
	Analyst       string    `json:"analyst" binding:"max=120"`
	CarbonPct     float64   `json:"carbonPct" binding:"gte=0,lte=6"`
	SiliconPct    float64   `json:"siliconPct" binding:"gte=0,lte=6"`
	ManganesePct  float64   `json:"manganesePct" binding:"gte=0,lte=5"`
	SulfurPct     float64   `json:"sulfurPct" binding:"gte=0,lte=1"`
	PhosphorusPct float64   `json:"phosphorusPct" binding:"gte=0,lte=1"`
	SampledAt     time.Time `json:"sampledAt" binding:"required"`
	Evidence      string    `json:"evidence" binding:"required,min=3,max=2000"`
}

type UpdateChemicalSample struct {
	ExpectedVersion uint `json:"expectedVersion" binding:"required"`
	CreateChemicalSample
}
