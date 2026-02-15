package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v53/github"
	"github.com/neunexus/metaflow_cicd/workflow"
	"golang.org/x/oauth2"
)

// UpdateGitHubStatusActivity updates GitHub commit status via Status API.
// Requires GITHUB_TOKEN env var (PAT or GitHub App token).
func UpdateGitHubStatusActivity(ctx context.Context, input workflow.GitHubStatusInput) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set; cannot update GitHub status")
	}
	if input.Owner == "" || input.Repo == "" || input.Ref == "" {
		return fmt.Errorf("owner, repo, and ref required for GitHub status")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	status := &github.RepoStatus{
		State:       github.String(input.State),
		TargetURL:   github.String(input.TargetURL),
		Description: github.String(input.Description),
		Context:     github.String(input.Context),
	}

	_, _, err := client.Repositories.CreateStatus(ctx, input.Owner, input.Repo, input.Ref, status)
	if err != nil {
		return fmt.Errorf("failed to update github status: %w", err)
	}
	return nil
}
