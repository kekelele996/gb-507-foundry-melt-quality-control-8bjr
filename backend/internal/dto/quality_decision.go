package dto

import "time"

// CreateQualityDecision is the public write contract for 质量决定. Status is deliberately
// omitted so callers cannot bypass the service state machine.
type CreateQualityDecision struct {
	Code        string    `json:"code" binding:"required,min=2,max=64"`
	Name        string    `json:"name" binding:"required,min=2,max=160"`
	Description string    `json:"description" binding:"max=1000"`
	HeatCode    string    `json:"heatCode" binding:"required,max=64"`
	SampleCode  string    `json:"sampleCode" binding:"required,max=64"`
	Reviewer    string    `json:"reviewer" binding:"max=120"`
	Reason      string    `json:"reason" binding:"required,min=3,max=1000"`
	Conditions  string    `json:"conditions" binding:"max=1000"`
	DecidedAt   time.Time `json:"decidedAt" binding:"required"`
	Evidence    string    `json:"evidence" binding:"required,min=3,max=2000"`
}

type UpdateQualityDecision struct {
	ExpectedVersion uint      `json:"expectedVersion" binding:"required"`
	CreateQualityDecision
}
