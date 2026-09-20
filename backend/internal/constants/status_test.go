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

func TestReleaseReviewGraphAndSampleLock(t *testing.T) {
	if !CanTransition(ChemicalSampleTransitions, "verified", "locked") {
		t.Fatal("verified samples must lock after a passing joint release")
	}
	if CanTransition(ChemicalSampleTransitions, "locked", "verified") {
		t.Fatal("locked samples must never reopen")
	}
	for _, target := range []string{"accepted", "remelted", "scrapped"} {
		if !CanTransition(ReleaseReviewTransitions, "open", target) {
			t.Fatalf("open review must be able to reach %s", target)
		}
		if CanTransition(ReleaseReviewTransitions, target, "open") {
			t.Fatalf("%s review must be terminal", target)
		}
	}
}
