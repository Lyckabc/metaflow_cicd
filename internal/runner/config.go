package runner

import (
	"context"
	"fmt"
	"os"

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
		runCmd = "python " + project.CIConfigPath + " run"
		if req.BuildMode == "cd" {
			runCmd = "python " + project.CDConfigPath + " run"
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
