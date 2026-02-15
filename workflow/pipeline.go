package workflow

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"
)

// PipelineConfig is the output of GetConfig stage (from DB).
type PipelineConfig struct {
	ServiceName      string
	RepoURL          string
	Branch           string
	RegistryURL      string
	RunCommand       string
	RegistryID       string
	RegistryPassword string
	BuildMode        string // "ci" or "cd"
}

// RunnerInput is the output of Manager (Pre-flight) and input for Runner Workflow.
// Contains Git URL, token, config path, and secrets for Metaflow execution.
type RunnerInput struct {
	ProjectName string
	GitURL      string
	AccessToken string // Git PAT for clone
	Branch      string
	ConfigPath  string // e.g. "metaflow_ci.py" or "flow.py"
	Secrets     map[string]string
	BuildMode   string // "ci" or "cd"

	// GitHub metadata for commit status updates
	GitHubOwner       string
	GitHubRepo        string
	GitHubSHA         string
	TemporalUIBaseURL string
}

// RunResult is the output of RunWork stage.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Success  bool
}

// DockerBuildResult is the output of DockerBuildPush stage.
type DockerBuildResult struct {
	ImageRef string // e.g. registry.example.com/Nucleus:2502151430
	Stdout   string
	Stderr   string
	ExitCode int
	Success  bool
}

// buildTemporalUIURL returns the workflow run URL for GitHub status target_url.
func buildTemporalUIURL(baseURL, namespace, workflowID, runID string) string {
	base := strings.TrimSuffix(baseURL, "/")
	return fmt.Sprintf("%s/namespaces/%s/workflows/%s/%s", base, namespace, workflowID, runID)
}

// updateGitHubStatusIfSet sends status to GitHub when req has GitHub metadata.
func updateGitHubStatusIfSet(ctx workflow.Context, req PipelineRequest, state, description string) {
	if req.GitHubOwner == "" || req.GitHubRepo == "" || req.GitHubSHA == "" || req.TemporalUIBaseURL == "" {
		return
	}
	info := workflow.GetInfo(ctx)
	targetURL := buildTemporalUIURL(req.TemporalUIBaseURL, info.Namespace, info.WorkflowExecution.ID, info.WorkflowExecution.RunID)
	ctxShort := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second})
	_ = workflow.ExecuteActivity(ctxShort, "UpdateGitHubStatusActivity", GitHubStatusInput{
		Owner:       req.GitHubOwner,
		Repo:        req.GitHubRepo,
		Ref:         req.GitHubSHA,
		State:       state,
		Description: description,
		Context:     "temporal/" + req.BuildMode,
		TargetURL:   targetURL,
	}).Get(ctx, nil)
}

// DynamicRunnerWorkflow runs the pipeline: GetConfig → RunWork (CI test) or DockerBuildPush (build+push).
func DynamicRunnerWorkflow(ctx workflow.Context, req PipelineRequest) (*RunResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// 1. Start: set GitHub status to pending
	updateGitHubStatusIfSet(ctx, req, "pending", "Temporal CI/CD is running...")

	var config PipelineConfig
	err := workflow.ExecuteActivity(ctx, "GetConfigActivity", req).Get(ctx, &config)
	if err != nil {
		updateGitHubStatusIfSet(ctx, req, "error", "Config failed")
		return nil, err
	}
	if config.RepoURL == "" {
		updateGitHubStatusIfSet(ctx, req, "failure", "repo_url not found")
		return &RunResult{Stderr: "repo_url not found in DB and no fallback", ExitCode: 1, Success: false}, nil
	}

	// CI: run_command (test) + Docker build + push
	// CD: Docker build + push only
	if config.BuildMode == "ci" {
		var testResult RunResult
		err = workflow.ExecuteActivity(ctx, "RunWorkActivity", &config).Get(ctx, &testResult)
		if err != nil {
			updateGitHubStatusIfSet(ctx, req, "error", "CI test failed")
			return nil, err
		}
		if !testResult.Success {
			updateGitHubStatusIfSet(ctx, req, "failure", "CI test failed")
			return &testResult, nil
		}
	}

	// CI & CD: Docker build + push
	var dockerResult DockerBuildResult
	err = workflow.ExecuteActivity(ctx, "DockerBuildPushActivity", &config).Get(ctx, &dockerResult)
	if err != nil {
		updateGitHubStatusIfSet(ctx, req, "error", "Build failed")
		return nil, err
	}
	if dockerResult.Success {
		updateGitHubStatusIfSet(ctx, req, "success", "CI/CD passed")
	} else {
		updateGitHubStatusIfSet(ctx, req, "failure", "Build failed")
	}
	return &RunResult{
		Stdout:   dockerResult.Stdout,
		Stderr:   dockerResult.Stderr,
		ExitCode: dockerResult.ExitCode,
		Success:  dockerResult.Success,
	}, nil
}
