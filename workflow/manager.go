package workflow

import (
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"
)

// ManagerWorkflow performs pre-flight check and routes to Runner Workflow.
// 1. PreFlightCheckActivity: DB lookup, branch match, source resolution
// 2. RunnerWorkflow: Metaflow execution (clone, run, cleanup)
// 3. On CI success: Docker build & push to registry (tag: service_name:YYMMDDHHmm)
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
	if !result.Success {
		updateGitHubStatusIfSet(ctx, req, "failure", "CI/CD failed")
		return &result, nil
	}

	// 4. CI success: build & push to registry
	registryURL, registryID, registryPassword := getRegistryFromSecrets(runnerInput.Secrets, runnerInput.SecretsMapping)
	if registryURL != "" && registryID != "" && registryPassword != "" {
		buildCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 15 * time.Minute,
		})
		dockerConfig := &PipelineConfig{
			ServiceName:      runnerInput.ProjectName,
			RepoURL:          runnerInput.GitURL,
			Branch:           runnerInput.Branch,
			RegistryURL:      registryURL,
			RegistryID:       registryID,
			RegistryPassword: registryPassword,
			AccessToken:      runnerInput.AccessToken,
		}
		var dockerResult DockerBuildResult
		err = workflow.ExecuteActivity(buildCtx, "DockerBuildPushActivity", dockerConfig).Get(ctx, &dockerResult)
		if err != nil {
			updateGitHubStatusIfSet(ctx, req, "error", "Build & push failed")
			return nil, err
		}
		if dockerResult.Success {
			updateGitHubStatusIfSet(ctx, req, "success", "CI/CD passed")
			return &RunResult{
				Stdout:   result.Stdout + "\n" + dockerResult.Stdout,
				Stderr:   result.Stderr + dockerResult.Stderr,
				ExitCode: 0,
				Success:  true,
			}, nil
		}
		updateGitHubStatusIfSet(ctx, req, "failure", "Build & push failed")
		return &RunResult{
			Stdout:   result.Stdout + "\n" + dockerResult.Stdout,
			Stderr:   result.Stderr + dockerResult.Stderr,
			ExitCode: dockerResult.ExitCode,
			Success:  false,
		}, nil
	}

	updateGitHubStatusIfSet(ctx, req, "success", "CI/CD passed")
	return &result, nil
}

// Registry env var names from metaflow-ci.toml [secrets_mapping].
const (
	registryURLKey      = "REGISTRY_URL"
	registryIDKey       = "REGISTRY_ID"
	registryPasswordKey = "REGISTRY_PASSWORD"
)

// getRegistryFromSecrets extracts registry URL, ID, password from secrets.
// If secretsMapping is provided (from metaflow-ci.toml [secrets_mapping]), uses it to resolve
// REGISTRY_URL, REGISTRY_ID, REGISTRY_PASSWORD -> secret_key -> secrets[secret_key].
// Otherwise falls back to common key patterns: REGISTRY_*, TOJI_REGISTRY_*.
func getRegistryFromSecrets(secrets map[string]string, secretsMapping map[string]string) (url, id, password string) {
	if secretsMapping != nil {
		if sk := secretsMapping[registryURLKey]; sk != "" {
			if u := strings.TrimSpace(secrets[sk]); u != "" {
				if ik := secretsMapping[registryIDKey]; ik != "" {
					if i := strings.TrimSpace(secrets[ik]); i != "" {
						if pk := secretsMapping[registryPasswordKey]; pk != "" {
							if p := secrets[pk]; p != "" {
								return u, i, p
							}
						}
					}
				}
			}
		}
	}
	// Fallback: try common key patterns
	keys := []struct{ url, id, pwd string }{
		{"REGISTRY_URL", "REGISTRY_ID", "REGISTRY_PASSWORD"},
		{"TOJI_REGISTRY_URL", "TOJI_REGISTRY_ID", "TOJI_REGISTRY_PASSWORD"},
	}
	for _, k := range keys {
		if u := strings.TrimSpace(secrets[k.url]); u != "" {
			if i := strings.TrimSpace(secrets[k.id]); i != "" {
				if p := secrets[k.pwd]; p != "" {
					return u, i, p
				}
			}
		}
	}
	return "", "", ""
}
