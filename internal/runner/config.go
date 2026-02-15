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

// GetConfigActivity queries DB by service_name. If not found, uses fallback from request (webhook).
func GetConfigActivity(ctx context.Context, req workflow.PipelineRequest) (*workflow.PipelineConfig, error) {
	db := connectMetaflowDB()
	repo, err := repository.New(db)
	if err != nil {
		return nil, fmt.Errorf("repository init: %w", err)
	}

	project, err := repo.GetProjectByServiceName(ctx, req.ServiceName)
	if err == repository.ErrNotFound && strings.Contains(req.ServiceName, "/") {
		// Try repo name only (e.g. Nucleus from Lyckabc/Nucleus)
		parts := strings.Split(req.ServiceName, "/")
		project, err = repo.GetProjectByServiceName(ctx, parts[len(parts)-1])
	}
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}

	var svcName, repoURL, branch, registryURL, runCmd string
	if project != nil {
		svcName = project.ServiceName
		repoURL = project.RepoURL
		branch = project.Branch
		registryURL = project.RegistryURL
		runCmd = project.RunCommand
	} else {
		svcName = req.ServiceName
		repoURL = req.RepoURL
		branch = req.Branch
		if branch == "" {
			branch = "main"
		}
	}
	if repoURL == "" {
		return nil, fmt.Errorf("service_name %q not in ci_projects and no repo_url fallback", req.ServiceName)
	}
	if svcName == "" {
		svcName = req.ServiceName
	}
	if branch == "" {
		branch = req.Branch
		if branch == "" {
			branch = "main"
		}
	}
	if runCmd == "" {
		runCmd = "pip install -r requirements.txt"
	}

	// Override branch from request (webhook provides head/base ref)
	if req.Branch != "" {
		branch = req.Branch
	}
	if req.RepoURL != "" {
		repoURL = req.RepoURL
	}

	// Registry credentials from ci_secrets
	registryID, _ := repo.GetSecretValue(ctx, "REGISTRY_ID")
	registryPassword, _ := repo.GetSecretValue(ctx, "REGISTRY_PASSWORD")

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
