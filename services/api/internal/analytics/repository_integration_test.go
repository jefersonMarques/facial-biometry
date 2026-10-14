package analytics

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func TestPostgresIdentityCompletionAndReconciliation(t *testing.T) {
	databaseURL := os.Getenv("FACEPROOF_ANALYTICS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration database is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	key := make([]byte, 32)
	for index := range key {
		key[index] = 0x42
	}
	repository, err := Open(ctx, databaseURL, key)
	if err != nil {
		t.Fatalf("open analytics database: %v", err)
	}
	defer repository.Close()

	createdAt := time.Now().UTC().Add(-time.Minute)
	completedAt := time.Now().UTC()
	checkID := fmt.Sprintf("chk_pg_sql_%d", createdAt.UnixNano())

	err = repository.RecordCheckCreated(ctx, CheckCreated{
		CheckID:          checkID,
		TenantID:         "integration",
		SubjectCPF:       "00000000000",
		Scenario:         "genuine_live",
		ExpectedDecision: "approved",
		Status:           "biometry_pending",
		CreatedAt:        createdAt,
		ExpiresAt:        completedAt.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}
	defer repository.db.ExecContext(context.Background(), "DELETE FROM faceproof_checks WHERE check_id = $1", checkID)

	if err := repository.MarkStarted(ctx, checkID); err != nil {
		t.Fatalf("mark started: %v", err)
	}
	if err := repository.RecordAuthoritativeState(
		ctx, checkID, "approved", "approved", &completedAt,
		0.808035334524588, 0.5836501440650698, 1,
	); err != nil {
		t.Fatalf("reconcile authoritative state: %v", err)
	}

	record := CompletionSnapshot{
		CheckID:                 checkID,
		Status:                  "approved",
		Decision:                "approved",
		CompletedAt:             completedAt,
		DocumentAttempts:        1,
		BiometricSessions:       1,
		LivenessScore:           0.808035334524588,
		FaceSimilarity:          0.5836501440650698,
		MatchThreshold:          0.363,
		LivenessThreshold:       0.68,
		ReviewLivenessThreshold: 0.55,
		RequirePassivePAD:       true,
		FrameSimilarities:       []float64{0.589, 0.585, 0.577},
		PassivePADScore:         0.9997566491365433,
		QualityScore:            0.5706449121373898,
		EmbeddingModel:          "sface",
	}
	if err := repository.RecordCompletion(ctx, record); err != nil {
		t.Fatalf("record biometric completion: %v", err)
	}

	var status string
	var similarity float64
	var liveness float64
	var margin float64
	var pad float64
	err = repository.db.QueryRowContext(
		ctx,
		`SELECT status, face_similarity, liveness_score, margin, passive_pad_score
		   FROM faceproof_checks
		  WHERE check_id = $1`,
		checkID,
	).Scan(&status, &similarity, &liveness, &margin, &pad)
	if err != nil {
		t.Fatalf("read persisted completion: %v", err)
	}
	if status != "approved" ||
		math.Abs(similarity-record.FaceSimilarity) > 1e-9 ||
		math.Abs(liveness-record.LivenessScore) > 1e-9 ||
		math.Abs(margin-(record.FaceSimilarity-record.MatchThreshold)) > 1e-9 ||
		math.Abs(pad-record.PassivePADScore) > 1e-9 {
		t.Fatalf("persisted completion differs from engine result: status=%s similarity=%f liveness=%f margin=%f pad=%f",
			status, similarity, liveness, margin, pad)
	}

	if err := repository.RecordAuthoritativeState(
		ctx, checkID, "approved", "approved", &completedAt,
		record.LivenessScore, record.FaceSimilarity, 1,
	); err != nil {
		t.Fatalf("reconcile completed record: %v", err)
	}
}
