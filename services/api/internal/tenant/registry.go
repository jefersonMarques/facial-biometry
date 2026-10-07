package tenant

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const SchemaVersion = 1

var tenantIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}[a-z0-9]$`)

type Tenant struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type fileTenant struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	APIKeySHA256 string `json:"apiKeySha256"`
	Enabled      bool   `json:"enabled"`
}

type fileDocument struct {
	SchemaVersion int          `json:"schemaVersion"`
	Tenants       []fileTenant `json:"tenants"`
}

type credential struct {
	Tenant Tenant
	Hash   string
}

type Registry struct {
	credentials []credential
}

func Empty() *Registry {
	return &Registry{}
}

func FromLegacyKey(id string, name string, apiKey []byte) (*Registry, error) {
	if len(apiKey) == 0 {
		return Empty(), nil
	}
	id = strings.TrimSpace(strings.ToLower(id))
	name = strings.TrimSpace(name)
	if !tenantIDPattern.MatchString(id) {
		return nil, errors.New("legacy tenant id is invalid")
	}
	if name == "" {
		return nil, errors.New("legacy tenant name is required")
	}
	sum := sha256.Sum256(apiKey)
	return &Registry{
		credentials: []credential{{
			Tenant: Tenant{
				ID:      id,
				Name:    name,
				Enabled: true,
			},
			Hash: hex.EncodeToString(sum[:]),
		}},
	}, nil
}

func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tenant registry: %w", err)
	}

	var document fileDocument
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode tenant registry: %w", err)
	}
	if document.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported tenant registry schema version %d", document.SchemaVersion)
	}
	if len(document.Tenants) == 0 {
		return nil, errors.New("tenant registry must contain at least one tenant")
	}

	ids := make(map[string]bool, len(document.Tenants))
	hashes := make(map[string]bool, len(document.Tenants))
	credentials := make([]credential, 0, len(document.Tenants))

	for _, item := range document.Tenants {
		id := strings.TrimSpace(strings.ToLower(item.ID))
		name := strings.TrimSpace(item.Name)
		hash := strings.ToLower(strings.TrimSpace(item.APIKeySHA256))

		if !tenantIDPattern.MatchString(id) {
			return nil, fmt.Errorf("tenant id %q is invalid", item.ID)
		}
		if name == "" {
			return nil, fmt.Errorf("tenant %s name is required", id)
		}
		if len(hash) != sha256.Size*2 {
			return nil, fmt.Errorf("tenant %s API key hash is invalid", id)
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return nil, fmt.Errorf("tenant %s API key hash is invalid", id)
		}
		if ids[id] {
			return nil, fmt.Errorf("duplicate tenant id %q", id)
		}
		if hashes[hash] {
			return nil, errors.New("duplicate tenant API key hash")
		}
		ids[id] = true
		hashes[hash] = true

		credentials = append(credentials, credential{
			Tenant: Tenant{
				ID:      id,
				Name:    name,
				Enabled: item.Enabled,
			},
			Hash: hash,
		})
	}

	return &Registry{credentials: credentials}, nil
}

func (registry *Registry) AuthenticateBearer(authorization string) (Tenant, bool) {
	if registry == nil {
		return Tenant{}, false
	}

	const prefix = "Bearer "
	authorization = strings.TrimSpace(authorization)
	if !strings.HasPrefix(authorization, prefix) {
		return Tenant{}, false
	}
	apiKey := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	if len(apiKey) < 32 {
		return Tenant{}, false
	}

	sum := sha256.Sum256([]byte(apiKey))
	provided := []byte(hex.EncodeToString(sum[:]))

	var matched Tenant
	found := 0
	for _, candidate := range registry.credentials {
		equal := subtle.ConstantTimeCompare(provided, []byte(candidate.Hash))
		if equal == 1 && candidate.Tenant.Enabled {
			matched = candidate.Tenant
			found = 1
		}
	}
	return matched, found == 1
}

func (registry *Registry) Count() int {
	if registry == nil {
		return 0
	}
	return len(registry.credentials)
}
