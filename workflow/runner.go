package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// RunnerWorkflow runs Metaflow execution: clone, config verify, secret injection, run, cleanup.
// Receives RunnerInput from Manager (Pre-flight) workflow.
func RunnerWorkflow(ctx workflow.Context, input *RunnerInput) (*RunResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result RunResult
	err := workflow.ExecuteActivity(ctx, "RunMetaflowActivity", input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
