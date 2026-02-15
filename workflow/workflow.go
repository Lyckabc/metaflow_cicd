package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// PipelineRequest is the input for workflows.
type PipelineRequest struct {
	ServiceName string
	RepoURL     string
	Branch      string
	BuildMode   string // "ci" (PR opened) or "cd" (PR merged)

	// GitHub metadata for commit status updates (Branch Protection)
	GitHubOwner       string // e.g. "my-org"
	GitHubRepo        string // e.g. "my-repo"
	GitHubSHA         string // Commit SHA for status API
	TemporalUIBaseURL string // e.g. https://temporal.toji.homes (for target_url)
}

// GitHubStatusInput is passed to UpdateGitHubStatusActivity.
type GitHubStatusInput struct {
	Owner       string
	Repo        string
	Ref         string
	State       string // "pending", "success", "error", "failure"
	Description string
	Context     string // e.g. "temporal/ci", "temporal/cd"
	TargetURL   string
}

// CIWorkflow runs the CI pipeline (build via Dagger).
func CIWorkflow(ctx workflow.Context, req PipelineRequest) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var result string
	err := workflow.ExecuteActivity(ctx, "DaggerBuildActivity", req).Get(ctx, &result)
	return result, err
}
