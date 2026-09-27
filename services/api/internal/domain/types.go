package domain

import "time"

type SessionKind string

const (
	SessionKindEnrollment   SessionKind = "enrollment"
	SessionKindVerification SessionKind = "verification"
)

type CaptureSession struct {
	ID                   string
	SubjectID            string
	Kind                 SessionKind
	IlluminationPattern  []float64
	CaptureDurationMS    int
	SampleIntervalMS     int
	IlluminationSettleMS int
	CreatedAt            time.Time
	ExpiresAt            time.Time
	Completed            bool
}

type ClientFrameQuality struct {
	Brightness float64 `json:"brightness"`
	Contrast   float64 `json:"contrast"`
	Sharpness  float64 `json:"sharpness"`
}

type CapturedFrame struct {
	ImageBase64    string              `json:"imageBase64"`
	TimestampMS    int64               `json:"timestampMs"`
	ChallengeIndex int                 `json:"challengeIndex"`
	ClientLight    float64             `json:"clientLight"`
	ClientQuality  *ClientFrameQuality `json:"clientQuality,omitempty"`
}

type CaptureMetadata struct {
	SDKVersion      string `json:"sdkVersion"`
	StartedAtUnixMS int64  `json:"startedAtUnixMs"`
	EndedAtUnixMS   int64  `json:"endedAtUnixMs"`
	FrameWidth      int    `json:"frameWidth"`
	FrameHeight     int    `json:"frameHeight"`
	UserAgent       string `json:"userAgent"`
	Platform        string `json:"platform"`
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
