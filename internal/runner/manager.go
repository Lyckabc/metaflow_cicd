package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/neunexus/metaflow_cicd/internal/repository"
	"github.com/neunexus/metaflow_cicd/workflow"
)

// PreFlightCheckActivity performs Manager pre-flight: DB lookup, branch match, source resolution.
// Returns RunnerInput when all checks pass; otherwise returns error.
//
// Pre-flight checks:
// 1. projects.main_repo_url matches webhook repo URL
// 2. branch matches projects.target_branches (regex/wildcard)
// 3. Source resolution: ci_source_name host or main_repo_url
//
// Security: Convoy Webhook Secret을 사용한 Signature Verification 로직은
// metaflow_manager/internal/handler/webhook.go의 WebhookAdapter에서 X-Convoy-Signature 검증으로 처리됨.
// sources.webhook_secret을 project별로 사용하려면 여기서 추가 검증 로직을 넣을 수 있음.
func PreFlightCheckActivity(ctx context.Context, req workflow.PipelineRequest) (*workflow.RunnerInput, error) {
	db := connectMetaflowDB()
	repo, err := repository.New(db)
	if err != nil {
		return nil, fmt.Errorf("repository init: %w", err)
	}

	// 1. Pre-flight: main_repo_url 존재 확인
	// Webhook의 clone_url이 projects.main_repo_url에 존재하는지 확인 (정규화 후 매칭)
	project, err := repo.GetProjectByMainRepoURL(ctx, req.RepoURL)
	if err != nil && err != repository.ErrNotFound {
		return nil, fmt.Errorf("get project: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project not found for main_repo_url=%q", req.RepoURL)
	}

	// 2. Pre-flight: branch가 target_branches 패턴과 일치하는지 검사
	if !repository.BranchMatchesTargetBranches(req.Branch, project.TargetBranches) {
		return nil, fmt.Errorf("branch %q does not match target_branches %v", req.Branch, project.TargetBranches)
	}

	// 3. Source 참조 로직: CI 실행 시 source의 host(Git URL) 확인
	// host가 있으면 해당 URL 사용, 없으면 main_repo_url 사용
	gitURL := project.MainRepoURL
	sourceName := project.CISourceName
	if req.BuildMode == "cd" {
		sourceName = project.CDSourceName
	}
	if sourceName != nil && *sourceName != "" {
		source, err := repo.GetSourceByName(ctx, *sourceName)
		if err == nil && source.Host != nil && strings.TrimSpace(*source.Host) != "" {
			gitURL = strings.TrimSpace(*source.Host)
		}
		// source가 없거나 host가 비어있으면 main_repo_url 유지
	}

	// Access token: source에 있으면 사용
	var accessToken string
	if sourceName != nil && *sourceName != "" {
		if source, err := repo.GetSourceByName(ctx, *sourceName); err == nil && source.AccessToken != nil {
			accessToken = strings.TrimSpace(*source.AccessToken)
		}
	}

	// 4. Config path: CI/CD 모드에 따라
	configPath := project.CIConfigPath
	if req.BuildMode == "cd" {
		configPath = project.CDConfigPath
	}

	// 5. Secrets: project_id로 조회 (scope=prod 기본)
	scope := "prod"
	secrets, err := repo.GetSecretsByProjectID(ctx, int(project.ID), scope)
	if err != nil {
		return nil, fmt.Errorf("get secrets: %w", err)
	}
	secretsMap := make(map[string]string)
	for _, s := range secrets {
		secretsMap[s.SecretKey] = s.SecretValue
	}

	// 6. SecretsMapping: metaflow-ci.toml [secrets_mapping] from cloned repo
	var secretsMapping map[string]string
	if cfg, err := FetchAndParseConfig(ctx, gitURL, req.Branch, accessToken, configPath); err == nil && cfg != nil && cfg.SecretsMapping != nil {
		secretsMapping = cfg.SecretsMapping
	}

	return &workflow.RunnerInput{
		ProjectName:       project.ProjectName,
		GitURL:            gitURL,
		AccessToken:       accessToken,
		Branch:            req.Branch,
		ConfigPath:        configPath,
		Secrets:           secretsMap,
		SecretsMapping:    secretsMapping,
		BuildMode:         req.BuildMode,
		GitHubOwner:       req.GitHubOwner,
		GitHubRepo:        req.GitHubRepo,
		GitHubSHA:         req.GitHubSHA,
		TemporalUIBaseURL: req.TemporalUIBaseURL,
	}, nil
}
