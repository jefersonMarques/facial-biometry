package decoder

import "time"

type Header struct {
	CreatedAt  time.Time
	Timestamp  uint32
	Version    uint8
	Body       []byte
	HeaderSize int
	Format     string
}

type Payload struct {
	TemplateID    uint16
	Signature     []byte
	Image         []byte
	EncodedFields []byte
	FieldsText    string
	Extra         []byte
	SignedData    []byte
	ImageIsBPG    bool
}

type FieldValue struct {
	Name  string
	Label string
	Value string
}

type RawFieldValue struct {
	Index  int
	Name   string
	Label  string
	Value  string
	Mapped bool
}

type TechnicalData struct {
	HeaderFormat          string
	Timestamp             uint32
	TemplateKnown         bool
	TemplateFieldCount    int
	DecodedFieldCount     int
	UnmappedFieldCount    int
	CertificateGroupID    string
	CertificateCandidates int
	QRPayloadSize         int
	QRPayloadSHA256       string
	HeaderSize            int
	BodySize              int
	SignedDataSize        int
	SignedDataSHA256      string
	SignatureSize         int
	SignatureSHA256       string
	EncodedFieldsSize     int
	EmbeddedImageSize     int
	EmbeddedImageSHA256   string
	ImageFileSize         int
	ImageSHA256           string
	ExtraSize             int
	FieldCount            int
	ImageWidth            int
	ImageHeight           int
	ImageBitDepth         int
	ImagePixelFormat      string
	ImageColorSpace       string
	ImageHasAlpha         bool
	ImageAnimation        bool
	PreviewWidth          int
	PreviewHeight         int
}

type Result struct {
	InputEncoding      string
	Version            uint8
	TemplateID         uint16
	TemplateName       string
	OwnerName          string
	CreatedAt          time.Time
	SignatureValid     bool
	SignatureAlgorithm string
	CertificateID      string
	Fields             []FieldValue
	RawFields          []RawFieldValue
	DecodedFieldsText  string
	RawQRPayload       []byte
	HeaderBytes        []byte
	Body               []byte
	Signature          []byte
	EncodedFields      []byte
	SignedData         []byte
	Image              []byte
	EmbeddedImage      []byte
	ImageFormat        string
	PreviewImage       []byte
	PreviewMIME        string
	PreviewError       string
	Extra              []byte
	Technical          TechnicalData
}
