package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// FetchAndParseConfig clones the repo and parses metaflow-ci.toml to get secrets_mapping.
// Used by PreFlightCheck to resolve registry credentials from DB secrets by key.
func FetchAndParseConfig(ctx context.Context, gitURL, branch, accessToken, configPath string) (*MetaflowCITOML, error) {
	if !IsTOMLConfig(configPath) {
		return nil, nil
	}
	tmpDir, err := os.MkdirTemp("", "metaflow-cicd-config-*")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneURL := gitURL
	if accessToken != "" {
		cloneURL = injectTokenIntoURL(gitURL, accessToken)
	}
	if branch == "" {
		branch = "main"
	}
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, cloneURL, tmpDir)
	if err := cloneCmd.Run(); err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}

	fullPath := filepath.Join(tmpDir, configPath)
	cfg, err := ParseMetaflowCITOML(fullPath)
	if err != nil {
		return nil, fmt.Errorf("parse metaflow-ci.toml: %w", err)
	}
	return cfg, nil
}
