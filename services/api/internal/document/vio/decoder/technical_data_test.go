package decoder

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

type unknownTemplateRepository struct{}

func (unknownTemplateRepository) FindTemplate(uint16) (Template, error) {
	return Template{}, ErrTemplateNotFound
}

func (unknownTemplateRepository) FindCertificates(string, int64) ([]Certificate, error) {
	return nil, errors.New("FindCertificates should not be called for an unknown template")
}

type unusedVerifier struct{}

func (unusedVerifier) Verify([]byte, []byte, Certificate) (string, error) {
	return "", errors.New("Verify should not be called for an unknown template")
}

func TestDecodeBytesPreservesUnknownTemplateFieldsAndRawBlocks(t *testing.T) {
	encodedFields := encodeVersion5FieldsForTest([]byte("ABC^DEF^GHI"))
	body := make([]byte, 0)
	body = appendUint16(body, 999)
	body = appendUint16(body, 0)
	body = appendUint16(body, 0)
	body = appendUint16(body, uint16(len(encodedFields)))
	body = append(body, encodedFields...)

	data := make([]byte, 5, 5+len(body))
	binary.BigEndian.PutUint32(data[:4], 1_700_000_000)
	data[4] = 5
	data = append(data, body...)

	service := NewService(unknownTemplateRepository{}, unusedVerifier{})
	result, err := service.DecodeBytes(data, "test")
	if err != nil {
		t.Fatalf("DecodeBytes returned error: %v", err)
	}

	if result.Technical.TemplateKnown {
		t.Fatalf("expected unknown template")
	}
	if len(result.RawFields) != 3 {
		t.Fatalf("expected 3 raw fields, got %d", len(result.RawFields))
	}
	if result.Technical.UnmappedFieldCount != 3 {
		t.Fatalf("expected 3 unmapped fields, got %d", result.Technical.UnmappedFieldCount)
	}
	if strings.TrimSpace(result.RawFields[2].Value) != "GHI" {
		t.Fatalf("unexpected last raw field: %q", result.RawFields[2].Value)
	}
	if len(result.RawQRPayload) != len(data) || len(result.HeaderBytes) != 5 || len(result.Body) != len(body) {
		t.Fatalf("raw QR blocks were not preserved")
	}
	if strings.TrimSpace(result.DecodedFieldsText) != "ABC^DEF^GHI" {
		t.Fatalf("unexpected decoded field text: %q", result.DecodedFieldsText)
	}
}

func TestMapFieldValuesKeepsValuesBeyondTemplate(t *testing.T) {
	definitions := []TemplateField{{Name: "first", Label: "First"}}
	fields, rawFields, unmapped := mapFieldValues(definitions, []string{"A", "B", "C"})

	if len(fields) != 1 {
		t.Fatalf("expected 1 mapped field, got %d", len(fields))
	}
	if len(rawFields) != 3 {
		t.Fatalf("expected all 3 decoded values, got %d", len(rawFields))
	}
	if unmapped != 2 {
		t.Fatalf("expected 2 unmapped values, got %d", unmapped)
	}
	if rawFields[1].Mapped || rawFields[1].Name != "field_2" {
		t.Fatalf("unexpected unmapped field metadata: %+v", rawFields[1])
	}
}
