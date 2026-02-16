package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neunexus/metaflow_cicd/workflow"
)

// RunMetaflowActivity clones repo, parses metaflow-ci.toml (or legacy .py), injects secrets, runs pre_build+command.
//
// Flow (per guide):
// 1. Git Clone: clone from Git URL (with token if provided)
// 2. Config Load: if ci_config_path is .toml → parse [build], [secrets_mapping], [config]
// 3. Secret Injection: apply secrets_mapping (env_var = DB secret_key) → set env from input.Secrets
// 4. Execution: pre_build → command (from TOML) or legacy "python <path> run"
// 5. Clean-up: remove temp dir
func RunMetaflowActivity(ctx context.Context, input *workflow.RunnerInput) (*workflow.RunResult, error) {
	tmpDir, err := os.MkdirTemp("", "metaflow-cicd-"+input.ProjectName+"-*")
	if err != nil {
		return &workflow.RunResult{Stderr: err.Error(), ExitCode: 1, Success: false}, nil
	}
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	// 1. Git Clone
	cloneURL := input.GitURL
	if input.AccessToken != "" {
		cloneURL = injectTokenIntoURL(input.GitURL, input.AccessToken)
	}
	branch := input.Branch
	if branch == "" {
		branch = "main"
	}
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, cloneURL, tmpDir)
	var cloneOut, cloneErr bytes.Buffer
	cloneCmd.Stdout = &cloneOut
	cloneCmd.Stderr = &cloneErr
	if err := cloneCmd.Run(); err != nil {
		return &workflow.RunResult{
			Stdout:   cloneOut.String(),
			Stderr:   cloneErr.String() + "\nclone error: " + err.Error(),
			ExitCode: 1,
			Success:  false,
		}, nil
	}

	// 2. Config Load
	configPath := strings.TrimSpace(input.ConfigPath)
	configFullPath := filepath.Join(tmpDir, configPath)
	if _, err := os.Stat(configFullPath); os.IsNotExist(err) {
		return &workflow.RunResult{
			Stderr:   fmt.Sprintf("config file not found: %s", configPath),
			ExitCode: 1,
			Success:  false,
		}, nil
	}

	// Build env from current process + secrets
	envMap := envFromOS()
	for k, v := range input.Secrets {
		envMap[k] = v
	}

	// TOML 우선 시도: 파싱 성공 시 사용 (경로/확장자 의존 제거)
	var preBuild, command string
	tomlCfg, parseErr := ParseMetaflowCITOML(configFullPath)
	if parseErr == nil && tomlCfg != nil && (tomlCfg.Build.Command != "" || tomlCfg.Build.Entrypoint != "") {
		// TOML 파싱 성공 → [build] command 사용 (Go 실행)
		preBuild = strings.TrimSpace(tomlCfg.Build.PreBuild)
		command = strings.TrimSpace(tomlCfg.Build.Command)
		if command == "" {
			command = "python " + tomlCfg.Build.Entrypoint + " run"
		}
		// Apply secrets_mapping
		for envVar, secretKey := range tomlCfg.SecretsMapping {
			if v, ok := input.Secrets[secretKey]; ok {
				envMap[envVar] = v
			}
		}
		for k, v := range tomlCfg.Config {
			envMap[k] = v
		}
	} else {
		// Legacy: Python 파일. .toml은 python 실행 불가.
		if IsTOMLConfig(configPath) || IsTOMLConfig(configFullPath) || strings.ToLower(filepath.Ext(configFullPath)) == ".toml" {
			return &workflow.RunResult{
				Stderr:   fmt.Sprintf("config %s is TOML but parse failed: %v; cannot run as Python", configPath, parseErr),
				ExitCode: 1,
				Success:  false,
			}, nil
		}
		command = "python " + configPath + " run"
	}

	envSlice := envMapToSlice(envMap)

	// 3. Run pre_build (if set)
	if preBuild != "" {
		preResult := runShell(ctx, tmpDir, envSlice, preBuild)
		if !preResult.Success {
			return preResult, nil
		}
	}

	// 4. Run command
	result := runShell(ctx, tmpDir, envSlice, command)
	return result, nil
}

func envFromOS() map[string]string {
	m := make(map[string]string)
	for _, e := range os.Environ() {
		idx := strings.Index(e, "=")
		if idx > 0 {
			m[e[:idx]] = e[idx+1:]
		}
	}
	return m
}

func envMapToSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func runShell(ctx context.Context, dir string, env []string, cmdStr string) *workflow.RunResult {
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return &workflow.RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Success:  exitCode == 0,
	}
}

func injectTokenIntoURL(rawURL, token string) string {
	if token == "" {
		return rawURL
	}
	if len(rawURL) > 8 && rawURL[:8] == "https://" {
		return "https://" + token + "@" + rawURL[8:]
	}
	if len(rawURL) > 7 && rawURL[:7] == "http://" {
		return "http://" + token + "@" + rawURL[7:]
	}
	return rawURL
}
