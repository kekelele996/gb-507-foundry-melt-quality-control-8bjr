package constants

import "testing"

func TestFurnaceTransitionGraph(t *testing.T) {
	if !CanTransition(FurnaceTransitions, "available", "charging") {
		t.Fatalf("expected available -> charging transition to be allowed")
	}
	if CanTransition(FurnaceTransitions, "available", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}
