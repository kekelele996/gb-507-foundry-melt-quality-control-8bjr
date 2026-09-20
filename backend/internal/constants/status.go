package constants

// Shared status values are mirrored in frontend/src/types/status.ts. Keeping
// the lists explicit makes state-machine drift visible during code review.

type HeatState string

const (
	HeatStateCharged  HeatState = "charged"
	HeatStateMelting  HeatState = "melting"
	HeatStateSampling HeatState = "sampling"
	HeatStateHold     HeatState = "hold"
	HeatStateAccepted HeatState = "accepted"
	HeatStateRejected HeatState = "rejected"
)

var AllHeatState = []string{"charged", "melting", "sampling", "hold", "accepted", "rejected"}

type DecisionType string

const (
	DecisionTypeAccept DecisionType = "accept"
	DecisionTypeRemelt DecisionType = "remelt"
	DecisionTypeScrap  DecisionType = "scrap"
)

var AllDecisionType = []string{"accept", "remelt", "scrap"}

var FurnaceTransitions = map[string]map[string]bool{
	"available":   {"charging": true, "maintenance": true},
	"charging":    {"available": true, "maintenance": true},
	"maintenance": {"available": true, "locked": true},
	"locked":      {"maintenance": true},
}

var HeatTransitions = map[string]map[string]bool{
	"charged":  {"melting": true},
	"melting":  {"sampling": true},
	"sampling": {"hold": true},
	"hold":     {},
	"accepted": {},
	"rejected": {},
}

var ChemicalSampleTransitions = map[string]map[string]bool{
	"collected": {"testing": true},
	"testing":   {"verified": true, "rejected": true},
	"verified":  {},
	"rejected":  {},
}

var QualityDecisionTransitions = map[string]map[string]bool{
	"draft":  {"accept": true, "remelt": true, "scrap": true},
	"accept": {},
	"remelt": {},
	"scrap":  {},
}

func CanTransition(graph map[string]map[string]bool, from, to string) bool {
	targets, exists := graph[from]
	return exists && targets[to]
}
