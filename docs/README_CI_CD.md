# CI/CD 워크플로우 (PR / Merge)

## 전체 흐름

```
GitHub Event (PR/push) → Convoy (webhook) → trigger (WebhookAdapter + Trigger) → Temporal Workflow
```

- **Convoy**: GitHub webhook 수신, 검증 후 trigger로 전달
- **trigger**: WebhookAdapter가 payload를 변환하고, Trigger가 `DynamicRunnerWorkflow` 시작
- **cicd-worker**: 단일 worker가 CI/CD 모두 처리 (별도 CI worker / CD worker 없음)

## Worker 구조

| 구분 | 설명 |
|------|------|
| **Worker** | `cicd-worker` 1개 (CI 전용·CD 전용 worker 분리 없음) |
| **Task Queue** | `ci-task-queue` |
| **Workflow** | `DynamicRunnerWorkflow` (BuildMode 파라미터로 CI/CD 분기) |

`pull_request` 이벤트에 따라 **BuildMode**만 다르게 전달되고, 동일한 workflow가 내부에서 분기합니다.

## 이벤트별 동작

| GitHub 이벤트 | 조건 | BuildMode | 동작 |
|---------------|------|-----------|------|
| `pull_request` | action=opened, synchronize | CI | PR head 브랜치 clone → run_command (테스트) → docker build + push |
| `pull_request` | action=closed, merged=true | CD | target 브랜치 clone → docker build + push |
| `push` | - | CD | push된 브랜치 clone → docker build + push |

### Workflow 내부 분기

- **CI** (`BuildMode == "ci"`): `RunWorkActivity` → `DockerBuildPushActivity`
- **CD** (`BuildMode == "cd"`): `DockerBuildPushActivity`만 실행

## GitHub Status API (Branch Protection)

워크플로우 시작/종료 시 GitHub Commit Status를 업데이트하여 Merge를 제한할 수 있습니다.

| 환경 변수 | 서비스 | 설명 |
|-----------|--------|------|
| `GITHUB_TOKEN` | cicd-worker | GitHub PAT 또는 App token |
| `TEMPORAL_UI_BASE_URL` | trigger | Temporal Web UI 주소 (예: https://temporal.toji.homes) |

### Branch Protection 설정

1. GitHub Repo → Settings → Branches → Branch protection rule
2. "Require status checks to pass before merging" 체크
3. Status check에 `temporal/ci` 또는 `temporal/cd` 추가

> 한 번이라도 해당 context로 Status가 전송되어야 리스트에 나타납니다.

## DB 설정

### ci_projects

| 컬럼 | 설명 |
|------|------|
| service_name | 저장소 이름 (예: Nucleus) 또는 owner/repo |
| repo_url | Git clone URL |
| branch | 기본 브랜치 |
| registry_url | Docker 레지스트리 (예: registry.toji.homes) |
| run_command | CI 시 실행할 명령 (예: pip install -r requirements.txt) |

### ci_secrets

| key | 설명 |
|-----|------|
| REGISTRY_ID | 레지스트리 로그인 ID |
| REGISTRY_PASSWORD | 레지스트리 로그인 비밀번호 |

## 이미지 태그

- 형식: `{registry_url}/{service_name}:{tag}`
- tag: `yymmddhhmm` (예: 2502151430 = 2025-02-15 14:30)

## GitHub Webhook 설정

Convoy/GitHub Webhook에서 다음 이벤트 구독:

- **pull_request** (opened, synchronize, closed)
- **push**

## 파일 구조

| 파일 | 내용 |
|------|------|
| `metaflow_manager/internal/handler/webhook.go` | WebhookAdapter - GitHub payload 변환, BuildMode 결정 |
| `metaflow_manager/internal/handler/trigger.go` | Trigger - 워크플로우 시작 |
| `internal/runner/github_status.go` | UpdateGitHubStatusActivity - GitHub Status API |
| `workflow/pipeline.go` | DynamicRunnerWorkflow - CI/CD 분기 및 Status 업데이트 |
| `cmd/worker/workflow.go` | Workflow/Activity 등록 |
