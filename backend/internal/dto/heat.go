package dto

import "time"

// CreateHeat is the public write contract for 炉次. Status is deliberately
// omitted so callers cannot bypass the service state machine.
type CreateHeat struct {
	Code               string    `json:"code" binding:"required,min=2,max=64"`
	Name               string    `json:"name" binding:"required,min=2,max=160"`
	Description        string    `json:"description" binding:"max=1000"`
	FurnaceCode        string    `json:"furnaceCode" binding:"required,max=64"`
	AlloyGrade         string    `json:"alloyGrade" binding:"required,max=80"`
	Owner              string    `json:"owner" binding:"required,max=120"`
	ChargeWeightKg     float64   `json:"chargeWeightKg" binding:"required,gt=0,lte=500000"`
	TargetTemperatureC float64   `json:"targetTemperatureC" binding:"required,gte=500,lte=2200"`
	CarbonMinPct       float64   `json:"carbonMinPct" binding:"gte=0,lte=6"`
	CarbonMaxPct       float64   `json:"carbonMaxPct" binding:"required,gt=0,lte=6"`
	SiliconMinPct      float64   `json:"siliconMinPct" binding:"gte=0,lte=6"`
	SiliconMaxPct      float64   `json:"siliconMaxPct" binding:"required,gt=0,lte=6"`
	SulfurMaxPct       float64   `json:"sulfurMaxPct" binding:"required,gt=0,lte=1"`
	PhosphorusMaxPct   float64   `json:"phosphorusMaxPct" binding:"required,gt=0,lte=1"`
	StartedAt          time.Time `json:"startedAt" binding:"required"`
	Evidence           string    `json:"evidence" binding:"max=2000"`
}

type UpdateHeat struct {
	ExpectedVersion uint      `json:"expectedVersion" binding:"required"`
	CreateHeat
}
