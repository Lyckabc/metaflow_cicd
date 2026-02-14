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
| cicd-worker | CI 워크플로우 Worker (ci-task-queue) | - |
| temporal-admin-tools | CLI 도구 | `docker exec -it temporal-admin-tools tctl ...` |
| starter | 워크플로우 트리거 (profile: tools) | `docker compose --profile tools run --rm starter` |

## 3. DB 테이블(DDL) 및 POST API (ci_projects / ci_secrets)

테이블은 GORM `AutoMigrate`로 생성되거나, 아래 DDL로 직접 생성할 수 있습니다.

```sql
CREATE TABLE public.ci_projects (
	id serial4 NOT NULL,
	service_name varchar(50) NULL,
	repo_url text NOT NULL,
	branch varchar(50) DEFAULT 'main'::character varying NULL,
	registry_url text NULL,
	CONSTRAINT ci_projects_pkey PRIMARY KEY (id),
	CONSTRAINT ci_projects_service_name_key UNIQUE (service_name)
);

CREATE TABLE public.ci_secrets (
	id serial4 NOT NULL,
	"key" varchar(50) NULL,
	value text NOT NULL,
	description text NOT NULL,
	CONSTRAINT ci_secrets_key_key UNIQUE (key),
	CONSTRAINT ci_secrets_pkey PRIMARY KEY (id)
);
```

### POST로 DB에 데이터 넣기 (API 서버)

`cmd/server`는 POST로 `ci_projects`, `ci_secrets`를 DB에 넣는 HTTP API를 제공합니다.

**서버 실행 (GORM 사용):**

```bash
cd /morphogen/neunexus/cicd/temporal/metaflow_cicd

# 환경 변수 설정 (temporal/.env 기준으로 아래처럼 export 하거나 .env 로드)
export METAFLOW_CICD_DB_HOST=toji.homes
export METAFLOW_CICD_DB_PORT=5432
export METAFLOW_CICD_DB_NAME=metaflow_cicd
export METAFLOW_CICD_DB_USER=admin_metaflow
export METAFLOW_CICD_DB_PASSWORD='metaflow_Foundation$3'
export METAFLOW_CICD_DB_SSLMODE=disable

# API 서버 기동 (기본 포트 8080, METAFLOW_CICD_API_PORT 로 변경 가능)
go run ./cmd/server
go run ./cmd/server --host 0.0.0.0
```

**POST 요청 예시:**

```bash
# ci_projects 등록 (repo_url 필수, branch 생략 시 main)
curl -X POST http://localhost:8080/ci_projects \
  -H "Content-Type: application/json" \
  -d '{"service_name":"my-service","repo_url":"https://github.com/org/repo","branch":"main","registry_url":"https://registry.example.com"}'

# ci_secrets 등록 (key, value, description 모두 필수)
curl -X POST http://localhost:8080/ci_secrets \
  -H "Content-Type: application/json" \
  -d '{"key":"registry_password","value":"secret123","description":"Registry login password"}'
```

### GORM 실행 방법 요약

| 목적 | 명령 | 비고 |
|------|------|------|
| API 서버 (POST로 DB 입력) | `go run ./cmd/server` | 위 환경 변수 설정 후 실행 |
| Starter (워크플로우 트리거) | `go run ./cmd/starter` | `CI_SERVICE_NAME` 등 env 또는 DB에서 프로젝트 조회 |

- **DB 연결**: `METAFLOW_CICD_DSN` 한 번에 지정하거나, `METAFLOW_CICD_DB_HOST`, `METAFLOW_CICD_DB_PORT`, `METAFLOW_CICD_DB_USER`, `METAFLOW_CICD_DB_PASSWORD`, `METAFLOW_CICD_DB_NAME`, `METAFLOW_CICD_DB_SSLMODE` 로 분리 지정.
- **AutoMigrate**: `repository.New(db)` 호출 시 `ci_projects`, `ci_secrets` 테이블이 없으면 생성됩니다.

## 4. 워크플로우 실행 방법

### 방법 1) Starter (Docker, 권장)

같은 네트워크에서 **서비스 이름** `temporal:7233`으로 접속해 워크플로우를 한 번 시작합니다.

**DB에 프로젝트가 있을 때** (테이블 `ci_projects`, 컬럼 `service_name`, `repo_url`, `branch`):

```bash
cd /morphogen/neunexus/cicd/temporal
docker compose --profile tools run --rm starter
```

**DB 없이 / 특정 프로젝트만 실행** — 환경 변수로 지정:

```bash
# 예: my-api 서비스, 브랜치 main
CI_SERVICE_NAME=my-api CI_REPO_URL=https://github.com/org/my-api CI_BRANCH=main \
  docker compose --profile tools run --rm starter

# .env 에 넣거나 export 해도 됨
export CI_SERVICE_NAME=my-api
export CI_REPO_URL=https://github.com/org/my-api
export CI_BRANCH=main
docker compose --profile tools run --rm starter
```

- `CI_SERVICE_NAME`: 프로젝트 식별자 (기본 `my-service`). DB에 있으면 DB 값 사용, 없으면 env 사용.
- `CI_REPO_URL`, `CI_BRANCH`: DB에 행이 없을 때만 사용 (기본 브랜치 `main`).

### 방법 2) Temporal CLI (tctl)

Worker가 떠 있는 상태에서 수동으로 한 번 시작:

```bash
docker exec -it temporal-admin-tools tctl workflow start \
  --taskqueue ci-task-queue \
  --workflow_type CIWorkflow \
  --input '{"ServiceName":"my-service","RepoURL":"https://github.com/org/repo","Branch":"main"}'
```

### 방법 3) Starter (호스트에서 go run)

- `TEMPORAL_ADDRESS=127.0.0.1:7233 go run ./cmd/starter`
- 또는 `/etc/hosts`에 `127.0.0.1 temporal` 추가 후 `go run ./cmd/starter`

Starter는 DB(`ci_projects`)에서 `CI_SERVICE_NAME`(기본 `my-service`)으로 프로젝트를 조회하고, 없으면 env `CI_REPO_URL`, `CI_BRANCH`를 씁니다.

### 방법 4) 코드에서 클라이언트로 실행

다른 서비스에서 Temporal 클라이언트로 워크플로우를 시작할 수 있습니다. 같은 네트워크면 서비스 이름 사용.

```go
c, _ := client.Dial(client.Options{HostPort: "temporal:7233"})
defer c.Close()

options := client.StartWorkflowOptions{
    ID:        "CI-my-service-1",
    TaskQueue: "ci-task-queue",
}
args := workflow.PipelineRequest{
    ServiceName: "my-service",
    RepoURL:     "https://github.com/org/repo",
    Branch:      "main",
}
we, err := c.ExecuteWorkflow(context.Background(), options, workflow.CIWorkflow, args)
```

## 5. 로그 확인

```bash
# Worker 로그
docker logs -f cicd-worker

# Temporal 서버 로그
docker logs -f temporal
```

## 6. 빌드만 다시 하기

Worker 코드 수정 후 이미지만 다시 빌드:

```bash
cd /morphogen/neunexus/cicd/temporal
docker compose build cicd-worker
docker compose up -d cicd-worker
```

## 디렉터리 구조

```
metaflow_cicd/
├── cmd/
│   ├── server/     # POST API (ci_projects, ci_secrets 등록)
│   ├── worker/     # Temporal Worker (워크플로우/액티비티 실행)
│   └── starter/    # 워크플로우 트리거 (DB 조회 후 시작)
├── workflow/       # CIWorkflow, PipelineRequest 등 공용 정의
├── internal/       # repository, secret store 등
├── go.mod, go.sum
└── README.md
```

Worker는 `CIWorkflow`와 `DaggerBuildActivity`를 등록하며, 현재 액티비티는 스텁입니다. 실제 Dagger 파이프라인 연동은 `cmd/worker/activity.go`에서 구현하면 됩니다.
