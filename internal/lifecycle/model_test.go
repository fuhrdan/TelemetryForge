package lifecycle

import "testing"

func TestLifecycleTransitions(t *testing.T) {
    if !CanTransition(StateDraft, StateShadow) || !CanTransition(StateApproved, StateActive) || !CanTransition(StateRetired, StateActive) {
        t.Fatal("expected valid lifecycle transitions")
    }
    if CanTransition(StateDraft, StateActive) || CanTransition(StateActive, StateShadow) {
        t.Fatal("unexpected unsafe lifecycle transition")
    }
}

func TestWaiverRequiresReason(t *testing.T) {
    if ValidateEvidence("waived", "") == nil { t.Fatal("waiver without reason must fail") }
    if ValidateEvidence("waived", "reviewed risk") != nil { t.Fatal("documented waiver should pass") }
}
