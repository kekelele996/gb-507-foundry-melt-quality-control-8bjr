package dto

// ReleaseAdjudicationRequest is the write contract for the 炉次放行合议 panel.
// Reviewers do not choose samples themselves: the panel always evaluates the
// two most recent verified samples of the heat. Acceptance is only possible
// when every gate passes; remelt/scrap decisions must always carry a reason.
type ReleaseAdjudicationRequest struct {
	HeatCode string `json:"heatCode" binding:"required,max=64"`
	Decision string `json:"decision" binding:"required,oneof=accept remelt scrap"`
	Reason   string `json:"reason" binding:"required,min=3,max=1000"`
	Evidence string `json:"evidence" binding:"required,min=3,max=2000"`
}
