package runner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/neunexus/metaflow_cicd/internal/repository"
	"github.com/neunexus/metaflow_cicd/workflow"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// GetConfigActivity queries DB by main_repo_url or project_name. Used by DynamicRunnerWorkflow (legacy).
func GetConfigActivity(ctx context.Context, req workflow.PipelineRequest) (*workflow.PipelineConfig, error) {
	db := connectMetaflowDB()
	repo, err := repository.New(db)
	if err != nil {
		return nil, fmt.Errorf("repository init: %w", err)
	}

	var project *repository.Project
	project, err = repo.GetProjectByMainRepoURL(ctx, req.RepoURL)
	if err == repository.ErrNotFound {
		project, err = repo.GetProjectByProjectName(ctx, req.ServiceName)
	}
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}

	svcName := req.ServiceName
	repoURL := req.RepoURL
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	registryURL := ""
	runCmd := "pip install -r requirements.txt"
	registryID := ""
	registryPassword := ""

	if project != nil {
		svcName = project.ProjectName
		repoURL = project.MainRepoURL
		if len(project.TargetBranches) > 0 {
			branch = project.TargetBranches[0]
		}
		configPath := project.CIConfigPath
		if req.BuildMode == "cd" {
			configPath = project.CDConfigPath
		}
		// TOML config: fetch from repo and use [build] command (metaflow-ci.toml)
		if IsTOMLConfig(configPath) {
			gitURL := repoURL
			accessToken := ""
			sn := project.CISourceName
			if req.BuildMode == "cd" {
				sn = project.CDSourceName
			}
			if sn != nil && *sn != "" {
				if src, err := repo.GetSourceByName(ctx, *sn); err == nil {
					if src.Host != nil && strings.TrimSpace(*src.Host) != "" {
						gitURL = strings.TrimSpace(*src.Host)
					}
					if src.AccessToken != nil {
						accessToken = strings.TrimSpace(*src.AccessToken)
					}
				}
			}
			if req.Branch != "" {
				branch = req.Branch
			}
			if cfg, err := FetchAndParseConfig(ctx, gitURL, branch, accessToken, configPath); err == nil && cfg != nil {
				preBuild := strings.TrimSpace(cfg.Build.PreBuild)
				cmd := strings.TrimSpace(cfg.Build.Command)
				if cmd == "" {
					cmd = "python " + cfg.Build.Entrypoint + " run"
				}
				if preBuild != "" {
					runCmd = preBuild + " && " + cmd
				} else {
					runCmd = cmd
				}
			} else {
				// TOML 파일은 python으로 실행 불가 - fallback "python <path> run" 사용 시 NameError 발생
				if err != nil {
					return nil, fmt.Errorf("failed to fetch/parse metaflow-ci.toml (config=%s): %w", configPath, err)
				}
				return nil, fmt.Errorf("failed to fetch/parse metaflow-ci.toml (config=%s): config is nil", configPath)
			}
		} else {
			runCmd = "python " + configPath + " run"
		}
		registryID, _ = repo.GetSecretValue(ctx, int(project.ID), "REGISTRY_ID", "prod")
		registryPassword, _ = repo.GetSecretValue(ctx, int(project.ID), "REGISTRY_PASSWORD", "prod")
		registryURL, _ = repo.GetSecretValue(ctx, int(project.ID), "REGISTRY_URL", "prod")
	}
	if req.Branch != "" {
		branch = req.Branch
	}
	if req.RepoURL != "" {
		repoURL = req.RepoURL
	}
	if repoURL == "" {
		return nil, fmt.Errorf("repo_url not found (add project to DB or provide in request)")
	}
	if svcName == "" {
		svcName = req.ServiceName
	}

	buildMode := req.BuildMode
	if buildMode == "" {
		buildMode = "ci"
	}

	return &workflow.PipelineConfig{
		ServiceName:      svcName,
		RepoURL:          repoURL,
		Branch:           branch,
		RegistryURL:      registryURL,
		RunCommand:       runCmd,
		RegistryID:       registryID,
		RegistryPassword: registryPassword,
		BuildMode:        buildMode,
	}, nil
}

func connectMetaflowDB() *gorm.DB {
	host := os.Getenv("METAFLOW_CICD_DB_HOST")
	port := os.Getenv("METAFLOW_CICD_DB_PORT")
	user := os.Getenv("METAFLOW_CICD_DB_USER")
	password := os.Getenv("METAFLOW_CICD_DB_PASSWORD")
	dbname := os.Getenv("METAFLOW_CICD_DB_NAME")
	sslmode := os.Getenv("METAFLOW_CICD_DB_SSLMODE")
	if port == "" {
		port = "5432"
	}
	if sslmode == "" {
		sslmode = "disable"
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("metaflow_cicd DB connect failed: " + err.Error())
	}
	return db
}
