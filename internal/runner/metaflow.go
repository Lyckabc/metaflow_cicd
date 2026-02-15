package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/neunexus/metaflow_cicd/workflow"
)

// RunMetaflowActivity clones repo, verifies config, injects secrets, runs Metaflow, and cleans up.
// Git Clone: Manager에서 받은 Git URL과 Token으로 임시 디렉토리에 클론
// Config Load: config_path 파일이 존재하는지 확인
// Secret Injection: secrets를 환경 변수로 주입
// Execution: python <config_path> run 실행, 로그를 실시간 캡처
// Clean-up: 성공/실패 여부와 관계없이 임시 디렉토리 삭제
func RunMetaflowActivity(ctx context.Context, input *workflow.RunnerInput) (*workflow.RunResult, error) {
	tmpDir, err := os.MkdirTemp("", "metaflow-cicd-"+input.ProjectName+"-*")
	if err != nil {
		return &workflow.RunResult{Stderr: err.Error(), ExitCode: 1, Success: false}, nil
	}
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	// 1. Git Clone (Token 사용 시 URL에 포함)
	cloneURL := input.GitURL
	if input.AccessToken != "" {
		// https://token@github.com/org/repo 형식
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

	// 2. Config Load: config_path 파일 존재 확인
	configFullPath := filepath.Join(tmpDir, input.ConfigPath)
	if _, err := os.Stat(configFullPath); os.IsNotExist(err) {
		return &workflow.RunResult{
			Stderr:   fmt.Sprintf("config file not found: %s", input.ConfigPath),
			ExitCode: 1,
			Success:  false,
		}, nil
	}

	// 3. Secret Injection: 환경 변수로 주입
	cmd := exec.CommandContext(ctx, "python", input.ConfigPath, "run")
	cmd.Dir = tmpDir
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}
	for k, v := range input.Secrets {
		envMap[k] = v
	}
	cmd.Env = make([]string, 0, len(envMap))
	for k, v := range envMap {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// 4. Execution: python <config_path> run
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
	}, nil
}

func injectTokenIntoURL(rawURL, token string) string {
	// https://github.com/org/repo -> https://token@github.com/org/repo
	if token == "" {
		return rawURL
	}
	// Simple: insert token after scheme
	if len(rawURL) > 8 && rawURL[:8] == "https://" {
		return "https://" + token + "@" + rawURL[8:]
	}
	if len(rawURL) > 7 && rawURL[:7] == "http://" {
		return "http://" + token + "@" + rawURL[7:]
	}
	return rawURL
}
