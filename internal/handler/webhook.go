package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var errSkipEvent = errors.New("skip: event not handled")

// WebhookAdapter receives GitHub webhook payload from Convoy and delegates to Trigger.
type WebhookAdapter struct {
	trigger *Trigger
	secret  string // Convoy endpoint secret for X-Convoy-Signature verification
}

// NewWebhookAdapter creates a WebhookAdapter that forwards to the given Trigger.
// If secret is non-empty, verifies X-Convoy-Signature (HMAC-SHA256 hex or advanced format).
func NewWebhookAdapter(trigger *Trigger, secret string) *WebhookAdapter {
	return &WebhookAdapter{trigger: trigger, secret: secret}
}

// ServeHTTP handles POST /webhooks/github - parses GitHub payload and starts workflow.
func (w *WebhookAdapter) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(rw, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("WebhookAdapter: read body: %v", err)
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	if w.secret != "" {
		sig := r.Header.Get("X-Convoy-Signature")
		if sig == "" {
			log.Printf("WebhookAdapter: missing X-Convoy-Signature (secret configured)")
			writeJSON(rw, http.StatusUnauthorized, map[string]string{"error": "missing signature"})
			return
		}
		if !verifyConvoySignature(body, sig, w.secret) {
			log.Printf("WebhookAdapter: invalid X-Convoy-Signature")
			writeJSON(rw, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
			return
		}
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		// Convoy may send GitHub payload as string in "payload" field
		if ps, ok := payload["payload"].(string); ok && ps != "" {
			var inner map[string]interface{}
			if json.Unmarshal([]byte(ps), &inner) == nil {
				data = inner
			}
		}
		if data == nil {
			data = payload
		}
	}

	// Event type: GitHub sends X-GitHub-Event; Convoy may wrap in payload and omit header
	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		if ev, _ := payload["event"].(map[string]interface{}); ev != nil {
			event, _ = ev["event_type"].(string)
		}
		if event == "" {
			event, _ = payload["event_type"].(string)
		}
		// Normalize: "github.pull_request" -> "pull_request"
		if strings.HasPrefix(event, "github.") {
			event = strings.TrimPrefix(event, "github.")
		}
		// Fallback: infer from payload structure (Convoy may omit event_type)
		if event == "" && data != nil {
			if _, hasPR := data["pull_request"]; hasPR {
				event = "pull_request"
			} else if _, hasRef := data["ref"]; hasRef {
				event = "push"
			}
		}
	}
	req, err := transformGitHubToTrigger(event, data)
	if err != nil {
		if errors.Is(err, errSkipEvent) {
			rw.WriteHeader(http.StatusOK)
			return
		}
		log.Printf("WebhookAdapter: transform error: %v (event=%q)", err, event)
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.RepoURL == "" {
		log.Printf("WebhookAdapter: no repository (event=%q, has_data=%v)", event, data != nil)
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "no repository in payload"})
		return
	}

	workflowID, runID, err := w.trigger.StartWorkflow(r.Context(), req)
	if err != nil {
		log.Printf("WebhookAdapter: StartWorkflow error: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("Workflow started: %s (event=%s, delivery=%s)", workflowID, event, r.Header.Get("X-GitHub-Delivery"))
	writeJSON(rw, http.StatusAccepted, map[string]string{
		"workflow_id": workflowID,
		"run_id":      runID,
	})
}

// parseOwnerRepo splits "owner/repo" into owner and repo.
func parseOwnerRepo(fullName string) (owner, repo string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 && parts[0] != "" {
		return "", parts[0]
	}
	return "", ""
}

// transformGitHubToTrigger converts GitHub webhook payload to TriggerRequest.
// - pull_request opened → CI (head ref)
// - pull_request closed + merged → CD (base/target branch)
// - push (to main) → CD (optional, for direct pushes)
func transformGitHubToTrigger(event string, payload map[string]interface{}) (TriggerRequest, error) {
	repo, _ := payload["repository"].(map[string]interface{})
	if repo == nil {
		return TriggerRequest{}, errSkipEvent
	}
	fullName, _ := repo["full_name"].(string)
	cloneURL, _ := repo["clone_url"].(string)
	name, _ := repo["name"].(string)
	if fullName == "" {
		fullName = cloneURL
	}
	githubOwner, githubRepo := parseOwnerRepo(fullName)
	// service_name: ci_projects uses short name (e.g. Nucleus) or full_name
	serviceName := name
	if serviceName == "" {
		if githubRepo != "" {
			serviceName = githubRepo
		} else {
			parts := strings.Split(fullName, "/")
			if len(parts) > 0 {
				serviceName = parts[len(parts)-1]
			}
		}
	}

	switch event {
	case "pull_request":
		pr, _ := payload["pull_request"].(map[string]interface{})
		if pr == nil {
			return TriggerRequest{}, errSkipEvent
		}
		action, _ := pr["action"].(string)
		merged, _ := pr["merged"].(bool)
		head, _ := pr["head"].(map[string]interface{})
		base, _ := pr["base"].(map[string]interface{})

		// SHA for status API: head.sha (PR branch commit)
		sha, _ := head["sha"].(string)
		if sha == "" {
			sha, _ = pr["merge_commit_sha"].(string)
		}

		if action == "opened" || action == "synchronize" {
			// CI: PR head
			headRef, _ := head["ref"].(string)
			headRepo, _ := head["repo"].(map[string]interface{})
			headCloneURL := cloneURL
			if headRepo != nil {
				if u, ok := headRepo["clone_url"].(string); ok {
					headCloneURL = u
				}
			}
			return TriggerRequest{
				ServiceName: serviceName,
				RepoURL:     headCloneURL,
				Branch:      headRef,
				BuildMode:   "ci",
				GitHubOwner: githubOwner,
				GitHubRepo:  githubRepo,
				GitHubSHA:   sha,
			}, nil
		}
		if action == "closed" && merged {
			// CD: target branch
			baseRef, _ := base["ref"].(string)
			if baseRef == "" {
				baseRef = "main"
			}
			return TriggerRequest{
				ServiceName: serviceName,
				RepoURL:     cloneURL,
				Branch:      baseRef,
				BuildMode:   "cd",
				GitHubOwner: githubOwner,
				GitHubRepo:  githubRepo,
				GitHubSHA:   sha,
			}, nil
		}
		return TriggerRequest{}, errSkipEvent // ignore other PR actions

	case "push":
		ref, _ := payload["ref"].(string)
		branch := "main"
		if ref != "" && strings.HasPrefix(ref, "refs/heads/") {
			branch = ref[len("refs/heads/"):]
		}
		// SHA: "after" (commit SHA) or head_commit.id
		sha, _ := payload["after"].(string)
		if sha == "" {
			if hc, ok := payload["head_commit"].(map[string]interface{}); ok {
				sha, _ = hc["id"].(string)
			}
		}
		return TriggerRequest{
			ServiceName: serviceName,
			RepoURL:     cloneURL,
			Branch:      branch,
			BuildMode:   "cd",
			GitHubOwner: githubOwner,
			GitHubRepo:  githubRepo,
			GitHubSHA:   sha,
		}, nil
	}

	return TriggerRequest{}, nil
}

// verifyConvoySignature validates X-Convoy-Signature (HMAC-SHA256).
// Supports simple (hex) and advanced (t=timestamp,v1=hash) formats.
func verifyConvoySignature(body []byte, sigHeader, secret string) bool {
	parts := strings.Split(sigHeader, ",")
	var toVerify []string
	var payloadForHMAC []byte = body

	if len(parts) > 1 {
		// Advanced: t=1492774577,v1=hash,v0=hash
		var timestamp int64
		for _, p := range parts {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch kv[0] {
			case "t":
				timestamp, _ = strconv.ParseInt(kv[1], 10, 64)
			case "v0", "v1":
				toVerify = append(toVerify, kv[1])
			}
		}
		if timestamp > 0 && len(toVerify) > 0 {
			// Replay protection: reject if older than 1 hour
			if time.Since(time.Unix(timestamp, 0)) > time.Hour {
				return false
			}
			// Payload for HMAC: timestamp + "," + body
			payloadForHMAC = []byte(strconv.FormatInt(timestamp, 10) + "," + string(body))
		}
	} else {
		// Simple: single hex hash
		toVerify = []string{strings.TrimSpace(sigHeader)}
	}

	if len(toVerify) == 0 {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadForHMAC)
	sum := mac.Sum(nil)
	expectedHex := hex.EncodeToString(sum)
	expectedB64 := base64.StdEncoding.EncodeToString(sum)

	for _, v := range toVerify {
		v = strings.TrimSpace(v)
		if v == expectedHex || v == expectedB64 {
			return true
		}
	}
	return false
}
