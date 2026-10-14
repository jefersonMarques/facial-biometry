package identity

import (
	"time"

	"faceproof/services/api/internal/domain"
)

type Status string
type FlowType string

const (
	FlowCNH               FlowType = "cnh"
	FlowFaceEnrollment    FlowType = "face_enrollment"
	FlowFaceVerification  FlowType = "face_verification"
	FlowPhotoVerification FlowType = "photo_verification"
)

const (
	StatusPendingDocument    Status = "pending_document"
	StatusProcessingDocument Status = "processing_document"
	StatusBiometryPending    Status = "biometry_pending"
	StatusApproved           Status = "approved"
	StatusReview             Status = "review"
	StatusRejected           Status = "rejected"
	StatusExpired            Status = "expired"
)

type LowResolutionEvidence struct {
	Method          string                `json:"method"`
	Status          string                `json:"status"`
	ErrorCode       string                `json:"errorCode,omitempty"`
	SourceWidth     int                   `json:"sourceWidth"`
	SourceHeight    int                   `json:"sourceHeight"`
	PreparedWidth   int                   `json:"preparedWidth"`
	PreparedHeight  int                   `json:"preparedHeight"`
	ScaleX          float64               `json:"scaleX"`
	ScaleY          float64               `json:"scaleY"`
	Quality         *float64              `json:"quality,omitempty"`
	DurationMS      int64                 `json:"durationMs"`
	PDFVIO          *PortraitPairEvidence `json:"pdfVio,omitempty"`
	VIOLive         *PortraitPairEvidence `json:"vioLive,omitempty"`
}

type LowResolutionReference struct {
	Method          string      `json:"method"`
	Embedding       []float64   `json:"embedding"`
	Embeddings      [][]float64 `json:"embeddings,omitempty"`
	EmbeddingModel  string      `json:"embeddingModel"`
}

type PortraitPairEvidence struct {
	Status            string   `json:"status"`
	CombinedCosine    *float64 `json:"combinedCosine,omitempty"`
	RobustCosine      *float64 `json:"robustCosine,omitempty"`
	ReferenceVariants int      `json:"referenceVariants"`
	TargetVariants    int      `json:"targetVariants"`
	FrameComparisons  int      `json:"frameComparisons"`
	ErrorCode         string   `json:"errorCode,omitempty"`
}

type DualPortraitEvidence struct {
	Status             string   `json:"status"`
	Similarity         *float64 `json:"similarity,omitempty"`
	PDFQuality         *float64 `json:"pdfQuality,omitempty"`
	VIOQuality         *float64 `json:"vioQuality,omitempty"`
	VIOPhotoPresent    bool     `json:"vioPhotoPresent"`
	VIOPhotoValidated  bool     `json:"vioPhotoValidated"`
	VIOPhotoWidth      int      `json:"vioPhotoWidth,omitempty"`
	VIOPhotoHeight     int      `json:"vioPhotoHeight,omitempty"`
	DecodeDurationMS   int64    `json:"decodeDurationMs"`
	AnalysisDurationMS int64    `json:"analysisDurationMs"`
	EmbeddingModel     string   `json:"embeddingModel,omitempty"`
	PDFVIO             *PortraitPairEvidence `json:"pdfVio,omitempty"`
	PDFLive            *PortraitPairEvidence `json:"pdfLive,omitempty"`
	VIOLive            *PortraitPairEvidence `json:"vioLive,omitempty"`
	VIOFaceSize        *float64 `json:"vioFaceSize,omitempty"`
	VIOSharpness       *float64 `json:"vioSharpness,omitempty"`
	VIOBrightness      *float64 `json:"vioBrightness,omitempty"`
	PDFFaceSize        *float64 `json:"pdfFaceSize,omitempty"`
	PDFSharpness       *float64 `json:"pdfSharpness,omitempty"`
	PDFBrightness      *float64 `json:"pdfBrightness,omitempty"`
	ErrorCode          string   `json:"errorCode,omitempty"`
}

type DocumentEvidence struct {
	PDFSHA256             string    `json:"pdfSha256"`
	PDFSigningTime        time.Time `json:"pdfSigningTime"`
	PDFSigner             string    `json:"pdfSigner"`
	PDFCreator            string    `json:"pdfCreator"`
	PDFProducer           string    `json:"pdfProducer"`
	PDFSourceIntegrity    string    `json:"pdfSourceIntegrity"`
	PDFSignatureAlgorithm string    `json:"pdfSignatureAlgorithm"`
	ReferencePhotoSource     string    `json:"referencePhotoSource"`
	ReferencePhotoMethod     string    `json:"referencePhotoMethod,omitempty"`
	ReferencePhotoConfidence string    `json:"referencePhotoConfidence,omitempty"`
	ReferencePhotoSHA256     string    `json:"referencePhotoSha256,omitempty"`
	ReferencePhotoWidth      int       `json:"referencePhotoWidth,omitempty"`
	ReferencePhotoHeight     int       `json:"referencePhotoHeight,omitempty"`
	DualPortrait             *DualPortraitEvidence `json:"dualPortrait,omitempty"`
	LowResolutionBenchmark []LowResolutionEvidence `json:"lowResolutionBenchmark,omitempty"`
	VIOTemplateID          uint16    `json:"vioTemplateId"`
	VIOCreatedAt           time.Time `json:"vioCreatedAt"`
	VIOSignatureAlgorithm  string    `json:"vioSignatureAlgorithm"`
	Name                   string    `json:"name,omitempty"`
	BirthDate              string    `json:"birthDate,omitempty"`
	Category               string    `json:"category,omitempty"`
	ExpiryDate             string    `json:"expiryDate,omitempty"`
	IssuingUF              string    `json:"issuingUf,omitempty"`
}

type DemoArtifacts struct {
	ReferencePhotoDataURL string `json:"referencePhotoDataUrl,omitempty"`
	CapturedPhotoDataURL  string `json:"capturedPhotoDataUrl,omitempty"`
	VIOPhotoDataURL       string `json:"vioPhotoDataUrl,omitempty"`
}

type CompletionReceipt struct {
	SessionID        string    `json:"sessionId"`
	IdempotencyKey   string    `json:"idempotencyKey"`
	RequestSHA256    string    `json:"requestSha256"`
	ResponseJSON     string    `json:"responseJson"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Check struct {
	ID                     string            `json:"id"`
	FlowType               FlowType          `json:"flowType,omitempty"`
	SubjectID              string            `json:"subjectId,omitempty"`
	DisplayName            string            `json:"displayName,omitempty"`
	DemoArtifacts          *DemoArtifacts    `json:"demoArtifacts,omitempty"`
	TenantID               string            `json:"tenantId,omitempty"`
	ExpectedCPF            string            `json:"expectedCpf"`
	MinimumDocumentDate    time.Time         `json:"minimumDocumentDate"`
	CampaignID             string            `json:"campaignId,omitempty"`
	Scenario               string            `json:"scenario,omitempty"`
	ExpectedDecision       string            `json:"expectedDecision,omitempty"`
	Status                 Status            `json:"status"`
	CreatedAt              time.Time         `json:"createdAt"`
	ExpiresAt              time.Time         `json:"expiresAt"`
	DocumentAttempts       int               `json:"documentAttempts"`
	BiometricSessions      int               `json:"biometricSessions"`
	CaptureSessionsIssued  int               `json:"captureSessionsIssued,omitempty"`
	CaptureSessionID       string            `json:"captureSessionId,omitempty"`
	ReferenceEmbedding      []float64         `json:"referenceEmbedding,omitempty"`
	ReferenceEmbeddings     [][]float64       `json:"referenceEmbeddings,omitempty"`
	ReferenceEmbeddingModel string            `json:"referenceEmbeddingModel,omitempty"`
	VIOReferenceEmbedding      []float64      `json:"vioReferenceEmbedding,omitempty"`
	VIOReferenceEmbeddings     [][]float64    `json:"vioReferenceEmbeddings,omitempty"`
	VIOReferenceEmbeddingModel string         `json:"vioReferenceEmbeddingModel,omitempty"`
	LowResolutionReferences []LowResolutionReference `json:"lowResolutionReferences,omitempty"`
	Document               *DocumentEvidence `json:"document,omitempty"`
	Decision               string            `json:"decision,omitempty"`
	LivenessScore          float64           `json:"livenessScore,omitempty"`
	FaceSimilarity         float64           `json:"faceSimilarity,omitempty"`
	CompletedAt            *time.Time        `json:"completedAt,omitempty"`
	LastErrorCode          string                     `json:"lastErrorCode,omitempty"`
	RuntimeFingerprint     *domain.RuntimeFingerprint      `json:"runtimeFingerprint,omitempty"`
	CaptureProtocol        *domain.CaptureProtocolMetadata `json:"captureProtocol,omitempty"`
	CompletionReceipt      *CompletionReceipt               `json:"completionReceipt,omitempty"`
}
