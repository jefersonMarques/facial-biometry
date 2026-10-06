package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var ErrNotFound = errors.New("analytics record not found")

type Repository struct {
	db         *sql.DB
	subjectKey []byte
}

type Campaign struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type CheckCreated struct {
	CheckID          string
	SubjectCPF       string
	CampaignID       string
	Scenario         string
	ExpectedDecision string
	Status           string
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

type DocumentSnapshot struct {
	PDFSHA256                string    `json:"pdfSha256,omitempty"`
	PDFSigningTime           time.Time `json:"pdfSigningTime,omitempty"`
	PDFSourceIntegrity       string    `json:"pdfSourceIntegrity,omitempty"`
	PDFSignatureAlgorithm    string    `json:"pdfSignatureAlgorithm,omitempty"`
	VIOTemplateID            uint16    `json:"vioTemplateId,omitempty"`
	VIOCreatedAt             time.Time `json:"vioCreatedAt,omitempty"`
	VIOSignatureAlgorithm    string    `json:"vioSignatureAlgorithm,omitempty"`
	ReferencePhotoSource     string    `json:"referencePhotoSource,omitempty"`
	ReferencePhotoMethod     string    `json:"referencePhotoMethod,omitempty"`
	ReferencePhotoConfidence string    `json:"referencePhotoConfidence,omitempty"`
	ReferencePhotoSHA256     string    `json:"referencePhotoSha256,omitempty"`
	ReferencePhotoWidth      int       `json:"referencePhotoWidth,omitempty"`
	ReferencePhotoHeight     int       `json:"referencePhotoHeight,omitempty"`
}

type GeometrySnapshot struct {
	RunID             string  `json:"runId,omitempty"`
	Status            string  `json:"status,omitempty"`
	SampleCount       int     `json:"sampleCount"`
	FarSamples        int     `json:"farSamples"`
	NearSamples       int     `json:"nearSamples"`
	ScaleRatio        float64 `json:"scaleRatio"`
	TransitionScore   float64 `json:"transitionScore"`
	PerspectiveChange float64 `json:"perspectiveChange"`
	DepthChange       float64 `json:"depthChange"`
	PhaseStability    float64 `json:"phaseStability"`
	EvidenceScore     float64 `json:"evidenceScore"`
	WASMStatus        string  `json:"wasmStatus,omitempty"`
	WASMMaxDelta      float64 `json:"wasmMaxDelta"`
}

type NativeShadowSnapshot struct {
	Status                     string  `json:"status,omitempty"`
	PythonFrames               int     `json:"pythonFrames"`
	NativeFrames               int     `json:"nativeFrames"`
	BBoxMaxDeltaPx             float64 `json:"bboxMaxDeltaPx"`
	ConfidenceMaxDelta         float64 `json:"confidenceMaxDelta"`
	QualityMaxDelta            float64 `json:"qualityMaxDelta"`
	PadComparedFrames          int     `json:"padComparedFrames"`
	PassivePadMaxDelta         float64 `json:"passivePadMaxDelta"`
	SelectedEmbeddingMinCosine float64 `json:"selectedEmbeddingMinCosine"`
	SelectedEmbeddingMaxDelta  float64 `json:"selectedEmbeddingMaxDelta"`
	CombinedEmbeddingCosine    float64 `json:"combinedEmbeddingCosine"`
	CombinedEmbeddingMaxDelta  float64 `json:"combinedEmbeddingMaxDelta"`
}

type CompletionSnapshot struct {
	CheckID                 string
	Status                  string
	Decision                string
	CompletedAt             time.Time
	DocumentAttempts        int
	BiometricSessions       int
	LivenessScore           float64
	FaceSimilarity          float64
	MatchThreshold          float64
	LivenessThreshold       float64
	ReviewLivenessThreshold float64
	RequirePassivePAD       bool
	FrameSimilarities       []float64
	PassivePADScore         float64
	TemporalMotionScore     float64
	IlluminationScore       float64
	GuidedCaptureScore      float64
	QualityScore            float64
	EmbeddingModel          string
	Geometry                *GeometrySnapshot
	NativeShadow            *NativeShadowSnapshot
	Runtime                 any
	CaptureProtocol         any
	Diagnostics             []string
}

type DashboardSummary struct {
	Total              int64             `json:"total"`
	Approved           int64             `json:"approved"`
	Review             int64             `json:"review"`
	Rejected           int64             `json:"rejected"`
	Expired            int64             `json:"expired"`
	Pending            int64             `json:"pending"`
	Completed          int64             `json:"completed"`
	CompletionRate     float64           `json:"completionRate"`
	AvgFaceSimilarity  float64           `json:"avgFaceSimilarity"`
	AvgLiveness        float64           `json:"avgLiveness"`
	AvgPassivePAD      float64           `json:"avgPassivePad"`
	AvgQuality         float64           `json:"avgQuality"`
	NativeOK           int64             `json:"nativeOk"`
	NativePartial      int64             `json:"nativePartial"`
	NativeDrift        int64             `json:"nativeDrift"`
	Scenarios          []ScenarioSummary `json:"scenarios"`
}

type ScenarioSummary struct {
	Scenario          string `json:"scenario"`
	Total             int64  `json:"total"`
	Approved          int64  `json:"approved"`
	Review            int64  `json:"review"`
	Rejected          int64  `json:"rejected"`
	ExpectedEvaluated int64  `json:"expectedEvaluated"`
	ExpectedCorrect   int64  `json:"expectedCorrect"`
}

type SummaryCheckRow struct {
	CheckID          string
	Scenario         string
	ExpectedDecision string
	Status           string
	Decision         string
	CompletedAt      *time.Time
	FaceSimilarity   *float64
	LivenessScore    *float64
}

type CheckListItem struct {
	CheckID          string     `json:"checkId"`
	CampaignID       string     `json:"campaignId,omitempty"`
	CampaignName     string     `json:"campaignName,omitempty"`
	Scenario         string     `json:"scenario"`
	ExpectedDecision string     `json:"expectedDecision,omitempty"`
	Status           string     `json:"status"`
	Decision         string     `json:"decision,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
	FaceSimilarity   *float64   `json:"faceSimilarity,omitempty"`
	LivenessScore    *float64   `json:"livenessScore,omitempty"`
	PassivePADScore  *float64   `json:"passivePadScore,omitempty"`
	QualityScore     *float64   `json:"qualityScore,omitempty"`
	NativeStatus     string     `json:"nativeStatus,omitempty"`
}

type CheckDetail struct {
	CheckListItem
	SubjectHash                    string          `json:"subjectHash"`
	ExpiresAt                      time.Time       `json:"expiresAt"`
	StartedAt                      *time.Time      `json:"startedAt,omitempty"`
	DurationMS                     *int64          `json:"durationMs,omitempty"`
	DocumentAttempts               int             `json:"documentAttempts"`
	BiometricSessions              int             `json:"biometricSessions"`
	MatchThreshold                 *float64        `json:"matchThreshold,omitempty"`
	Margin                         *float64        `json:"margin,omitempty"`
	LivenessThreshold              *float64        `json:"livenessThreshold,omitempty"`
	ReviewLivenessThreshold        *float64        `json:"reviewLivenessThreshold,omitempty"`
	RequirePassivePAD              *bool           `json:"requirePassivePad,omitempty"`
	TemporalMotionScore            *float64        `json:"temporalMotionScore,omitempty"`
	IlluminationScore              *float64        `json:"illuminationScore,omitempty"`
	GuidedCaptureScore             *float64        `json:"guidedCaptureScore,omitempty"`
	EmbeddingModel                 string          `json:"embeddingModel,omitempty"`
	FrameSimilarities              json.RawMessage `json:"frameSimilarities,omitempty"`
	Geometry                       json.RawMessage `json:"geometry,omitempty"`
	NativeShadow                   json.RawMessage `json:"nativeShadow,omitempty"`
	Runtime                        json.RawMessage `json:"runtime,omitempty"`
	CaptureProtocol                json.RawMessage `json:"captureProtocol,omitempty"`
	Document                       json.RawMessage `json:"document,omitempty"`
	Diagnostics                    json.RawMessage `json:"diagnostics,omitempty"`
	Events                         []Event          `json:"events"`
}

type Event struct {
	ID         int64           `json:"id"`
	EventType  string          `json:"eventType"`
	OccurredAt time.Time       `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type CheckFilter struct {
	Status     string
	Scenario   string
	CampaignID string
	Limit      int
}

type OpenCheckState struct {
	CheckID string
	Status  string
}


func Open(ctx context.Context, databaseURL string, subjectKey []byte) (*Repository, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, nil
	}
	if len(subjectKey) < 32 {
		return nil, errors.New("analytics subject key must contain at least 32 bytes")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect analytics database: %w", err)
	}

	repository := &Repository{
		db:         db,
		subjectKey: append([]byte(nil), subjectKey...),
	}
	if err := repository.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return repository, nil
}

func (repository *Repository) Close() error {
	if repository == nil || repository.db == nil {
		return nil
	}
	return repository.db.Close()
}

func (repository *Repository) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS faceproof_campaigns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS faceproof_checks (
    check_id TEXT PRIMARY KEY,
    subject_hash TEXT NOT NULL,
    campaign_id TEXT REFERENCES faceproof_campaigns(id) ON DELETE SET NULL,
    scenario TEXT NOT NULL DEFAULT 'unknown',
    expected_decision TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    actual_decision TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration_ms BIGINT,
    document_attempts INTEGER NOT NULL DEFAULT 0,
    biometric_sessions INTEGER NOT NULL DEFAULT 0,
    face_similarity DOUBLE PRECISION,
    liveness_score DOUBLE PRECISION,
    match_threshold DOUBLE PRECISION,
    margin DOUBLE PRECISION,
    liveness_threshold DOUBLE PRECISION,
    review_liveness_threshold DOUBLE PRECISION,
    require_passive_pad BOOLEAN,
    passive_pad_score DOUBLE PRECISION,
    temporal_motion_score DOUBLE PRECISION,
    illumination_score DOUBLE PRECISION,
    guided_capture_score DOUBLE PRECISION,
    quality_score DOUBLE PRECISION,
    embedding_model TEXT NOT NULL DEFAULT '',
    frame_similarities JSONB,
    geometry JSONB,
    native_shadow JSONB,
    runtime JSONB,
    capture_protocol JSONB,
    document JSONB,
    diagnostics JSONB,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS faceproof_checks_created_at_idx
    ON faceproof_checks(created_at DESC);
CREATE INDEX IF NOT EXISTS faceproof_checks_scenario_idx
    ON faceproof_checks(scenario);
CREATE INDEX IF NOT EXISTS faceproof_checks_status_idx
    ON faceproof_checks(status);
CREATE INDEX IF NOT EXISTS faceproof_checks_campaign_idx
    ON faceproof_checks(campaign_id);

CREATE TABLE IF NOT EXISTS faceproof_events (
    id BIGSERIAL PRIMARY KEY,
    check_id TEXT NOT NULL REFERENCES faceproof_checks(check_id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload JSONB,
    dedupe_key TEXT
);

CREATE INDEX IF NOT EXISTS faceproof_events_check_idx
    ON faceproof_events(check_id, occurred_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS faceproof_events_dedupe_idx
    ON faceproof_events(check_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL;
`
	if _, err := repository.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate analytics database: %w", err)
	}
	return nil
}

func (repository *Repository) CreateCampaign(ctx context.Context, campaign Campaign) error {
	_, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO faceproof_campaigns (id, name, description, created_at)
		 VALUES ($1, $2, $3, $4)`,
		campaign.ID,
		strings.TrimSpace(campaign.Name),
		strings.TrimSpace(campaign.Description),
		campaign.CreatedAt,
	)
	return err
}

func (repository *Repository) CampaignExists(ctx context.Context, id string) (bool, error) {
	if strings.TrimSpace(id) == "" {
		return true, nil
	}
	var exists bool
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM faceproof_campaigns WHERE id = $1)`,
		id,
	).Scan(&exists)
	return exists, err
}

func (repository *Repository) ListCampaigns(ctx context.Context) ([]Campaign, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, name, description, created_at
		 FROM faceproof_campaigns
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []Campaign
	for rows.Next() {
		var campaign Campaign
		if err := rows.Scan(
			&campaign.ID,
			&campaign.Name,
			&campaign.Description,
			&campaign.CreatedAt,
		); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, campaign)
	}
	return campaigns, rows.Err()
}

func (repository *Repository) RecordCheckCreated(ctx context.Context, record CheckCreated) error {
	_, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO faceproof_checks (
			check_id, subject_hash, campaign_id, scenario, expected_decision,
			status, created_at, expires_at
		) VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8)
		ON CONFLICT (check_id) DO NOTHING`,
		record.CheckID,
		repository.subjectHash(record.SubjectCPF),
		record.CampaignID,
		record.Scenario,
		record.ExpectedDecision,
		record.Status,
		record.CreatedAt,
		record.ExpiresAt,
	)
	return err
}

func (repository *Repository) RecordEvent(
	ctx context.Context,
	checkID string,
	eventType string,
	payload any,
) error {
	return repository.recordEvent(ctx, checkID, eventType, "", payload)
}

func (repository *Repository) RecordEventOnce(
	ctx context.Context,
	checkID string,
	eventType string,
	dedupeKey string,
	payload any,
) error {
	return repository.recordEvent(ctx, checkID, eventType, dedupeKey, payload)
}

func (repository *Repository) recordEvent(
	ctx context.Context,
	checkID string,
	eventType string,
	dedupeKey string,
	payload any,
) error {
	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return err
	}
	_, err = repository.db.ExecContext(
		ctx,
		`INSERT INTO faceproof_events (check_id, event_type, payload, dedupe_key)
		 VALUES ($1, $2, $3, NULLIF($4, ''))
		 ON CONFLICT (check_id, dedupe_key)
		 WHERE dedupe_key IS NOT NULL
		 DO NOTHING`,
		checkID,
		eventType,
		jsonParameter(payloadJSON),
		dedupeKey,
	)
	return err
}

func (repository *Repository) MarkStarted(ctx context.Context, checkID string) error {
	_, err := repository.db.ExecContext(
		ctx,
		`UPDATE faceproof_checks
		 SET started_at = COALESCE(started_at, NOW()), updated_at = NOW()
		 WHERE check_id = $1`,
		checkID,
	)
	return err
}

func (repository *Repository) RecordDocumentAccepted(
	ctx context.Context,
	checkID string,
	status string,
	attempts int,
	document DocumentSnapshot,
) error {
	documentJSON, err := marshalJSON(document)
	if err != nil {
		return err
	}
	_, err = repository.db.ExecContext(
		ctx,
		`UPDATE faceproof_checks
		 SET status = $2,
		     document_attempts = $3,
		     document = $4,
		     updated_at = NOW()
		 WHERE check_id = $1`,
		checkID,
		status,
		attempts,
		jsonParameter(documentJSON),
	)
	return err
}

func (repository *Repository) RecordDocumentFailure(
	ctx context.Context,
	checkID string,
	status string,
	attempts int,
	errorCode string,
) error {
	_, err := repository.db.ExecContext(
		ctx,
		`UPDATE faceproof_checks
		 SET status = $2,
		     document_attempts = $3,
		     updated_at = NOW()
		 WHERE check_id = $1`,
		checkID,
		status,
		attempts,
	)
	if err != nil {
		return err
	}
	return repository.RecordEvent(ctx, checkID, "document_rejected", map[string]any{
		"attempt":   attempts,
		"errorCode": errorCode,
	})
}

func (repository *Repository) RecordBiometrySession(
	ctx context.Context,
	checkID string,
	sessionID string,
) error {
	if err := repository.RecordEvent(ctx, checkID, "biometric_session_issued", map[string]any{
		"sessionId": sessionID,
	}); err != nil {
		return err
	}
	_, err := repository.db.ExecContext(
		ctx,
		`UPDATE faceproof_checks
		 SET status = 'biometry_pending',
		     biometric_sessions = biometric_sessions + 1,
		     updated_at = NOW()
		 WHERE check_id = $1`,
		checkID,
	)
	return err
}

func (repository *Repository) RecordStatus(
	ctx context.Context,
	checkID string,
	status string,
	decision string,
	completedAt *time.Time,
) error {
	_, err := repository.db.ExecContext(
		ctx,
		`UPDATE faceproof_checks
		 SET status = $2,
		     actual_decision = CASE WHEN $3 = '' THEN actual_decision ELSE $3 END,
		     completed_at = COALESCE($4::timestamptz, completed_at),
		     duration_ms = CASE
		         WHEN $4::timestamptz IS NULL OR started_at IS NULL THEN duration_ms
		         ELSE GREATEST(0, (EXTRACT(EPOCH FROM ($4::timestamptz - started_at)) * 1000)::BIGINT)
		     END,
		     updated_at = NOW()
		 WHERE check_id = $1`,
		checkID,
		status,
		decision,
		completedAt,
	)
	return err
}

func (repository *Repository) RecordCompletion(
	ctx context.Context,
	record CompletionSnapshot,
) error {
	frameSimilarities, err := marshalJSON(record.FrameSimilarities)
	if err != nil {
		return err
	}
	geometry, err := marshalNullableJSON(record.Geometry)
	if err != nil {
		return err
	}
	nativeShadow, err := marshalNullableJSON(record.NativeShadow)
	if err != nil {
		return err
	}
	runtimeJSON, err := marshalNullableJSON(record.Runtime)
	if err != nil {
		return err
	}
	captureProtocolJSON, err := marshalNullableJSON(record.CaptureProtocol)
	if err != nil {
		return err
	}
	diagnostics, err := marshalJSON(record.Diagnostics)
	if err != nil {
		return err
	}

	_, err = repository.db.ExecContext(
		ctx,
		`UPDATE faceproof_checks
		 SET status = $2,
		     actual_decision = $3,
		     completed_at = $4,
		     duration_ms = CASE
		         WHEN started_at IS NULL THEN NULL
		         ELSE GREATEST(0, (EXTRACT(EPOCH FROM ($4 - started_at)) * 1000)::BIGINT)
		     END,
		     document_attempts = $5,
		     biometric_sessions = $6,
		     liveness_score = $7,
		     face_similarity = $8,
		     match_threshold = $9,
		     margin = $8 - $9,
		     liveness_threshold = $10,
		     review_liveness_threshold = $11,
		     require_passive_pad = $12,
		     frame_similarities = $13,
		     passive_pad_score = $14,
		     temporal_motion_score = $15,
		     illumination_score = $16,
		     guided_capture_score = $17,
		     quality_score = $18,
		     embedding_model = $19,
		     geometry = $20,
		     native_shadow = $21,
		     runtime = $22,
		     capture_protocol = $23,
		     diagnostics = $24,
		     updated_at = NOW()
		 WHERE check_id = $1`,
		record.CheckID,
		record.Status,
		record.Decision,
		record.CompletedAt,
		record.DocumentAttempts,
		record.BiometricSessions,
		record.LivenessScore,
		record.FaceSimilarity,
		record.MatchThreshold,
		record.LivenessThreshold,
		record.ReviewLivenessThreshold,
		record.RequirePassivePAD,
		jsonParameter(frameSimilarities),
		record.PassivePADScore,
		record.TemporalMotionScore,
		record.IlluminationScore,
		record.GuidedCaptureScore,
		record.QualityScore,
		record.EmbeddingModel,
		jsonParameter(geometry),
		jsonParameter(nativeShadow),
		jsonParameter(runtimeJSON),
		jsonParameter(captureProtocolJSON),
		jsonParameter(diagnostics),
	)
	return err
}

func (repository *Repository) Summary(ctx context.Context) (DashboardSummary, error) {
	var summary DashboardSummary
	var avgFace, avgLiveness, avgPAD, avgQuality sql.NullFloat64

	err := repository.db.QueryRowContext(
		ctx,
		`SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE status = 'approved'),
		    COUNT(*) FILTER (WHERE status = 'review'),
		    COUNT(*) FILTER (WHERE status = 'rejected'),
		    COUNT(*) FILTER (WHERE status = 'expired'),
		    COUNT(*) FILTER (WHERE status NOT IN ('approved','review','rejected','expired')),
		    COUNT(*) FILTER (WHERE completed_at IS NOT NULL),
		    AVG(face_similarity) FILTER (WHERE face_similarity IS NOT NULL),
		    AVG(liveness_score) FILTER (WHERE liveness_score IS NOT NULL),
		    AVG(passive_pad_score) FILTER (WHERE passive_pad_score IS NOT NULL),
		    AVG(quality_score) FILTER (WHERE quality_score IS NOT NULL),
		    COUNT(*) FILTER (WHERE native_shadow->>'status' IN ('ok', 'authority')),
		    COUNT(*) FILTER (WHERE native_shadow->>'status' = 'partial'),
		    COUNT(*) FILTER (WHERE native_shadow->>'status' = 'drift')
		 FROM faceproof_checks`,
	).Scan(
		&summary.Total,
		&summary.Approved,
		&summary.Review,
		&summary.Rejected,
		&summary.Expired,
		&summary.Pending,
		&summary.Completed,
		&avgFace,
		&avgLiveness,
		&avgPAD,
		&avgQuality,
		&summary.NativeOK,
		&summary.NativePartial,
		&summary.NativeDrift,
	)
	if err != nil {
		return DashboardSummary{}, err
	}

	summary.AvgFaceSimilarity = nullFloat(avgFace)
	summary.AvgLiveness = nullFloat(avgLiveness)
	summary.AvgPassivePAD = nullFloat(avgPAD)
	summary.AvgQuality = nullFloat(avgQuality)
	if summary.Total > 0 {
		summary.CompletionRate = float64(summary.Completed) / float64(summary.Total)
	}

	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT
		    scenario,
		    COUNT(*),
		    COUNT(*) FILTER (WHERE actual_decision = 'approved'),
		    COUNT(*) FILTER (WHERE actual_decision = 'review'),
		    COUNT(*) FILTER (WHERE actual_decision = 'rejected'),
		    COUNT(*) FILTER (WHERE expected_decision <> '' AND actual_decision <> ''),
		    COUNT(*) FILTER (
		        WHERE expected_decision <> ''
		          AND actual_decision <> ''
		          AND expected_decision = actual_decision
		    )
		 FROM faceproof_checks
		 GROUP BY scenario
		 ORDER BY COUNT(*) DESC, scenario`,
	)
	if err != nil {
		return DashboardSummary{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var item ScenarioSummary
		if err := rows.Scan(
			&item.Scenario,
			&item.Total,
			&item.Approved,
			&item.Review,
			&item.Rejected,
			&item.ExpectedEvaluated,
			&item.ExpectedCorrect,
		); err != nil {
			return DashboardSummary{}, err
		}
		summary.Scenarios = append(summary.Scenarios, item)
	}
	if err := rows.Err(); err != nil {
		return DashboardSummary{}, err
	}

	return summary, nil
}

func (repository *Repository) ListSummaryCheckRows(
	ctx context.Context,
) ([]SummaryCheckRow, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT
		    check_id,
		    scenario,
		    expected_decision,
		    status,
		    actual_decision,
		    completed_at,
		    face_similarity,
		    liveness_score
		 FROM faceproof_checks
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SummaryCheckRow
	for rows.Next() {
		var item SummaryCheckRow
		var completedAt sql.NullTime
		var face, liveness sql.NullFloat64
		if err := rows.Scan(
			&item.CheckID,
			&item.Scenario,
			&item.ExpectedDecision,
			&item.Status,
			&item.Decision,
			&completedAt,
			&face,
			&liveness,
		); err != nil {
			return nil, err
		}
		item.CompletedAt = nullTimePtr(completedAt)
		item.FaceSimilarity = nullFloatPtr(face)
		item.LivenessScore = nullFloatPtr(liveness)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) ListOpenCheckStates(
	ctx context.Context,
	limit int,
) ([]OpenCheckState, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT check_id, status
		 FROM faceproof_checks
		 WHERE status NOT IN ('approved','review','rejected','expired')
		 ORDER BY created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []OpenCheckState
	for rows.Next() {
		var state OpenCheckState
		if err := rows.Scan(&state.CheckID, &state.Status); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

func (repository *Repository) BackfillEngineMetrics(ctx context.Context) error {
	_, err := repository.db.ExecContext(
		ctx,
		`WITH latest_engine AS (
		    SELECT DISTINCT ON (check_id)
		        check_id,
		        NULLIF(payload->>'passivePad', '')::DOUBLE PRECISION AS passive_pad_score,
		        NULLIF(payload->>'quality', '')::DOUBLE PRECISION AS quality_score,
		        NULLIF(payload->>'nativeShadow', '') AS native_status
		    FROM faceproof_events
		    WHERE event_type = 'engine_completed'
		    ORDER BY check_id, occurred_at DESC, id DESC
		)
		UPDATE faceproof_checks c
		SET passive_pad_score = COALESCE(c.passive_pad_score, e.passive_pad_score),
		    quality_score = COALESCE(c.quality_score, e.quality_score),
		    native_shadow = CASE
		        WHEN c.native_shadow IS NULL AND e.native_status IS NOT NULL
		            THEN jsonb_build_object('status', e.native_status)
		        ELSE c.native_shadow
		    END,
		    updated_at = NOW()
		FROM latest_engine e
		WHERE c.check_id = e.check_id
		  AND (
		      c.passive_pad_score IS NULL
		      OR c.quality_score IS NULL
		      OR (c.native_shadow IS NULL AND e.native_status IS NOT NULL)
		  )`,
	)
	return err
}

func (repository *Repository) UpdateCheckClassification(
	ctx context.Context,
	checkID string,
	campaignID string,
	scenario string,
	expectedDecision string,
) error {
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE faceproof_checks
		 SET campaign_id = NULLIF($2, ''),
		     scenario = $3,
		     expected_decision = $4,
		     updated_at = NOW()
		 WHERE check_id = $1`,
		checkID,
		strings.TrimSpace(campaignID),
		scenario,
		expectedDecision,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (repository *Repository) RecordAuthoritativeState(
	ctx context.Context,
	checkID string,
	status string,
	decision string,
	completedAt *time.Time,
	livenessScore float64,
	faceSimilarity float64,
	biometricSessions int,
) error {
	_, err := repository.db.ExecContext(
		ctx,
		`UPDATE faceproof_checks
		 SET status = $2,
		     actual_decision = CASE WHEN $3 = '' THEN actual_decision ELSE $3 END,
		     completed_at = COALESCE($4::timestamptz, completed_at),
		     duration_ms = CASE
		         WHEN $4::timestamptz IS NULL OR started_at IS NULL THEN duration_ms
		         ELSE GREATEST(0, (EXTRACT(EPOCH FROM ($4::timestamptz - started_at)) * 1000)::BIGINT)
		     END,
		     liveness_score = CASE WHEN $5 = 0 THEN liveness_score ELSE $5 END,
		     face_similarity = CASE WHEN $6 = 0 THEN face_similarity ELSE $6 END,
		     biometric_sessions = GREATEST(biometric_sessions, $7),
		     updated_at = NOW()
		 WHERE check_id = $1`,
		checkID,
		status,
		decision,
		completedAt,
		livenessScore,
		faceSimilarity,
		biometricSessions,
	)
	return err
}

func (repository *Repository) ListCheckIDs(ctx context.Context) ([]string, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT check_id
		 FROM faceproof_checks
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (repository *Repository) ListChecks(
	ctx context.Context,
	filter CheckFilter,
) ([]CheckListItem, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	where := []string{"1=1"}
	args := []any{}
	add := func(column, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		args = append(args, value)
		where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	add("c.status", filter.Status)
	add("c.scenario", filter.Scenario)
	add("c.campaign_id", filter.CampaignID)
	args = append(args, limit)

	query := fmt.Sprintf(
		`SELECT
		    c.check_id,
		    COALESCE(c.campaign_id, ''),
		    COALESCE(p.name, ''),
		    c.scenario,
		    c.expected_decision,
		    c.status,
		    c.actual_decision,
		    c.created_at,
		    c.completed_at,
		    c.face_similarity,
		    c.liveness_score,
		    c.passive_pad_score,
		    c.quality_score,
		    COALESCE(c.native_shadow->>'status', '')
		 FROM faceproof_checks c
		 LEFT JOIN faceproof_campaigns p ON p.id = c.campaign_id
		 WHERE %s
		 ORDER BY c.created_at DESC
		 LIMIT $%d`,
		strings.Join(where, " AND "),
		len(args),
	)

	rows, err := repository.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CheckListItem
	for rows.Next() {
		var item CheckListItem
		var completedAt sql.NullTime
		var face, liveness, pad, quality sql.NullFloat64
		if err := rows.Scan(
			&item.CheckID,
			&item.CampaignID,
			&item.CampaignName,
			&item.Scenario,
			&item.ExpectedDecision,
			&item.Status,
			&item.Decision,
			&item.CreatedAt,
			&completedAt,
			&face,
			&liveness,
			&pad,
			&quality,
			&item.NativeStatus,
		); err != nil {
			return nil, err
		}
		item.CompletedAt = nullTimePtr(completedAt)
		item.FaceSimilarity = nullFloatPtr(face)
		item.LivenessScore = nullFloatPtr(liveness)
		item.PassivePADScore = nullFloatPtr(pad)
		item.QualityScore = nullFloatPtr(quality)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) GetCheck(ctx context.Context, checkID string) (CheckDetail, error) {
	var detail CheckDetail
	var completedAt, startedAt sql.NullTime
	var duration sql.NullInt64
	var face, liveness, pad, quality sql.NullFloat64
	var matchThreshold, margin, livenessThreshold, reviewThreshold sql.NullFloat64
	var requirePAD sql.NullBool
	var temporalMotion, illumination, guidedCapture sql.NullFloat64
	var frameSimilarities, geometry, nativeShadow, runtimeJSON, captureProtocol, document, diagnostics []byte

	err := repository.db.QueryRowContext(
		ctx,
		`SELECT
		    c.check_id,
		    COALESCE(c.campaign_id, ''),
		    COALESCE(p.name, ''),
		    c.scenario,
		    c.expected_decision,
		    c.status,
		    c.actual_decision,
		    c.created_at,
		    c.completed_at,
		    c.face_similarity,
		    c.liveness_score,
		    c.passive_pad_score,
		    c.quality_score,
		    COALESCE(c.native_shadow->>'status', ''),
		    c.subject_hash,
		    c.expires_at,
		    c.started_at,
		    c.duration_ms,
		    c.document_attempts,
		    c.biometric_sessions,
		    c.match_threshold,
		    c.margin,
		    c.liveness_threshold,
		    c.review_liveness_threshold,
		    c.require_passive_pad,
		    c.temporal_motion_score,
		    c.illumination_score,
		    c.guided_capture_score,
		    c.embedding_model,
		    c.frame_similarities,
		    c.geometry,
		    c.native_shadow,
		    c.runtime,
		    c.capture_protocol,
		    c.document,
		    c.diagnostics
		 FROM faceproof_checks c
		 LEFT JOIN faceproof_campaigns p ON p.id = c.campaign_id
		 WHERE c.check_id = $1`,
		checkID,
	).Scan(
		&detail.CheckID,
		&detail.CampaignID,
		&detail.CampaignName,
		&detail.Scenario,
		&detail.ExpectedDecision,
		&detail.Status,
		&detail.Decision,
		&detail.CreatedAt,
		&completedAt,
		&face,
		&liveness,
		&pad,
		&quality,
		&detail.NativeStatus,
		&detail.SubjectHash,
		&detail.ExpiresAt,
		&startedAt,
		&duration,
		&detail.DocumentAttempts,
		&detail.BiometricSessions,
		&matchThreshold,
		&margin,
		&livenessThreshold,
		&reviewThreshold,
		&requirePAD,
		&temporalMotion,
		&illumination,
		&guidedCapture,
		&detail.EmbeddingModel,
		&frameSimilarities,
		&geometry,
		&nativeShadow,
		&runtimeJSON,
		&captureProtocol,
		&document,
		&diagnostics,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CheckDetail{}, ErrNotFound
	}
	if err != nil {
		return CheckDetail{}, err
	}

	detail.CompletedAt = nullTimePtr(completedAt)
	detail.StartedAt = nullTimePtr(startedAt)
	detail.DurationMS = nullInt64Ptr(duration)
	detail.FaceSimilarity = nullFloatPtr(face)
	detail.LivenessScore = nullFloatPtr(liveness)
	detail.PassivePADScore = nullFloatPtr(pad)
	detail.QualityScore = nullFloatPtr(quality)
	detail.MatchThreshold = nullFloatPtr(matchThreshold)
	detail.Margin = nullFloatPtr(margin)
	detail.LivenessThreshold = nullFloatPtr(livenessThreshold)
	detail.ReviewLivenessThreshold = nullFloatPtr(reviewThreshold)
	detail.RequirePassivePAD = nullBoolPtr(requirePAD)
	detail.TemporalMotionScore = nullFloatPtr(temporalMotion)
	detail.IlluminationScore = nullFloatPtr(illumination)
	detail.GuidedCaptureScore = nullFloatPtr(guidedCapture)
	detail.FrameSimilarities = rawJSON(frameSimilarities)
	detail.Geometry = rawJSON(geometry)
	detail.NativeShadow = rawJSON(nativeShadow)
	detail.Runtime = rawJSON(runtimeJSON)
	detail.CaptureProtocol = rawJSON(captureProtocol)
	detail.Document = rawJSON(document)
	detail.Diagnostics = rawJSON(diagnostics)

	events, err := repository.listEvents(ctx, checkID)
	if err != nil {
		return CheckDetail{}, err
	}
	detail.Events = events
	return detail, nil
}

func (repository *Repository) listEvents(ctx context.Context, checkID string) ([]Event, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, event_type, occurred_at, payload
		 FROM faceproof_events
		 WHERE check_id = $1
		 ORDER BY occurred_at, id`,
		checkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var item Event
		var payload []byte
		if err := rows.Scan(&item.ID, &item.EventType, &item.OccurredAt, &payload); err != nil {
			return nil, err
		}
		item.Payload = rawJSON(payload)
		events = append(events, item)
	}
	return events, rows.Err()
}

func (repository *Repository) subjectHash(cpf string) string {
	mac := hmac.New(sha256.New, repository.subjectKey)
	_, _ = mac.Write([]byte(strings.TrimSpace(cpf)))
	return hex.EncodeToString(mac.Sum(nil))
}

func marshalJSON(value any) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(value)
}

func marshalNullableJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func jsonParameter(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func rawJSON(value []byte) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}

func nullFloat(value sql.NullFloat64) float64 {
	if !value.Valid {
		return 0
	}
	return value.Float64
}

func nullFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullBoolPtr(value sql.NullBool) *bool {
	if !value.Valid {
		return nil
	}
	result := value.Bool
	return &result
}
