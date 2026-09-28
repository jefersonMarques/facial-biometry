package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var ErrNotFound = errors.New("identity check not found")

var checkIDPattern = regexp.MustCompile(`^chk_[A-Za-z0-9_-]{8,64}$`)

type encryptedDocument struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	Version    int    `json:"version"`
}

type Repository struct {
	directory      string
	indexDirectory string
	aead           cipher.AEAD
	mu             sync.RWMutex
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

	indexDirectory := filepath.Join(directory, "by-id")
	if err := os.MkdirAll(indexDirectory, 0o700); err != nil {
		return nil, err
	}

	return &Repository{
		directory:      directory,
		indexDirectory: indexDirectory,
		aead:           aead,
	}, nil
}

func (repository *Repository) Create(token string, check Check) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()

	if !checkIDPattern.MatchString(check.ID) {
		return errors.New("invalid identity check id")
	}

	tokenHash := hashToken(token)
	path := repository.pathFor(tokenHash)
	if _, err := os.Stat(path); err == nil {
		return errors.New("identity token collision")
	} else if !os.IsNotExist(err) {
		return err
	}

	if _, err := os.Stat(repository.indexPath(check.ID)); err == nil {
		return errors.New("identity check id collision")
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := repository.saveLocked(tokenHash, check); err != nil {
		return err
	}
	if err := repository.saveIndexLocked(check.ID, tokenHash); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func (repository *Repository) Load(token string) (Check, error) {
	repository.mu.RLock()
	defer repository.mu.RUnlock()
	return repository.loadLocked(hashToken(token))
}

func (repository *Repository) LoadByID(id string) (Check, error) {
	repository.mu.RLock()
	defer repository.mu.RUnlock()

	tokenHash, err := repository.tokenHashForIDLocked(id)
	if err != nil {
		return Check{}, err
	}
	return repository.loadLocked(tokenHash)
}

func (repository *Repository) Update(token string, mutate func(*Check) error) (Check, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.updateLocked(hashToken(token), mutate)
}

func (repository *Repository) UpdateByID(id string, mutate func(*Check) error) (Check, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()

	tokenHash, err := repository.tokenHashForIDLocked(id)
	if err != nil {
		return Check{}, err
	}
	return repository.updateLocked(tokenHash, mutate)
}

func (repository *Repository) updateLocked(tokenHash string, mutate func(*Check) error) (Check, error) {
	check, err := repository.loadLocked(tokenHash)
	if err != nil {
		return Check{}, err
	}
	if err := mutate(&check); err != nil {
		return Check{}, err
	}
	if err := repository.saveLocked(tokenHash, check); err != nil {
		return Check{}, err
	}
	return check, nil
}

func (repository *Repository) loadLocked(tokenHash string) (Check, error) {
	documentBytes, err := os.ReadFile(repository.pathFor(tokenHash))
	if os.IsNotExist(err) {
		return Check{}, ErrNotFound
	}
	if err != nil {
		return Check{}, err
	}

	var document encryptedDocument
	if err := json.Unmarshal(documentBytes, &document); err != nil {
		return Check{}, err
	}
	if document.Version != 1 {
		return Check{}, errors.New("unsupported identity document version")
	}

	nonce, err := base64.StdEncoding.DecodeString(document.Nonce)
	if err != nil {
		return Check{}, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(document.Ciphertext)
	if err != nil {
		return Check{}, err
	}
	plaintext, err := repository.aead.Open(nil, nonce, ciphertext, []byte(tokenHash))
	if err != nil {
		return Check{}, errors.New("failed to decrypt identity check")
	}

	var check Check
	if err := json.Unmarshal(plaintext, &check); err != nil {
		return Check{}, err
	}
	return check, nil
}

func (repository *Repository) saveLocked(tokenHash string, check Check) error {
	plaintext, err := json.Marshal(check)
	if err != nil {
		return err
	}

	nonce := make([]byte, repository.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := repository.aead.Seal(nil, nonce, plaintext, []byte(tokenHash))
	document := encryptedDocument{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Version:    1,
	}
	documentBytes, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}

	path := repository.pathFor(tokenHash)
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, documentBytes, 0o600); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func (repository *Repository) saveIndexLocked(id, tokenHash string) error {
	path := repository.indexPath(id)
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, []byte(tokenHash), 0o600); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func (repository *Repository) tokenHashForIDLocked(id string) (string, error) {
	if !checkIDPattern.MatchString(id) {
		return "", ErrNotFound
	}
	value, err := os.ReadFile(repository.indexPath(id))
	if os.IsNotExist(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	tokenHash := string(value)
	if len(tokenHash) != sha256.Size*2 {
		return "", errors.New("invalid identity check index")
	}
	if _, err := hex.DecodeString(tokenHash); err != nil {
		return "", errors.New("invalid identity check index")
	}
	return tokenHash, nil
}

func (repository *Repository) pathFor(tokenHash string) string {
	return filepath.Join(repository.directory, tokenHash+".json")
}

func (repository *Repository) indexPath(id string) string {
	return filepath.Join(repository.indexDirectory, id)
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
