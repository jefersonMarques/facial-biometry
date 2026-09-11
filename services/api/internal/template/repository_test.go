package template

import (
	"bytes"
	"testing"
	"time"

	"faceproof/services/api/internal/domain"
)

func TestRepositoryEncryptsAndLoadsTemplate(t *testing.T) {
	directory := t.TempDir()
	key := bytes.Repeat([]byte{7}, 32)
	repository, err := NewRepository(directory, key)
	if err != nil {
		t.Fatal(err)
	}

	input := domain.BiometricTemplate{
		SubjectID:      "person-1",
		Embedding:      []float64{0.1, 0.2, 0.3},
		EmbeddingModel: "test",
		CreatedAt:      time.Now().UTC(),
	}
	if err := repository.Save(input); err != nil {
		t.Fatal(err)
	}

	output, err := repository.Load("person-1")
	if err != nil {
		t.Fatal(err)
	}
	if output.SubjectID != input.SubjectID || len(output.Embedding) != len(input.Embedding) {
		t.Fatalf("unexpected template: %+v", output)
	}
}
