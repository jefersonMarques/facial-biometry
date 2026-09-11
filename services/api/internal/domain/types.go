package domain

import "time"

type SessionKind string

const (
	SessionKindEnrollment   SessionKind = "enrollment"
	SessionKindVerification SessionKind = "verification"
)

type CaptureSession struct {
	ID                  string
	SubjectID           string
	Kind                SessionKind
	IlluminationPattern []float64
	CreatedAt           time.Time
	ExpiresAt           time.Time
	Completed           bool
}

type CapturedFrame struct {
	ImageBase64    string  `json:"imageBase64"`
	TimestampMS    int64   `json:"timestampMs"`
	ChallengeIndex int     `json:"challengeIndex"`
	ClientLight    float64 `json:"clientLight"`
}

type EngineRequest struct {
	Frames              []CapturedFrame `json:"frames"`
	IlluminationPattern []float64       `json:"illuminationPattern"`
}

type EngineSignal struct {
	Score  float64 `json:"score"`
	Status string  `json:"status"`
}

type EngineQuality struct {
	Score           float64 `json:"score"`
	FacePresence    float64 `json:"facePresence"`
	Sharpness       float64 `json:"sharpness"`
	Brightness      float64 `json:"brightness"`
	FaceSize        float64 `json:"faceSize"`
	DetectedFrames  int     `json:"detectedFrames"`
	ProcessedFrames int     `json:"processedFrames"`
}

type EngineResult struct {
	LivenessScore  float64       `json:"livenessScore"`
	PassivePAD     EngineSignal  `json:"passivePad"`
	TemporalMotion EngineSignal  `json:"temporalMotion"`
	Illumination   EngineSignal  `json:"illumination"`
	Quality        EngineQuality `json:"quality"`
	Embedding      []float64     `json:"embedding"`
	EmbeddingModel string        `json:"embeddingModel"`
	BestFrameIndex int           `json:"bestFrameIndex"`
	Diagnostics    []string      `json:"diagnostics"`
}

type BiometricTemplate struct {
	SubjectID      string    `json:"subjectId"`
	Embedding      []float64 `json:"embedding"`
	EmbeddingModel string    `json:"embeddingModel"`
	CreatedAt      time.Time `json:"createdAt"`
}
