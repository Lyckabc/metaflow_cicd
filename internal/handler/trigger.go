package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/neunexus/metaflow_cicd/workflow"
	"go.temporal.io/sdk/client"
)

// TriggerRequest is the API input for starting a CI workflow.
type TriggerRequest struct {
	ServiceName string `json:"service_name"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
}

// Trigger handles POST /trigger - starts Temporal CI workflow.
type Trigger struct {
	client client.Client
}

// NewTrigger creates a Trigger handler.
func NewTrigger(c client.Client) *Trigger {
	return &Trigger{client: c}
}

// ServeHTTP handles the trigger request.
func (t *Trigger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if body.RepoURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repo_url required"})
		return
	}
	if body.ServiceName == "" {
		body.ServiceName = body.RepoURL
	}
	if body.Branch == "" {
		body.Branch = "main"
	}

	workflowID, runID, err := t.StartWorkflow(r.Context(), body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"workflow_id": workflowID,
		"run_id":      runID,
	})
}

// StartWorkflow starts the CI workflow and returns workflow ID and run ID.
func (t *Trigger) StartWorkflow(ctx context.Context, req TriggerRequest) (workflowID, runID string, err error) {
	workflowID = "CI-" + strings.ReplaceAll(req.ServiceName, "/", "-")
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "ci-task-queue",
	}
	args := workflow.PipelineRequest{
		ServiceName: req.ServiceName,
		RepoURL:     req.RepoURL,
		Branch:      req.Branch,
	}

	we, err := t.client.ExecuteWorkflow(ctx, options, workflow.CIWorkflow, args)
	if err != nil {
		return "", "", err
	}
	return we.GetID(), we.GetRunID(), nil
}
