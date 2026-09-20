package dto

import "time"

// CreateFurnace is the public write contract for 炉台. Status is deliberately
// omitted so callers cannot bypass the service state machine.
type CreateFurnace struct {
	Code             string    `json:"code" binding:"required,min=2,max=64"`
	Name             string    `json:"name" binding:"required,min=2,max=160"`
	Description      string    `json:"description" binding:"max=1000"`
	PlantArea        string    `json:"plantArea" binding:"required,max=120"`
	FurnaceType      string    `json:"furnaceType" binding:"required,max=80"`
	CapacityTonnes   float64   `json:"capacityTonnes" binding:"required,gt=0,lte=500"`
	SupportedAlloys  string    `json:"supportedAlloys" binding:"required,max=500"`
	MaxTemperatureC  float64   `json:"maxTemperatureC" binding:"required,gte=500,lte=2200"`
	Operator         string    `json:"operator" binding:"required,max=120"`
	LastInspectionAt time.Time `json:"lastInspectionAt" binding:"required"`
	Evidence         string    `json:"evidence" binding:"max=2000"`
}

type UpdateFurnace struct {
	ExpectedVersion uint      `json:"expectedVersion" binding:"required"`
	Name            string    `json:"name" binding:"required,min=2,max=160"`
	Description     string    `json:"description" binding:"max=1000"`
	PlantArea       string    `json:"plantArea" binding:"required,max=120"`
	FurnaceType     string    `json:"furnaceType" binding:"required,max=80"`
	CapacityTonnes  float64   `json:"capacityTonnes" binding:"required,gt=0,lte=500"`
	SupportedAlloys string    `json:"supportedAlloys" binding:"required,max=500"`
	MaxTemperatureC float64   `json:"maxTemperatureC" binding:"required,gte=500,lte=2200"`
	Operator        string    `json:"operator" binding:"required,max=120"`
	LastInspectionAt time.Time `json:"lastInspectionAt" binding:"required"`
	Evidence        string    `json:"evidence" binding:"max=2000"`
}
