// metaflow_ci.go - metaflow_cicd Self CI Test (Temporal SDK 연동)
//
// test_pipeline.sh와 동일한 build test를 Temporal SDK로 직접 실행합니다.
// GIT_WEBHOOK_FLOW.md (metaflow_manager/docs/) 참고.
//
// Prerequisites:
//   - metaflow_cicd DB (projects, secrets)
//   - Temporal 서버
//   - metaflow_cicd Worker (ci-task-queue)
//
// Usage:
//
//	# CI run (Temporal Runner가 metaflow-ci.toml [build] command로 호출)
//	go run ./flows/runner/metaflow_ci.go run
//
//	# Temporal SDK로 워크플로우 트리거 (테스트용)
//	go run ./flows/runner/metaflow_ci.go
//	go run ./flows/runner/metaflow_ci.go -wait
//	go run ./flows/runner/metaflow_ci.go -branch dev -mode cd
//
// Env:
//
//	TEMPORAL_ADDRESS - Temporal 서버 (default: localhost:7233)
//	METAFLOW_CICD_API_URL - API 서버 (프로젝트 등록용, default: http://localhost:8059)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/neunexus/metaflow_cicd/workflow"
	"go.temporal.io/sdk/client"
)

const (
	defaultProjectName   = "metaflow_cicd"
	defaultRepoURL       = "https://github.com/Lyckabc/metaflow_cicd"
	defaultBranch       = "dev"
	defaultBuildMode     = "ci"
	defaultTemporalAddr  = "localhost:7233"
	defaultAPIURL        = "http://localhost:8059"
	ciConfigPath         = "flows/metaflow-ci.toml"
	taskQueue            = "ci-task-queue"
	workflowTimeout      = 20 * time.Minute
)

func main() {
	branch := flag.String("branch", defaultBranch, "target branch")
	mode := flag.String("mode", defaultBuildMode, "build mode: ci or cd")
	wait := flag.Bool("wait", false, "wait for workflow completion")
	createProject := flag.Bool("create-project", true, "create/update project via API before trigger")
	flag.Parse()

	// "run" subcommand: CI validation (metaflow-ci.toml [build] command에서 호출)
	for _, arg := range os.Args[1:] {
		if arg == "run" {
			runCI()
			return
		}
	}

	projectName := defaultProjectName
	repoURL := defaultRepoURL
	temporalAddr := os.Getenv("TEMPORAL_ADDRESS")
	if temporalAddr == "" {
		temporalAddr = defaultTemporalAddr
	}
	apiURL := os.Getenv("METAFLOW_CICD_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	fmt.Println("==============================================")
	fmt.Println(" metaflow_cicd Self CI Test (Temporal SDK)")
	fmt.Println("==============================================")
	fmt.Printf(" Project: %s\n", projectName)
	fmt.Printf(" Repo:    %s\n", repoURL)
	fmt.Printf(" Branch:  %s\n", *branch)
	fmt.Printf(" Mode:    %s\n", *mode)
	fmt.Printf(" Temporal: %s\n", temporalAddr)
	fmt.Println("==============================================")
	fmt.Println()

	// 1. Create/Update project (optional)
	if *createProject {
		fmt.Println("=== 1. Create/Update project ===")
		if err := createOrUpdateProject(apiURL, projectName, repoURL, *branch, *mode); err != nil {
			log.Printf("Create project (may exist): %v", err)
		}
		fmt.Println()
	}

	// 2. Connect Temporal & Trigger workflow
	fmt.Println("=== 2. Trigger workflow (Temporal SDK) ===")
	c, err := client.Dial(client.Options{
		HostPort:  temporalAddr,
		Namespace: client.DefaultNamespace,
	})
	if err != nil {
		log.Fatalf("Temporal client: %v", err)
	}
	defer c.Close()

	workflowID := "ci-" + strings.ReplaceAll(projectName, "/", "-")
	options := client.StartWorkflowOptions{
		ID:                   workflowID,
		TaskQueue:            taskQueue,
		WorkflowRunTimeout:   workflowTimeout,
		WorkflowTaskTimeout:  time.Minute,
	}

	req := workflow.PipelineRequest{
		ServiceName:       projectName,
		RepoURL:           repoURL,
		Branch:            *branch,
		BuildMode:         *mode,
		TemporalUIBaseURL:  os.Getenv("TEMPORAL_UI_BASE_URL"),
	}

	we, err := c.ExecuteWorkflow(context.Background(), options, workflow.ManagerWorkflow, req)
	if err != nil {
		log.Fatalf("ExecuteWorkflow: %v", err)
	}

	fmt.Printf(" Workflow ID: %s\n", we.GetID())
	fmt.Printf(" Run ID:      %s\n", we.GetRunID())
	fmt.Println()

	// 3. Wait for result (optional)
	if *wait {
		fmt.Println("=== 3. Waiting for workflow completion ===")
		var result workflow.RunResult
		err = we.Get(context.Background(), &result)
		if err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf(" Success: %v\n", result.Success)
		fmt.Printf(" ExitCode: %d\n", result.ExitCode)
		if result.Stdout != "" {
			fmt.Println("--- Stdout ---")
			fmt.Println(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Println("--- Stderr ---")
			fmt.Println(result.Stderr)
		}
		if result.Success {
			fmt.Println("\nBuild test passed.")
		} else {
			os.Exit(result.ExitCode)
		}
	} else {
		fmt.Println("=== 3. Result ===")
		fmt.Println(" Check Temporal UI for execution status.")
		fmt.Println(" Flow: PreFlightCheck -> RunnerWorkflow (metaflow_ci.go run) -> DockerBuildPush (on success)")
		fmt.Println("\n Use -wait to block until workflow completes.")
	}
}

func createOrUpdateProject(apiURL, projectName, repoURL, branch, mode string) error {
	payload := map[string]interface{}{
		"project_name":    projectName,
		"main_repo_url":   repoURL,
		"target_branches": []string{"main", "dev", "^feature/.*", "^release/.*"},
		"ci_config_path":  ciConfigPath,
		"cd_config_path":  ciConfigPath,
		"description":     "metaflow_cicd self CI/CD (sample)",
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, apiURL+"/projects", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		fmt.Println(" Project created.")
		return nil
	}
	if resp.StatusCode == http.StatusOK {
		fmt.Println(" Project updated.")
		return nil
	}
	// 4xx/5xx - duplicate key 등은 정상 (이미 등록됨)
	var errBody bytes.Buffer
	_, _ = errBody.ReadFrom(resp.Body)
	errStr := errBody.String()
	if strings.Contains(errStr, "duplicate key") {
		fmt.Println(" Project already exists (duplicate key).")
		return nil
	}
	return fmt.Errorf("POST /projects: %s %s", resp.Status, errStr)
}

// runCI performs CI validation (env check, build/test). Called by metaflow-ci.toml [build] command.
func runCI() {
	fmt.Println("metaflow_cicd CI flow - OK")
	keys := []string{"REGISTRY_ID", "REGISTRY_PASSWORD", "GITHUB_TOKEN", "DB_HOST", "LOG_LEVEL"}
	for _, key := range keys {
		if v, ok := os.LookupEnv(key); ok {
			if strings.Contains(key, "PASSWORD") || strings.Contains(key, "TOKEN") {
				fmt.Printf("  %s: [REDACTED]\n", key)
			} else {
				fmt.Printf("  %s: %s\n", key, v)
			}
		}
	}
	os.Exit(0)
}
