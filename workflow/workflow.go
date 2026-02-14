package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// PipelineRequest is the input for CIWorkflow.
type PipelineRequest struct {
	ServiceName string
	RepoURL     string
	Branch      string
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
