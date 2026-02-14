package main

import (
	"context"
	"log"

	"github.com/neunexus/metaflow_cicd/workflow"
)

// DaggerBuildActivity runs the Dagger build for the given pipeline request.
func DaggerBuildActivity(ctx context.Context, req workflow.PipelineRequest) (string, error) {
	log.Printf("DaggerBuildActivity: %s %s %s", req.ServiceName, req.RepoURL, req.Branch)
	// TODO: invoke Dagger pipeline
	return "ok", nil
}
