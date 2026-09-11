package risk

import "testing"

func TestVerificationDecision(t *testing.T) {
	engine := NewEngine(0.68, 0.55, 0.363)

	if decision := engine.VerificationDecision(0.80, 0.50); decision != "approved" {
		t.Fatalf("expected approved, got %s", decision)
	}
	if decision := engine.VerificationDecision(0.60, 0.50); decision != "review" {
		t.Fatalf("expected review, got %s", decision)
	}
	if decision := engine.VerificationDecision(0.80, 0.20); decision != "rejected" {
		t.Fatalf("expected rejected, got %s", decision)
	}
}
