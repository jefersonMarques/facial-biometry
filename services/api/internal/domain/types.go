package domain

import "time"

type SessionKind string

const (
	SessionKindEnrollment   SessionKind = "enrollment"
	SessionKindVerification SessionKind = "verification"
	SessionKindIdentity     SessionKind = "identity"
)

type CaptureSession struct {
	ID                     string
	SubjectID              string
	Kind                   SessionKind
	CaptureRunID           string
	CaptureProtocolVersion string
	IlluminationPattern  []float64
	CaptureDurationMS    int
	SampleIntervalMS     int
	IlluminationSettleMS int
	CreatedAt            time.Time
	ExpiresAt            time.Time
	Completed            bool
}

type CaptureProtocolMetadata struct {
	Version             string `json:"version"`
	RunID               string `json:"runId"`
	StartedAtUnixMS     int64  `json:"startedAtUnixMs"`
	FarStartedAtUnixMS  int64  `json:"farStartedAtUnixMs"`
	NearStartedAtUnixMS int64  `json:"nearStartedAtUnixMs"`
	SubmittingAtUnixMS  int64  `json:"submittingAtUnixMs"`
}

type RuntimeFingerprint struct {
	SDKVersion                 string `json:"sdkVersion"`
	LivenessCoreVersion        string `json:"livenessCoreVersion"`
	LivenessCoreWASMSHA256    string `json:"livenessCoreWasmSha256"`
	MediaPipeVersion           string `json:"mediaPipeVersion"`
	MediaPipeVisionSHA256      string `json:"mediaPipeVisionSha256"`
	FaceLandmarkerSHA256       string `json:"faceLandmarkerSha256"`
	RuntimeID                  string `json:"runtimeId"`
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

type GuidedCapturedFrame struct {
	ImageBase64   string              `json:"imageBase64,omitempty"`
	ImageBytes    []byte              `json:"-"`
	Phase         string              `json:"phase"`
	ClientQuality *ClientFrameQuality `json:"clientQuality,omitempty"`
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
	Frames              []CapturedFrame       `json:"frames"`
	GuidedFrames        []GuidedCapturedFrame `json:"guidedFrames,omitempty"`
	IlluminationPattern []float64             `json:"illuminationPattern"`
}

type EngineIdentityRequest struct {
	GuidedFrames []GuidedCapturedFrame `json:"guidedFrames"`
}

type EngineGuideResult struct {
	FaceDetected    bool          `json:"faceDetected"`
	Confidence      float64       `json:"confidence"`
	CenterX         float64       `json:"centerX"`
	CenterY         float64       `json:"centerY"`
	WidthRatio      float64       `json:"widthRatio"`
	HeightRatio     float64       `json:"heightRatio"`
	RollDegrees     float64       `json:"rollDegrees"`
	Quality         EngineQuality `json:"quality"`
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

type EngineFaceEmbedding struct {
	FrameIndex int       `json:"frameIndex"`
	Phase      string    `json:"phase"`
	Quality    float64   `json:"quality"`
	Embedding  []float64 `json:"embedding"`
}

type EngineResult struct {
	LivenessScore  float64               `json:"livenessScore"`
	PassivePAD     EngineSignal          `json:"passivePad"`
	TemporalMotion EngineSignal          `json:"temporalMotion"`
	Illumination   EngineSignal          `json:"illumination"`
	GuidedCapture  EngineSignal          `json:"guidedCapture"`
	Quality        EngineQuality         `json:"quality"`
	Embedding      []float64             `json:"embedding"`
	FaceEmbeddings []EngineFaceEmbedding `json:"faceEmbeddings,omitempty"`
	EmbeddingModel string                `json:"embeddingModel"`
	BestFrameIndex int                   `json:"bestFrameIndex"`
	Diagnostics    []string              `json:"diagnostics"`
}

type ReferenceResult struct {
	Embedding      []float64     `json:"embedding"`
	Embeddings     [][]float64   `json:"embeddings,omitempty"`
	EmbeddingModel string        `json:"embeddingModel"`
	Quality        EngineQuality `json:"quality"`
}

type BiometricTemplate struct {
	SubjectID      string    `json:"subjectId"`
	Embedding      []float64 `json:"embedding"`
	EmbeddingModel string    `json:"embeddingModel"`
	CreatedAt      time.Time `json:"createdAt"`
}
