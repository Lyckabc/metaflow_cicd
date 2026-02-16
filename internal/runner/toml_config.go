package runner

import (
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// MetaflowCITOML is the standard metaflow-ci.toml structure.
// See: flows/metaflow-ci.toml
type MetaflowCITOML struct {
	Project struct {
		Name     string `toml:"name"`
		Language string `toml:"language"`
		Version  string `toml:"version"`
	} `toml:"project"`
	Build struct {
		Entrypoint string `toml:"entrypoint"`
		PreBuild   string `toml:"pre_build"`
		Command    string `toml:"command"`
	} `toml:"build"`
	Config map[string]string `toml:"config"`
	SecretsMapping map[string]string `toml:"secrets_mapping"`
	Registry       struct {
		Enabled    bool   `toml:"enabled"`
		Dockerfile string `toml:"dockerfile"`
	} `toml:"registry"`
	Artifacts struct {
		RegistryName string `toml:"REGISTRY_NAME"`
	} `toml:"artifacts"`
}

// ParseMetaflowCITOML reads and parses a metaflow-ci.toml file.
func ParseMetaflowCITOML(path string) (*MetaflowCITOML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg MetaflowCITOML
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// IsTOMLConfig returns true if configPath has .toml extension.
func IsTOMLConfig(configPath string) bool {
	p := strings.TrimSpace(configPath)
	return strings.HasSuffix(strings.ToLower(p), ".toml")
}

// GetSecretValue returns the secret_value from secrets map using the secret_key.
// secrets_mapping maps [env_var_name] = [DB secrets.secret_key].
// Given envVarName (e.g. "REGISTRY_URL"), looks up secret_key (e.g. "TOJI_REGISTRY_URL")
// and returns secrets[secret_key] (the secret_value from DB).
// Returns empty string if envVarName is not in secrets_mapping or secret_key is not in secrets.
func (c *MetaflowCITOML) GetSecretValue(secrets map[string]string, envVarName string) string {
	if c.SecretsMapping == nil {
		return ""
	}
	secretKey, ok := c.SecretsMapping[envVarName]
	if !ok || secretKey == "" {
		return ""
	}
	return secrets[secretKey]
}

// RegistryEnvVars are the standard env var names for registry credentials in secrets_mapping.
const (
	RegistryURLEnv      = "REGISTRY_URL"
	RegistryIDEnv       = "REGISTRY_ID"
	RegistryPasswordEnv = "REGISTRY_PASSWORD"
)

// GetRegistryFromSecrets returns (url, id, password) using secrets_mapping.
// Uses REGISTRY_URL, REGISTRY_ID, REGISTRY_PASSWORD keys from secrets_mapping.
func (c *MetaflowCITOML) GetRegistryFromSecrets(secrets map[string]string) (url, id, password string) {
	url = strings.TrimSpace(c.GetSecretValue(secrets, RegistryURLEnv))
	id = strings.TrimSpace(c.GetSecretValue(secrets, RegistryIDEnv))
	password = c.GetSecretValue(secrets, RegistryPasswordEnv)
	return url, id, password
}
