package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

// WebhookAdapter receives GitHub webhook payload from Convoy and delegates to Trigger.
type WebhookAdapter struct {
	trigger *Trigger
}

// NewWebhookAdapter creates a WebhookAdapter that forwards to the given Trigger.
func NewWebhookAdapter(trigger *Trigger) *WebhookAdapter {
	return &WebhookAdapter{trigger: trigger}
}

// ServeHTTP handles POST /webhooks/github - parses GitHub payload and starts workflow.
func (w *WebhookAdapter) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Convoy endpoint 검증용 (GET 시 200 반환)
		writeJSON(rw, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Convoy가 래핑한 경우: { "data": { ...github payload } } 또는 raw
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		data = payload
	}

	req := transformGitHubToTrigger(data)
	if req.RepoURL == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "no repository in payload"})
		return
	}

	workflowID, runID, err := w.trigger.StartWorkflow(r.Context(), req)
	if err != nil {
		log.Printf("WebhookAdapter: StartWorkflow error: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("Workflow started: %s (delivery: %s)", workflowID, r.Header.Get("X-GitHub-Delivery"))
	writeJSON(rw, http.StatusAccepted, map[string]string{
		"workflow_id": workflowID,
		"run_id":      runID,
	})
}

// transformGitHubToTrigger converts GitHub webhook payload to TriggerRequest.
func transformGitHubToTrigger(payload map[string]interface{}) TriggerRequest {
	repo, _ := payload["repository"].(map[string]interface{})
	if repo == nil {
		return TriggerRequest{}
	}
	fullName, _ := repo["full_name"].(string)
	cloneURL, _ := repo["clone_url"].(string)
	ref, _ := payload["ref"].(string)
	branch := "main"
	if ref != "" && len(ref) > 11 {
		branch = ref[11:] // "refs/heads/"
	}
	if fullName == "" {
		fullName = cloneURL
	}
	return TriggerRequest{
		ServiceName: fullName,
		RepoURL:     cloneURL,
		Branch:      branch,
	}
}
