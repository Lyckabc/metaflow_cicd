package runner

import (
	"os"
	"path/filepath"

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
	Artifacts      struct {
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
	return filepath.Ext(configPath) == ".toml"
}
