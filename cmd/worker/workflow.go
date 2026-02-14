package main

import (
	"github.com/neunexus/metaflow_cicd/workflow"
	"go.temporal.io/sdk/worker"
)

func registerWorkflowsAndActivities(w worker.Worker) {
	w.RegisterWorkflow(workflow.CIWorkflow)
	w.RegisterActivity(DaggerBuildActivity)
}
