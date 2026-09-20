package model

import "time"

// Furnace records the physical and process capability of a melting furnace.
type Furnace struct {
	BaseModel
	PlantArea        string    `json:"plantArea" gorm:"size:120;index;not null"`
	FurnaceType      string    `json:"furnaceType" gorm:"size:80;index;not null"`
	CapacityTonnes   float64   `json:"capacityTonnes" gorm:"not null"`
	SupportedAlloys  string    `json:"supportedAlloys" gorm:"size:500;not null"`
	MaxTemperatureC  float64   `json:"maxTemperatureC" gorm:"not null"`
	Operator         string    `json:"operator" gorm:"size:120;index;not null"`
	LastInspectionAt time.Time `json:"lastInspectionAt" gorm:"index;not null"`
	Evidence         string    `json:"evidence" gorm:"size:2000"`
}

func (item *Furnace) GetBase() *BaseModel { return &item.BaseModel }

func (item Furnace) TableName() string { return "furnaces" }

const FurnaceInitialStatus = "available"
