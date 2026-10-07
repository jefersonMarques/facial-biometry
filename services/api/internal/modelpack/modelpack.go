package modelpack

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	SchemaVersion = 1

	RoleYUNet      = "yunet"
	RoleSFace      = "sface"
	RoleMiniFASNet = "minifasnet"
)

var requiredRoles = []string{
	RoleYUNet,
	RoleSFace,
	RoleMiniFASNet,
}

type Manifest struct {
	SchemaVersion        int         `json:"schemaVersion"`
	PackID               string      `json:"packId"`
	PackVersion          string      `json:"packVersion"`
	SecureCoreMinVersion string      `json:"secureCoreMinVersion"`
	CreatedAt            string      `json:"createdAt"`
	Models               []ModelSpec `json:"models"`
}

type ModelSpec struct {
	Role    string `json:"role"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type SourceModel struct {
	Role    string
	Name    string
	Version string
	Path    string
}

type CreateOptions struct {
	OutputPath           string
	PrivateKeyPath       string
	PackID               string
	PackVersion          string
	SecureCoreMinVersion string
	Models               []SourceModel
}

type VerifiedPack struct {
	Manifest          Manifest
	ManifestSHA256    string
	ExtractedRoot     string
	YUNetPath         string
	SFacePath         string
	MiniFASNetPath    string
}

func GenerateKeyPair(privatePath string, publicPath string) error {
	if strings.TrimSpace(privatePath) == "" || strings.TrimSpace(publicPath) == "" {
		return errors.New("private and public key paths are required")
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Ed25519 key: %w", err)
	}

	if err := writeSecret(privatePath, base64.StdEncoding.EncodeToString(privateKey)+"\n", 0o600); err != nil {
		return err
	}
	if err := writeSecret(publicPath, base64.StdEncoding.EncodeToString(publicKey)+"\n", 0o644); err != nil {
		return err
	}
	return nil
}

func Create(options CreateOptions) (Manifest, error) {
	privateKey, err := readPrivateKey(options.PrivateKeyPath)
	if err != nil {
		return Manifest{}, err
	}
	if strings.TrimSpace(options.OutputPath) == "" {
		return Manifest{}, errors.New("model pack output path is required")
	}
	if strings.TrimSpace(options.PackID) == "" || strings.TrimSpace(options.PackVersion) == "" {
		return Manifest{}, errors.New("model pack id and version are required")
	}
	if strings.TrimSpace(options.SecureCoreMinVersion) == "" {
		return Manifest{}, errors.New("Secure Core minimum version is required")
	}

	sources, err := normalizeSources(options.Models)
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		SchemaVersion:        SchemaVersion,
		PackID:               strings.TrimSpace(options.PackID),
		PackVersion:          strings.TrimSpace(options.PackVersion),
		SecureCoreMinVersion: strings.TrimSpace(options.SecureCoreMinVersion),
		CreatedAt:            time.Now().UTC().Format(time.RFC3339),
		Models:               make([]ModelSpec, 0, len(sources)),
	}

	files := make(map[string][]byte, len(sources))
	for _, source := range sources {
		data, err := os.ReadFile(source.Path)
		if err != nil {
			return Manifest{}, fmt.Errorf("read %s model: %w", source.Role, err)
		}
		if len(data) == 0 {
			return Manifest{}, fmt.Errorf("%s model is empty", source.Role)
		}

		archivePath := "models/" + source.Role + ".onnx"
		digest := sha256.Sum256(data)
		manifest.Models = append(manifest.Models, ModelSpec{
			Role:    source.Role,
			Name:    source.Name,
			Version: source.Version,
			Path:    archivePath,
			SHA256:  hex.EncodeToString(digest[:]),
			Size:    int64(len(data)),
		})
		files[archivePath] = data
	}

	sort.Slice(manifest.Models, func(i int, j int) bool {
		return manifest.Models[i].Role < manifest.Models[j].Role
	})

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("encode model pack manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	signature := ed25519.Sign(privateKey, manifestBytes)

	if err := os.MkdirAll(filepath.Dir(options.OutputPath), 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create model pack directory: %w", err)
	}

	temporary := options.OutputPath + ".part"
	_ = os.Remove(temporary)
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return Manifest{}, fmt.Errorf("create model pack: %w", err)
	}

	zipWriter := zip.NewWriter(output)
	writeErr := writeZipEntry(zipWriter, "manifest.json", manifestBytes, 0o600)
	if writeErr == nil {
		writeErr = writeZipEntry(zipWriter, "signature.ed25519", signature, 0o600)
	}
	if writeErr == nil {
		modelPaths := make([]string, 0, len(files))
		for path := range files {
			modelPaths = append(modelPaths, path)
		}
		sort.Strings(modelPaths)
		for _, path := range modelPaths {
			if err := writeZipEntry(zipWriter, path, files[path], 0o600); err != nil {
				writeErr = err
				break
			}
		}
	}
	closeZipErr := zipWriter.Close()
	closeFileErr := output.Close()
	if writeErr != nil {
		_ = os.Remove(temporary)
		return Manifest{}, writeErr
	}
	if closeZipErr != nil {
		_ = os.Remove(temporary)
		return Manifest{}, fmt.Errorf("close model pack archive: %w", closeZipErr)
	}
	if closeFileErr != nil {
		_ = os.Remove(temporary)
		return Manifest{}, fmt.Errorf("close model pack file: %w", closeFileErr)
	}
	if err := os.Rename(temporary, options.OutputPath); err != nil {
		_ = os.Remove(temporary)
		return Manifest{}, fmt.Errorf("publish model pack: %w", err)
	}
	return manifest, nil
}

func VerifyAndExtract(packPath string, publicKeyValue string, cacheRoot string) (VerifiedPack, error) {
	publicKey, err := ParsePublicKey(publicKeyValue)
	if err != nil {
		return VerifiedPack{}, err
	}
	if strings.TrimSpace(packPath) == "" {
		return VerifiedPack{}, errors.New("model pack path is required")
	}
	if strings.TrimSpace(cacheRoot) == "" {
		return VerifiedPack{}, errors.New("model pack cache directory is required")
	}

	reader, err := zip.OpenReader(packPath)
	if err != nil {
		return VerifiedPack{}, fmt.Errorf("open model pack: %w", err)
	}
	defer reader.Close()

	entries := make(map[string]*zip.File, len(reader.File))
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if _, exists := entries[entry.Name]; exists {
			return VerifiedPack{}, fmt.Errorf("duplicate model pack entry %q", entry.Name)
		}
		entries[entry.Name] = entry
	}

	manifestBytes, err := readZipEntry(entries["manifest.json"], 1<<20)
	if err != nil {
		return VerifiedPack{}, fmt.Errorf("read model pack manifest: %w", err)
	}
	signature, err := readZipEntry(entries["signature.ed25519"], ed25519.SignatureSize)
	if err != nil {
		return VerifiedPack{}, fmt.Errorf("read model pack signature: %w", err)
	}
	if len(signature) != ed25519.SignatureSize || !ed25519.Verify(publicKey, manifestBytes, signature) {
		return VerifiedPack{}, errors.New("model pack signature is invalid")
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return VerifiedPack{}, fmt.Errorf("decode model pack manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return VerifiedPack{}, err
	}

	manifestDigest := sha256.Sum256(manifestBytes)
	manifestSHA := hex.EncodeToString(manifestDigest[:])
	extractedRoot := filepath.Join(cacheRoot, manifestSHA)
	if err := os.MkdirAll(extractedRoot, 0o700); err != nil {
		return VerifiedPack{}, fmt.Errorf("create model pack cache: %w", err)
	}

	result := VerifiedPack{
		Manifest:       manifest,
		ManifestSHA256: manifestSHA,
		ExtractedRoot:  extractedRoot,
	}

	for _, model := range manifest.Models {
		entry := entries[model.Path]
		if entry == nil {
			return VerifiedPack{}, fmt.Errorf("model pack entry missing: %s", model.Path)
		}
		if int64(entry.UncompressedSize64) != model.Size {
			return VerifiedPack{}, fmt.Errorf("model size mismatch for %s", model.Role)
		}
		data, err := readZipEntry(entry, model.Size)
		if err != nil {
			return VerifiedPack{}, fmt.Errorf("read %s model: %w", model.Role, err)
		}
		digest := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(digest[:]), model.SHA256) {
			return VerifiedPack{}, fmt.Errorf("model SHA-256 mismatch for %s", model.Role)
		}

		target := filepath.Join(extractedRoot, model.Role+".onnx")
		if err := writeSecret(target, string(data), 0o600); err != nil {
			return VerifiedPack{}, err
		}

		switch model.Role {
		case RoleYUNet:
			result.YUNetPath = target
		case RoleSFace:
			result.SFacePath = target
		case RoleMiniFASNet:
			result.MiniFASNetPath = target
		}
	}

	if result.YUNetPath == "" || result.SFacePath == "" || result.MiniFASNetPath == "" {
		return VerifiedPack{}, errors.New("model pack did not resolve all required model roles")
	}
	return result, nil
}

func ValidateSecureCoreCompatibility(manifest Manifest, currentVersion string) error {
	required, err := parseVersion(manifest.SecureCoreMinVersion)
	if err != nil {
		return fmt.Errorf("invalid Secure Core minimum version: %w", err)
	}
	current, err := parseVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("invalid current Secure Core version: %w", err)
	}
	for index := 0; index < len(required); index++ {
		if current[index] > required[index] {
			return nil
		}
		if current[index] < required[index] {
			return fmt.Errorf(
				"model pack requires Secure Core >= %s; current is %s",
				manifest.SecureCoreMinVersion,
				currentVersion,
			)
		}
	}
	return nil
}

func parseVersion(value string) ([3]int, error) {
	var result [3]int
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 3 {
		return result, fmt.Errorf("version %q must use major.minor.patch", value)
	}
	for index, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return result, fmt.Errorf("invalid version %q", value)
		}
		result[index] = parsed
	}
	return result, nil
}

func ParsePublicKey(value string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("decode model pack public key: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("model pack public key must be %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(decoded), nil
}

func readPrivateKey(path string) (ed25519.PrivateKey, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model pack private key: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(value)))
	if err != nil {
		return nil, fmt.Errorf("decode model pack private key: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("model pack private key must be %d bytes", ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(decoded), nil
}

func normalizeSources(models []SourceModel) ([]SourceModel, error) {
	if len(models) != len(requiredRoles) {
		return nil, fmt.Errorf("model pack requires exactly %d models", len(requiredRoles))
	}

	seen := map[string]bool{}
	normalized := make([]SourceModel, 0, len(models))
	for _, model := range models {
		model.Role = strings.TrimSpace(strings.ToLower(model.Role))
		model.Name = strings.TrimSpace(model.Name)
		model.Version = strings.TrimSpace(model.Version)
		model.Path = strings.TrimSpace(model.Path)
		if model.Role == "" || model.Name == "" || model.Version == "" || model.Path == "" {
			return nil, errors.New("model role, name, version and path are required")
		}
		if seen[model.Role] {
			return nil, fmt.Errorf("duplicate model role %q", model.Role)
		}
		seen[model.Role] = true
		normalized = append(normalized, model)
	}
	for _, role := range requiredRoles {
		if !seen[role] {
			return nil, fmt.Errorf("missing required model role %q", role)
		}
	}
	return normalized, nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported model pack schema version %d", manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.PackID) == "" ||
		strings.TrimSpace(manifest.PackVersion) == "" ||
		strings.TrimSpace(manifest.SecureCoreMinVersion) == "" {
		return errors.New("model pack manifest metadata is incomplete")
	}
	if len(manifest.Models) != len(requiredRoles) {
		return errors.New("model pack manifest has an invalid model count")
	}

	seen := map[string]bool{}
	for _, model := range manifest.Models {
		if seen[model.Role] {
			return fmt.Errorf("duplicate model role %q", model.Role)
		}
		seen[model.Role] = true
		if model.Size <= 0 {
			return fmt.Errorf("invalid model size for %s", model.Role)
		}
		if len(model.SHA256) != sha256.Size*2 {
			return fmt.Errorf("invalid SHA-256 for %s", model.Role)
		}
		expectedPath := "models/" + model.Role + ".onnx"
		if model.Path != expectedPath {
			return fmt.Errorf("unexpected archive path for %s", model.Role)
		}
	}
	for _, role := range requiredRoles {
		if !seen[role] {
			return fmt.Errorf("missing required model role %q", role)
		}
	}
	return nil
}

func writeZipEntry(writer *zip.Writer, name string, data []byte, mode os.FileMode) error {
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetMode(mode)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create model pack entry %s: %w", name, err)
	}
	if _, err := io.Copy(entry, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write model pack entry %s: %w", name, err)
	}
	return nil
}

func readZipEntry(entry *zip.File, maxSize int64) ([]byte, error) {
	if entry == nil {
		return nil, errors.New("entry is missing")
	}
	if maxSize <= 0 {
		return nil, errors.New("invalid entry size limit")
	}
	if entry.UncompressedSize64 > uint64(maxSize) {
		return nil, errors.New("entry exceeds declared size limit")
	}

	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	limited := io.LimitReader(reader, maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, errors.New("entry exceeds size limit")
	}
	return data, nil
}

func writeSecret(path string, value string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	temporary := path + ".part"
	if err := os.WriteFile(temporary, []byte(value), mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Chmod(temporary, mode); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish %s: %w", path, err)
	}
	return nil
}
