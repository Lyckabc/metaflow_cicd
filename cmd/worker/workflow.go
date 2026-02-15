package main

import (
	"github.com/neunexus/metaflow_cicd/internal/runner"
	"github.com/neunexus/metaflow_cicd/workflow"
	"go.temporal.io/sdk/worker"
)

func registerWorkflowsAndActivities(w worker.Worker) {
	w.RegisterWorkflow(workflow.CIWorkflow)
	w.RegisterWorkflow(workflow.ManagerWorkflow)
	w.RegisterWorkflow(workflow.RunnerWorkflow)
	w.RegisterWorkflow(workflow.DynamicRunnerWorkflow)
	w.RegisterActivity(DaggerBuildActivity)
	w.RegisterActivity(runner.PreFlightCheckActivity)
	w.RegisterActivity(runner.RunMetaflowActivity)
	w.RegisterActivity(runner.GetConfigActivity)
	w.RegisterActivity(runner.RunWorkActivity)
	w.RegisterActivity(runner.DockerBuildPushActivity)
	w.RegisterActivity(runner.UpdateGitHubStatusActivity)
}
