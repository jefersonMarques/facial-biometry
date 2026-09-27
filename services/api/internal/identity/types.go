package identity

import "time"

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
	VIOTemplateID          uint16    `json:"vioTemplateId"`
	VIOCreatedAt           time.Time `json:"vioCreatedAt"`
	VIOSignatureAlgorithm  string    `json:"vioSignatureAlgorithm"`
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
	CaptureSessionID       string            `json:"captureSessionId,omitempty"`
	ReferenceEmbedding     []float64         `json:"referenceEmbedding,omitempty"`
	ReferenceEmbeddingModel string           `json:"referenceEmbeddingModel,omitempty"`
	Document               *DocumentEvidence `json:"document,omitempty"`
	Decision               string            `json:"decision,omitempty"`
	LivenessScore          float64           `json:"livenessScore,omitempty"`
	FaceSimilarity         float64           `json:"faceSimilarity,omitempty"`
	CompletedAt            *time.Time        `json:"completedAt,omitempty"`
	LastErrorCode          string            `json:"lastErrorCode,omitempty"`
}
