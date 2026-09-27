package data

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"faceproof/services/api/internal/document/vio/decoder"
)

//go:embed assets/vio_templates.json
var templatesJSON []byte

//go:embed assets/vio_certificates.json
var certificatesJSON []byte

type templateRecord struct {
	ID    uint16 `json:"id"`
	Name  string `json:"name"`
	Owner struct {
		Name string `json:"name"`
	} `json:"owner"`
	Fields []struct {
		Name  string `json:"name"`
		Label string `json:"label"`
	} `json:"fields"`
	CertificateGroup struct {
		ID string `json:"id"`
	} `json:"certificate_group"`
}

type certificateRecord struct {
	ID    string `json:"id"`
	Group struct {
		ID string `json:"id"`
	} `json:"group"`
	PublicKey string `json:"public_key"`
	Valid     struct {
		From  time.Time `json:"from"`
		Until time.Time `json:"until"`
	} `json:"valid"`
}

type Repository struct {
	once         sync.Once
	loadErr      error
	templates    []templateRecord
	certificates []certificateRecord
}

func NewRepository() *Repository {
	return &Repository{}
}

func (repository *Repository) FindTemplate(id uint16) (decoder.Template, error) {
	if err := repository.load(); err != nil {
		return decoder.Template{}, err
	}

	for _, record := range repository.templates {
		if record.ID == id {
			return templateFromRecord(record), nil
		}
	}

	return decoder.Template{}, fmt.Errorf("%w: %d", decoder.ErrTemplateNotFound, id)
}

func (repository *Repository) ListTemplates() ([]decoder.Template, error) {
	if err := repository.load(); err != nil {
		return nil, err
	}

	templates := make([]decoder.Template, 0, len(repository.templates))
	for _, record := range repository.templates {
		templates = append(templates, templateFromRecord(record))
	}
	return templates, nil
}

func templateFromRecord(record templateRecord) decoder.Template {
	fields := make([]decoder.TemplateField, 0, len(record.Fields))
	for _, field := range record.Fields {
		fields = append(fields, decoder.TemplateField{Name: field.Name, Label: field.Label})
	}

	return decoder.Template{
		ID:               record.ID,
		Name:             record.Name,
		OwnerName:        record.Owner.Name,
		Fields:           fields,
		CertificateGroup: record.CertificateGroup.ID,
	}
}

func (repository *Repository) FindCertificates(groupID string, unixSeconds int64) ([]decoder.Certificate, error) {
	if err := repository.load(); err != nil {
		return nil, err
	}

	createdAt := time.Unix(unixSeconds, 0)
	certificates := make([]decoder.Certificate, 0, 2)

	for _, record := range repository.certificates {
		if record.Group.ID != groupID {
			continue
		}
		if createdAt.Before(record.Valid.From) || createdAt.After(record.Valid.Until) {
			continue
		}

		certificates = append(certificates, decoder.Certificate{
			ID:        record.ID,
			PublicKey: record.PublicKey,
		})
	}

	if len(certificates) == 0 {
		return nil, fmt.Errorf("%w para o grupo %s na data %s", decoder.ErrCertificateNotFound, groupID, createdAt.Format(time.RFC3339))
	}

	return certificates, nil
}

func (repository *Repository) load() error {
	repository.once.Do(func() {
		if len(templatesJSON) == 0 || len(certificatesJSON) == 0 {
			repository.loadErr = errors.New("assets de templates/certificados não carregados")
			return
		}

		if err := json.Unmarshal(templatesJSON, &repository.templates); err != nil {
			repository.loadErr = fmt.Errorf("falha ao carregar templates Vio: %w", err)
			return
		}

		if err := json.Unmarshal(certificatesJSON, &repository.certificates); err != nil {
			repository.loadErr = fmt.Errorf("falha ao carregar certificados Vio: %w", err)
		}
	})

	return repository.loadErr
}
