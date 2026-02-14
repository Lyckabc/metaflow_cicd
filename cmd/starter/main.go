package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/neunexus/metaflow_cicd/internal/repository"
	"github.com/neunexus/metaflow_cicd/workflow"
	"go.temporal.io/sdk/client"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func connectDB() *gorm.DB {
	dsn := os.Getenv("METAFLOW_CICD_DSN")
	if dsn == "" {
		dsn = "host=" + os.Getenv("METAFLOW_CICD_DB_HOST") +
			" port=" + os.Getenv("METAFLOW_CICD_DB_PORT") +
			" user=" + os.Getenv("METAFLOW_CICD_DB_USER") +
			" password=" + os.Getenv("METAFLOW_CICD_DB_PASSWORD") +
			" dbname=" + os.Getenv("METAFLOW_CICD_DB_NAME") +
			" sslmode=" + os.Getenv("METAFLOW_CICD_DB_SSLMODE")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalln("DB connect failed:", err)
	}
	return db
}

func main() {
	serviceName := os.Getenv("CI_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "my-service"
	}

	// 1. DB에서 프로젝트 조회 (Repository 레이어, 없으면 환경 변수로 대체)
	db := connectDB()
	repo, err := repository.New(db)
	if err != nil {
		log.Fatalln("Repository init (AutoMigrate):", err)
	}
	ctx := context.Background()
	project, err := repo.GetProjectByServiceName(ctx, serviceName)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		log.Fatalln("GetProjectByServiceName:", err)
	}
	var serviceNameVal, repoURL, branch string
	if project != nil {
		serviceNameVal = project.ServiceName
		repoURL = project.RepoURL
		branch = project.Branch
	}
	if serviceNameVal == "" {
		serviceNameVal = serviceName
		repoURL = os.Getenv("CI_REPO_URL")
		branch = os.Getenv("CI_BRANCH")
		if branch == "" {
			branch = "main"
		}
		log.Printf("DB에 프로젝트 없음, env 사용: %s %s %s", serviceNameVal, repoURL, branch)
	}

	// 2. Temporal Client 연결 (서비스 이름으로 접속)
	// 기본값 temporal:7233 → 같은 Docker 네트워크에서 컨테이너 이름으로 해석됨
	// 호스트에서 실행 시: TEMPORAL_ADDRESS=localhost:7233 또는 /etc/hosts에 127.0.0.1 temporal
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "temporal:7233"
	}
	// localhost → 127.0.0.1 로 바꿔 IPv4로 연결 (IPv6 [::1] connection refused 방지)
	if strings.HasPrefix(hostPort, "localhost:") {
		hostPort = "127.0.0.1" + hostPort[len("localhost"):]
	}
	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		log.Fatalln("Temporal client (주소:", hostPort, "):", err,
			"— Temporal 서버가 떠 있는지, 포트가 열려 있는지 확인하세요.")
	}
	defer c.Close()

	// 3. Workflow 실행
	workflowID := "CI-" + serviceNameVal
	if workflowID == "CI-" {
		workflowID = "CI-default"
	}
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "ci-task-queue",
	}
	workflowArgs := workflow.PipelineRequest{
		ServiceName: serviceNameVal,
		RepoURL:     repoURL,
		Branch:      branch,
	}

	we, err := c.ExecuteWorkflow(context.Background(), options, workflow.CIWorkflow, workflowArgs)
	if err != nil {
		log.Fatalln("ExecuteWorkflow:", err)
	}
	log.Printf("Workflow started: %s", we.GetID())
}
