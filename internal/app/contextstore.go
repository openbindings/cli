package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/zalando/go-keyring"
)

// ContextConfig holds the non-secret fields of a named context.
// Persisted as JSON in ~/.config/openbindings/contexts/<name>.json.
type ContextConfig struct {
	Headers     map[string]string `json:"headers,omitempty"`
	Cookies     map[string]string `json:"cookies,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// ContextSummary is a compact representation for listing contexts.
type ContextSummary struct {
	Name           string `json:"name"`
	HasCredentials bool   `json:"hasCredentials"`
	HeaderCount    int    `json:"headerCount,omitempty"`
	CookieCount    int    `json:"cookieCount,omitempty"`
	EnvCount       int    `json:"envCount,omitempty"`
	MetadataCount  int    `json:"metadataCount,omitempty"`
	LoadError      string `json:"loadError,omitempty"`
}

// contextsDirFunc is the resolver for the contexts directory.
// Override in tests to use a temp directory.
var contextsDirFunc = defaultContextsDir

func defaultContextsDir() (string, error) {
	globalPath, err := GlobalConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(globalPath, ContextsDir), nil
}

// contextsDir returns the path to the contexts directory.
func contextsDir() (string, error) {
	return contextsDirFunc()
}

// contextConfigPath returns the JSON file path for a named context.
func contextConfigPath(name string) (string, error) {
	dir, err := contextsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// LoadContextConfig reads the non-secret config for a named context.
// Returns an empty config (not an error) if the file does not exist.
func LoadContextConfig(name string) (ContextConfig, error) {
	path, err := contextConfigPath(name)
	if err != nil {
		return ContextConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ContextConfig{}, nil
		}
		return ContextConfig{}, fmt.Errorf("reading context config %q: %w", name, err)
	}
	var cfg ContextConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ContextConfig{}, fmt.Errorf("parsing context config %q: %w", name, err)
	}
	return cfg, nil
}

// SaveContextConfig writes the non-secret config for a named context.
func SaveContextConfig(name string, cfg ContextConfig) error {
	dir, err := contextsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return fmt.Errorf("creating contexts directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling context config: %w", err)
	}
	path := filepath.Join(dir, name+".json")
	return AtomicWriteFile(path, data, FilePerm)
}

// LoadContextCredentials reads credentials from the OS keychain.
// Returns nil (not an error) if no credentials are stored.
func LoadContextCredentials(name string) (*delegates.Credentials, error) {
	secret, err := keyring.Get(KeychainService, name)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("reading keychain for context %q: %w", name, err)
	}
	var cred delegates.Credentials
	if err := json.Unmarshal([]byte(secret), &cred); err != nil {
		return nil, fmt.Errorf("parsing keychain credentials for context %q: %w", name, err)
	}
	return &cred, nil
}

// SaveContextCredentials writes credentials to the OS keychain as JSON.
func SaveContextCredentials(name string, cred *delegates.Credentials) error {
	if cred == nil {
		return DeleteContextCredentials(name)
	}
	data, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	if err := keyring.Set(KeychainService, name, string(data)); err != nil {
		return fmt.Errorf("writing keychain for context %q: %w", name, err)
	}
	return nil
}

// DeleteContextCredentials removes credentials from the OS keychain.
func DeleteContextCredentials(name string) error {
	err := keyring.Delete(KeychainService, name)
	if err != nil && err != keyring.ErrNotFound {
		return fmt.Errorf("deleting keychain for context %q: %w", name, err)
	}
	return nil
}

// LoadContext assembles a full BindingContext from config file + keychain.
func LoadContext(name string) (delegates.BindingContext, error) {
	cfg, err := LoadContextConfig(name)
	if err != nil {
		return delegates.BindingContext{}, err
	}
	cred, err := LoadContextCredentials(name)
	if err != nil {
		return delegates.BindingContext{}, err
	}

	return delegates.BindingContext{
		Credentials: cred,
		Headers:     cfg.Headers,
		Cookies:     cfg.Cookies,
		Environment: cfg.Environment,
		Metadata:    cfg.Metadata,
	}, nil
}

// DeleteContext removes both the config file and keychain entry for a context.
func DeleteContext(name string) error {
	path, err := contextConfigPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing context config %q: %w", name, err)
	}
	return DeleteContextCredentials(name)
}

// ContextExists returns true if a context has a config file or keychain entry.
func ContextExists(name string) bool {
	path, err := contextConfigPath(name)
	if err != nil {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	_, err = keyring.Get(KeychainService, name)
	return err == nil
}

// ListContexts returns summaries of all named contexts found in the config directory.
func ListContexts() ([]ContextSummary, error) {
	dir, err := contextsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading contexts directory: %w", err)
	}

	var summaries []ContextSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		cfg, err := LoadContextConfig(name)
		if err != nil {
			summaries = append(summaries, ContextSummary{
				Name:      name,
				LoadError: err.Error(),
			})
			continue
		}
		hasCreds := false
		if _, kerr := keyring.Get(KeychainService, name); kerr == nil {
			hasCreds = true
		}
		summaries = append(summaries, ContextSummary{
			Name:           name,
			HasCredentials: hasCreds,
			HeaderCount:    len(cfg.Headers),
			CookieCount:    len(cfg.Cookies),
			EnvCount:       len(cfg.Environment),
			MetadataCount:  len(cfg.Metadata),
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	return summaries, nil
}
