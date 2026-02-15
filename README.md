# metaflow_cicd

Temporal 기반 CI/CD 워크플로우. Worker가 `ci-task-queue`에서 워크플로우/액티비티를 처리합니다.

## 사전 요구사항

- Docker, Docker Compose
- **neunexus** 외부 네트워크 (이미 생성되어 있어야 함)
- PostgreSQL: Temporal DB 및(선택) metaflow_cicd용 DB가 `.env`에 설정되어 있어야 함

### neunexus 네트워크 (macvlan)

neunexus가 아래처럼 macvlan으로 만들어져 있으면, 컨테이너는 실행할 때마다 다른 IP(192.168.0.0/24 대역)를 받을 수 있습니다.

```bash
docker network create -d macvlan \
  --subnet=192.168.0.0/24 \
  --gateway=192.168.0.1 \
  -o parent=enp5s0 \
  neunexus
```

- **서비스 간 통신**: `temporal:7233`, `elasticsearch:9200`처럼 **컨테이너 이름**을 쓰면, Docker 내부 DNS가 현재 컨테이너 IP로 자동 해석하므로 IP가 바뀌어도 문제 없습니다.
- **호스트에서 접속**: Temporal gRPC는 `ports: "7233:7233"`으로 노출되므로 호스트에서는 항상 `localhost:7233`으로 접속하면 됩니다.
- **외부 DB(toji.homes 등)**: Temporal/Worker가 DB에 접속하는 쪽이므로, DB 서버의 방화벽·pg_hba에서 이 네트워크 대역(192.168.0.0/24) 또는 Docker 호스트 IP를 허용하면 됩니다.

## 1. 환경 설정

프로젝트 루트(`temporal/`)에 `.env` 파일이 있어야 합니다.

```bash
cd /morphogen/neunexus/cicd/temporal
# .env 에 DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME 등 설정 확인
```

## 2. 서비스 실행

```bash
cd /morphogen/neunexus/cicd/temporal
docker compose up -d
```

기동되는 서비스:

| 서비스 | 설명 | 접속 |
|--------|------|------|
| temporal | Temporal 서버 (gRPC) | 네트워크 내 `temporal:7233`, 호스트 `localhost:7233` |
| temporal-ui | 웹 UI | 설정에 따라 `PORT`/프록시 등 |
| metaflow_cicd | CI 워크플로우 Worker (ci-task-queue) | - |
| metaflow_manager | Trigger + WebhookAdapter (POST /trigger, /webhooks/github) | - |
| temporal-admin-tools | CLI 도구 | `docker exec -it temporal-admin-tools tctl ...` |

## 3. DB 테이블(DDL) 및 POST API (sources, projects, secrets)

테이블은 GORM `AutoMigrate`로 생성됩니다. DDL은 `migrations/001_schema.sql` 참조.

### 워크플로우 구조 (Manager → Runner)

- **ManagerWorkflow**: Pre-flight 검사 (main_repo_url 존재, branch 패턴 매칭), Source 참조, Runner 트리거
- **RunnerWorkflow**: Git clone, config 검증, secret 주입, `python <config_path> run` 실행

### POST로 DB에 데이터 넣기 (API 서버)

`cmd/server`는 POST로 `sources`, `projects`, `secrets`를 DB에 넣는 HTTP API를 제공합니다.

**서버 실행:**

```bash
cd /morphogen/neunexus/cicd/temporal/metaflow_cicd
go run ./cmd/server   # 기본 포트 8059
```

**POST 요청 예시:**

```bash
# projects 등록
curl -X POST http://localhost:8059/projects \
  -H "Content-Type: application/json" \
  -d '{"project_name":"metaflow_cicd","main_repo_url":"https://github.com/neunexus/metaflow_cicd","target_branches":["main","dev","^feature/.*"],"ci_config_path":"metaflow_ci.py","cd_config_path":"metaflow_ci.py"}'

# secrets 등록 (project_id 필요)
curl -X POST http://localhost:8059/secrets \
  -H "Content-Type: application/json" \
  -d '{"project_id":1,"secret_key":"REGISTRY_ID","secret_value":"user","scope":"prod"}'
```

### metaflow_cicd 프로젝트 시드

```bash
go run ./cmd/seed   # metaflow_cicd 프로젝트 생성
```

### 파이프라인 테스트

```bash
# 1. API 서버, metaflow_manager, metaflow_cicd Worker 실행 후
./scripts/test_pipeline.sh
```

## 4. 워크플로우 실행 방법

### 방법 1) POST /trigger (Webhook 또는 수동)

```bash
curl -X POST http://localhost:8080/trigger \
  -H "Content-Type: application/json" \
  -d '{"service_name":"metaflow_cicd","repo_url":"https://github.com/neunexus/metaflow_cicd","branch":"main","build_mode":"ci"}'
```

### 방법 2) Temporal CLI (tctl)

```bash
docker exec -it temporal-admin-tools tctl workflow start \
  --taskqueue ci-task-queue \
  --workflow_type ManagerWorkflow \
  --input '{"ServiceName":"metaflow_cicd","RepoURL":"https://github.com/neunexus/metaflow_cicd","Branch":"main","BuildMode":"ci"}'
```

### 방법 3) 코드에서 클라이언트로 실행

```go
c, _ := client.Dial(client.Options{HostPort: "temporal:7233"})
defer c.Close()
options := client.StartWorkflowOptions{ID: "ci-metaflow_cicd-1", TaskQueue: "ci-task-queue"}
args := workflow.PipelineRequest{
    ServiceName: "metaflow_cicd",
    RepoURL:     "https://github.com/neunexus/metaflow_cicd",
    Branch:      "main",
    BuildMode:   "ci",
}
we, err := c.ExecuteWorkflow(context.Background(), options, workflow.ManagerWorkflow, args)
```

## 5. 로그 확인

```bash
# Worker 로그
docker logs -f metaflow_cicd

# Temporal 서버 로그
docker logs -f temporal
```

## 6. 빌드만 다시 하기

Worker 코드 수정 후 이미지만 다시 빌드:

```bash
cd /morphogen/neunexus/cicd/temporal
docker compose build metaflow_cicd
docker compose up -d metaflow_cicd
```

## 디렉터리 구조

```
metaflow_cicd/
├── cmd/
│   ├── server/     # POST API (sources, projects, secrets)
│   ├── worker/     # Temporal Worker (ManagerWorkflow, RunnerWorkflow)
│   └── seed/       # metaflow_cicd 프로젝트 시드
├── workflow/       # ManagerWorkflow, RunnerWorkflow, PipelineRequest
├── internal/
│   ├── repository/ # sources, projects, secrets
│   └── runner/     # PreFlightCheckActivity, RunMetaflowActivity
├── metaflow_ci.py  # 샘플 CI용 Python 스크립트 (python metaflow_ci.py run)
├── metaflow-ci.toml # 파이프라인 설정 예시
├── migrations/     # DDL
└── scripts/        # test_pipeline.sh
```

Worker는 `ManagerWorkflow`(Pre-flight → Runner)와 `RunnerWorkflow`(Metaflow 실행)를 등록합니다.
