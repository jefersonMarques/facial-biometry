package template

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"faceproof/services/api/internal/domain"
)

var ErrTemplateNotFound = errors.New("biometric template not found")

var subjectIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type encryptedDocument struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	Version    int    `json:"version"`
}

type Repository struct {
	directory string
	aead      cipher.AEAD
}

func NewRepository(directory string, key []byte) (*Repository, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	return &Repository{directory: directory, aead: aead}, nil
}

func (repository *Repository) Save(biometricTemplate domain.BiometricTemplate) error {
	if !subjectIDPattern.MatchString(biometricTemplate.SubjectID) {
		return errors.New("invalid subject id")
	}

	plaintext, err := json.Marshal(biometricTemplate)
	if err != nil {
		return err
	}

	nonce := make([]byte, repository.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := repository.aead.Seal(nil, nonce, plaintext, []byte(biometricTemplate.SubjectID))

	document := encryptedDocument{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Version:    1,
	}
	documentBytes, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}

	path := repository.pathFor(biometricTemplate.SubjectID)
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, documentBytes, 0o600); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func (repository *Repository) Load(subjectID string) (domain.BiometricTemplate, error) {
	if !subjectIDPattern.MatchString(subjectID) {
		return domain.BiometricTemplate{}, errors.New("invalid subject id")
	}

	documentBytes, err := os.ReadFile(repository.pathFor(subjectID))
	if os.IsNotExist(err) {
		return domain.BiometricTemplate{}, ErrTemplateNotFound
	}
	if err != nil {
		return domain.BiometricTemplate{}, err
	}

	var document encryptedDocument
	if err := json.Unmarshal(documentBytes, &document); err != nil {
		return domain.BiometricTemplate{}, err
	}
	nonce, err := base64.StdEncoding.DecodeString(document.Nonce)
	if err != nil {
		return domain.BiometricTemplate{}, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(document.Ciphertext)
	if err != nil {
		return domain.BiometricTemplate{}, err
	}

	plaintext, err := repository.aead.Open(nil, nonce, ciphertext, []byte(subjectID))
	if err != nil {
		return domain.BiometricTemplate{}, errors.New("failed to decrypt biometric template")
	}

	var biometricTemplate domain.BiometricTemplate
	if err := json.Unmarshal(plaintext, &biometricTemplate); err != nil {
		return domain.BiometricTemplate{}, err
	}
	return biometricTemplate, nil
}

func (repository *Repository) pathFor(subjectID string) string {
	return filepath.Join(repository.directory, subjectID+".json")
}
