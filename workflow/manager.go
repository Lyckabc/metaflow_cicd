package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// ManagerWorkflow performs pre-flight check and routes to Runner Workflow.
// 1. PreFlightCheckActivity: DB lookup, branch match, source resolution
// 2. RunnerWorkflow: Metaflow execution (clone, run, cleanup)
func ManagerWorkflow(ctx workflow.Context, req PipelineRequest) (*RunResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// 1. Start: set GitHub status to pending
	updateGitHubStatusIfSet(ctx, req, "pending", "Temporal CI/CD pre-flight...")

	// 2. Pre-flight check
	var runnerInput *RunnerInput
	err := workflow.ExecuteActivity(ctx, "PreFlightCheckActivity", req).Get(ctx, &runnerInput)
	if err != nil {
		updateGitHubStatusIfSet(ctx, req, "error", "Pre-flight failed")
		return nil, err
	}

	// 3. Execute Runner Workflow (child)
	runnerCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 15 * time.Minute,
	})
	var result RunResult
	err = workflow.ExecuteChildWorkflow(runnerCtx, RunnerWorkflow, runnerInput).Get(ctx, &result)
	if err != nil {
		updateGitHubStatusIfSet(ctx, req, "error", "Runner failed")
		return nil, err
	}
	if result.Success {
		updateGitHubStatusIfSet(ctx, req, "success", "CI/CD passed")
	} else {
		updateGitHubStatusIfSet(ctx, req, "failure", "CI/CD failed")
	}
	return &result, nil
}
