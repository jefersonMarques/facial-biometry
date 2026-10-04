package identity

import (
	"time"

	"faceproof/services/api/internal/domain"
)

type Status string

const (
	StatusPendingDocument    Status = "pending_document"
	StatusProcessingDocument Status = "processing_document"
	StatusBiometryPending    Status = "biometry_pending"
	StatusApproved           Status = "approved"
	StatusReview             Status = "review"
	StatusRejected           Status = "rejected"
	StatusExpired            Status = "expired"
)

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
	VIOTemplateID          uint16    `json:"vioTemplateId"`
	VIOCreatedAt           time.Time `json:"vioCreatedAt"`
	VIOSignatureAlgorithm  string    `json:"vioSignatureAlgorithm"`
	Name                   string    `json:"name,omitempty"`
	BirthDate              string    `json:"birthDate,omitempty"`
	Category               string    `json:"category,omitempty"`
	ExpiryDate             string    `json:"expiryDate,omitempty"`
	IssuingUF              string    `json:"issuingUf,omitempty"`
}

type Check struct {
	ID                     string            `json:"id"`
	ExpectedCPF            string            `json:"expectedCpf"`
	MinimumDocumentDate    time.Time         `json:"minimumDocumentDate"`
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
	Document               *DocumentEvidence `json:"document,omitempty"`
	Decision               string            `json:"decision,omitempty"`
	LivenessScore          float64           `json:"livenessScore,omitempty"`
	FaceSimilarity         float64           `json:"faceSimilarity,omitempty"`
	CompletedAt            *time.Time        `json:"completedAt,omitempty"`
	LastErrorCode          string                     `json:"lastErrorCode,omitempty"`
	RuntimeFingerprint     *domain.RuntimeFingerprint      `json:"runtimeFingerprint,omitempty"`
	CaptureProtocol        *domain.CaptureProtocolMetadata `json:"captureProtocol,omitempty"`
}
