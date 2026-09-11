package risk

type Engine struct {
	livenessThreshold       float64
	reviewLivenessThreshold float64
	matchThreshold          float64
}

func NewEngine(livenessThreshold float64, reviewLivenessThreshold float64, matchThreshold float64) *Engine {
	return &Engine{
		livenessThreshold:       livenessThreshold,
		reviewLivenessThreshold: reviewLivenessThreshold,
		matchThreshold:          matchThreshold,
	}
}

func (engine *Engine) EnrollmentDecision(livenessScore float64) string {
	return engine.livenessDecision(livenessScore)
}

func (engine *Engine) VerificationDecision(livenessScore float64, similarity float64) string {
	livenessDecision := engine.livenessDecision(livenessScore)
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

func (engine *Engine) livenessDecision(score float64) string {
	if score >= engine.livenessThreshold {
		return "approved"
	}
	if score >= engine.reviewLivenessThreshold {
		return "review"
	}
	return "rejected"
}
