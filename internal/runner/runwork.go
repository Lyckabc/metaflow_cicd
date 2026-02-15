package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/neunexus/metaflow_cicd/workflow"
)

// RunWorkActivity clones the repo, runs the script, collects stdout/stderr, and cleans up.
func RunWorkActivity(ctx context.Context, config *workflow.PipelineConfig) (*workflow.RunResult, error) {
	tmpDir, err := os.MkdirTemp("", "cicd-"+config.ServiceName+"-*")
	if err != nil {
		return &workflow.RunResult{Stderr: err.Error(), ExitCode: 1, Success: false}, nil
	}
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	// Clone
	branch := config.Branch
	if branch == "" {
		branch = "main"
	}
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, config.RepoURL, tmpDir)
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

	// Run script (subprocess via shell for &&, |, etc.)
	runCmd := config.RunCommand
	if runCmd == "" {
		runCmd = "pip install -r requirements.txt"
	}
	runCmd = strings.TrimSpace(runCmd)
	if runCmd == "" {
		return &workflow.RunResult{Stderr: "run_command is empty", ExitCode: 1, Success: false}, nil
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", runCmd)
	cmd.Dir = tmpDir
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

