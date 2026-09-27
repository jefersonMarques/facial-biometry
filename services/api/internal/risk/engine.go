package risk

type Engine struct {
	livenessThreshold       float64
	reviewLivenessThreshold float64
	matchThreshold          float64
	requirePassivePAD       bool
}

func NewEngine(livenessThreshold float64, reviewLivenessThreshold float64, matchThreshold float64, requirePassivePAD bool) *Engine {
	return &Engine{
		livenessThreshold:       livenessThreshold,
		reviewLivenessThreshold: reviewLivenessThreshold,
		matchThreshold:          matchThreshold,
		requirePassivePAD:       requirePassivePAD,
	}
}

func (engine *Engine) EnrollmentDecision(livenessScore float64, passivePADAvailable bool) string {
	return engine.livenessDecision(livenessScore, passivePADAvailable)
}

func (engine *Engine) VerificationDecision(livenessScore float64, similarity float64, passivePADAvailable bool) string {
	livenessDecision := engine.livenessDecision(livenessScore, passivePADAvailable)
	if livenessDecision == "rejected" {
		return "rejected"
	}
	if similarity < engine.matchThreshold*0.9 {
		return "rejected"
	}
	if livenessDecision == "review" || similarity < engine.matchThreshold {
		return "review"
	}
	return "approved"
}

func (engine *Engine) livenessDecision(score float64, passivePADAvailable bool) string {
	if engine.requirePassivePAD && !passivePADAvailable {
		return "rejected"
	}
	if score >= engine.livenessThreshold {
		return "approved"
	}
	if score >= engine.reviewLivenessThreshold {
		return "review"
	}
	return "rejected"
}
