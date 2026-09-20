package model

import "time"

// Heat binds a melt to a furnace and an explicit chemistry specification.
type Heat struct {
	BaseModel
	FurnaceCode        string    `json:"furnaceCode" gorm:"size:64;index;not null"`
	AlloyGrade         string    `json:"alloyGrade" gorm:"size:80;index;not null"`
	Owner              string    `json:"owner" gorm:"size:120;index;not null"`
	ChargeWeightKg     float64   `json:"chargeWeightKg" gorm:"not null"`
	TargetTemperatureC float64   `json:"targetTemperatureC" gorm:"not null"`
	CarbonMinPct       float64   `json:"carbonMinPct" gorm:"not null"`
	CarbonMaxPct       float64   `json:"carbonMaxPct" gorm:"not null"`
	SiliconMinPct      float64   `json:"siliconMinPct" gorm:"not null"`
	SiliconMaxPct      float64   `json:"siliconMaxPct" gorm:"not null"`
	SulfurMaxPct       float64   `json:"sulfurMaxPct" gorm:"not null"`
	PhosphorusMaxPct   float64   `json:"phosphorusMaxPct" gorm:"not null"`
	StartedAt          time.Time `json:"startedAt" gorm:"index;not null"`
	Evidence           string    `json:"evidence" gorm:"size:2000"`
}

func (item *Heat) GetBase() *BaseModel { return &item.BaseModel }

func (item Heat) TableName() string { return "heats" }

const HeatInitialStatus = "charged"
