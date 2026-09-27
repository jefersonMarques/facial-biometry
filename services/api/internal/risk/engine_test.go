package risk

import "testing"

func TestVerificationDecision(t *testing.T) {
	engine := NewEngine(0.68, 0.55, 0.363, true)

	if decision := engine.VerificationDecision(0.80, 0.50, true); decision != "approved" {
		t.Fatalf("expected approved, got %s", decision)
	}
	if decision := engine.VerificationDecision(0.60, 0.50, true); decision != "review" {
		t.Fatalf("expected review, got %s", decision)
	}
	if decision := engine.VerificationDecision(0.80, 0.20, true); decision != "rejected" {
		t.Fatalf("expected rejected, got %s", decision)
	}
}

func TestDecisionRejectsWhenPassivePADIsRequiredAndUnavailable(t *testing.T) {
	engine := NewEngine(0.68, 0.55, 0.363, true)

	if decision := engine.EnrollmentDecision(0.95, false); decision != "rejected" {
		t.Fatalf("expected rejected enrollment, got %s", decision)
	}
	if decision := engine.VerificationDecision(0.95, 0.90, false); decision != "rejected" {
		t.Fatalf("expected rejected verification, got %s", decision)
	}
}
