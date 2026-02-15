package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/neunexus/metaflow_cicd/workflow"
	"go.temporal.io/sdk/client"
)

// TriggerRequest is the API input for starting a CI/CD workflow.
type TriggerRequest struct {
	ServiceName string `json:"service_name"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	BuildMode   string `json:"build_mode"` // "ci" or "cd", from webhook

	// GitHub metadata for commit status updates (from webhook payload)
	GitHubOwner string `json:"github_owner"`
	GitHubRepo  string `json:"github_repo"`
	GitHubSHA   string `json:"github_sha"`

	// TemporalUIBaseURL for target_url in GitHub status (e.g. https://temporal.toji.homes)
	TemporalUIBaseURL string `json:"temporal_ui_base_url"`
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
	if body.ServiceName == "" && body.RepoURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service_name or repo_url required"})
		return
	}
	if body.ServiceName == "" {
		body.ServiceName = body.RepoURL
	}
	if body.RepoURL == "" {
		// DB lookup will provide repo_url (GetConfig stage)
	}
	if body.Branch == "" {
		body.Branch = "main"
	}
	if body.BuildMode == "" {
		body.BuildMode = "ci"
	}
	// GitHub status: pass through from webhook; trigger may add TemporalUIBaseURL from env
	if body.TemporalUIBaseURL == "" {
		body.TemporalUIBaseURL = os.Getenv("TEMPORAL_UI_BASE_URL")
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
	prefix := req.BuildMode
	if prefix == "" {
		prefix = "ci"
	}
	workflowID = fmt.Sprintf("%s-%s-%d", prefix, strings.ReplaceAll(req.ServiceName, "/", "-"), time.Now().Unix())
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "ci-task-queue",
	}
	args := workflow.PipelineRequest{
		ServiceName:       req.ServiceName,
		RepoURL:           req.RepoURL,
		Branch:            req.Branch,
		BuildMode:         req.BuildMode,
		GitHubOwner:       req.GitHubOwner,
		GitHubRepo:        req.GitHubRepo,
		GitHubSHA:         req.GitHubSHA,
		TemporalUIBaseURL: req.TemporalUIBaseURL,
	}

	we, err := t.client.ExecuteWorkflow(ctx, options, workflow.DynamicRunnerWorkflow, args)
	if err != nil {
		return "", "", err
	}
	return we.GetID(), we.GetRunID(), nil
}
