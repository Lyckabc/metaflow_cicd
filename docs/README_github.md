# GitHub PR Status 및 [Details] 링크 설정 가이드

PR 페이지에 Temporal CI/CD 상태와 [Details] 링크가 표시되려면 아래 설정이 모두 완료되어야 합니다.

## 아키텍처

```
GitHub (PR/push) → Convoy ingest → metaflow_manager → ManagerWorkflow
                                                           ↓
                                              updateGitHubStatusIfSet()
                                                           ↓
                                              UpdateGitHubStatusActivity (GITHUB_TOKEN)
                                                           ↓
                                              GitHub Status API (target_url = Temporal UI)
```

## updateGitHubStatusIfSet 동작 조건

`workflow/pipeline.go`의 `updateGitHubStatusIfSet`는 **다음 4개 필드가 모두 비어있지 않을 때만** GitHub Status API를 호출합니다:

| 필드 | 출처 | 비어있으면 |
|------|------|------------|
| `GitHubOwner` | webhook payload `repository.full_name` | Status 미전송 |
| `GitHubRepo` | webhook payload `repository.full_name` | Status 미전송 |
| `GitHubSHA` | `pull_request`: `head.sha` / `push`: `after` | Status 미전송 |
| `TemporalUIBaseURL` | metaflow_manager 환경변수 `TEMPORAL_UI_BASE_URL` | Status 미전송 (또는 [Details] 링크 없음) |

**하나라도 비어있으면** 함수는 조기 반환하며 GitHub에 아무것도 전송하지 않습니다.

---

## 필수 설정 체크리스트

### 1. metaflow_manager 환경변수

| 변수 | 설명 | 예시 |
|------|------|------|
| `TEMPORAL_UI_BASE_URL` | [Details] 링크의 base URL. **비어있으면 [Details]가 표시되지 않음** | `https://temporal.toji.homes` |
| `CONVOY_ENDPOINT_SECRET` | Convoy webhook 서명 검증 (또는 `GITHUB_WEBHOOK_SECRET`) | `toji-foundation` |

`docker-compose.yml`에서 전달:

```yaml
metaflow_manager:
  environment:
    - TEMPORAL_UI_BASE_URL=${TEMPORAL_UI_BASE_URL:-}
    - CONVOY_ENDPOINT_SECRET=${GITHUB_WEBHOOK_SECRET}
```

`.env`에 반드시 설정:

```
TEMPORAL_UI_BASE_URL=https://temporal.toji.homes
GITHUB_WEBHOOK_SECRET=toji-foundation
```

### 2. metaflow_cicd (worker) 환경변수

| 변수 | 설명 |
|------|------|
| `GITHUB_TOKEN` | GitHub PAT 또는 App token. `repo:status` scope 필요. Status API 호출에 사용 |

`GITHUB_TOKEN`이 없거나 만료되면 `UpdateGitHubStatusActivity`가 실패합니다. (에러는 workflow에서 무시되므로 PR에는 아무 변화 없음)

### 3. Traefik 동적 설정

`lymphhub/config/traefik/dynamic/cicd.yml`:

```yaml
http:
  routers:
    # Temporal Web UI - [Details] 링크 대상
    temporal-ui:
      rule: "Host(`temporal.toji.homes`)"
      entryPoints: [websecure]
      service: temporal-ui
      tls:
        certResolver: cloudflare

    # GitHub → Convoy ingest
    convoy-ingest:
      rule: "Host(`convoy.toji.homes`) && PathPrefix(`/ingest`)"
      entryPoints: [websecure]
      service: convoy-ingest
      priority: 10

    # Convoy 대시보드/API
    convoy-ui:
      rule: "Host(`convoy.toji.homes`)"
      entryPoints: [websecure]
      service: convoy-web

    # Convoy → metaflow_manager (webhook 수신)
    cicd-trigger:
      rule: "Host(`cicd.toji.homes`) && PathPrefix(`/webhooks`)"
      entryPoints: [websecure]
      service: metaflow_manager

  services:
    temporal-ui:
      loadBalancer:
        servers:
          - url: "http://temporal-ui:8080"
        passHostHeader: true

    convoy-web:
      loadBalancer:
        servers:
          - url: "http://convoy-web:5005"
        passHostHeader: true

    convoy-ingest:
      loadBalancer:
        servers:
          - url: "http://convoy-web:5007"
        passHostHeader: true

    cicd-trigger:
      loadBalancer:
        servers:
          - url: "http://metaflow_manager:8080"
        passHostHeader: true
```

**필수 네트워크**: `temporal-ui`, `convoy-web`, `metaflow_manager`가 Traefik과 동일한 `neunexus` 네트워크에 있어야 합니다.

### 4. Convoy 설정

- **Source**: GitHub provider, HMAC verifier (X-Hub-Signature-256)
- **Endpoint URL**: `https://cicd.toji.homes/webhooks/github` (Traefik으로 노출)
- **Subscription**: Source → Endpoint 연결

`setup_convoy_github.py` 실행:

```bash
source .env
python setup_convoy_github.py \
  --github-secret "$GITHUB_WEBHOOK_SECRET" \
  --endpoint-url "https://cicd.toji.homes/webhooks/github"
```

### 5. GitHub Webhook 등록

| 항목 | 값 |
|------|-----|
| Payload URL | `https://convoy.toji.homes/ingest/{source_uid}` |
| Content type | `application/json` |
| Secret | `GITHUB_WEBHOOK_SECRET`와 동일 |
| Events | `pull_request` (opened, synchronize, closed) + `push` |

### 6. GitHub Branch Protection (선택)

1. Repo → Settings → Branches → Branch protection rule
2. "Require status checks to pass before merging" 체크
3. Status check에 `temporal/ci` 또는 `temporal/cd` 추가

> 한 번이라도 해당 context로 Status가 전송되어야 리스트에 나타납니다.

---

## [Details]가 보이지 않을 때 점검 사항

### 1. TEMPORAL_UI_BASE_URL 확인

```bash
# metaflow_manager 컨테이너에서 확인
docker exec metaflow_manager env | grep TEMPORAL_UI_BASE_URL
# 기대: TEMPORAL_UI_BASE_URL=https://temporal.toji.homes
```

비어있으면 `.env`에 추가 후 `docker compose up -d metaflow_manager` 재시작.

### 2. X-GitHub-Event 헤더 전달 확인

Convoy가 endpoint로 전달할 때 `X-GitHub-Event` 헤더를 유지해야 합니다. 이 헤더가 없으면 webhook handler가 `pull_request`를 `push`로 잘못 파싱하여 `GitHubSHA`가 비어있을 수 있습니다.

- **pull_request** 이벤트: `GitHubSHA` = `pull_request.head.sha`
- **push** 이벤트: `GitHubSHA` = `after`

Convoy가 헤더를 전달하지 않는 경우, Convoy 설정 또는 버전을 확인하세요.

### 3. GITHUB_TOKEN 확인

```bash
# metaflow_cicd worker에서 확인
docker exec metaflow_cicd env | grep GITHUB_TOKEN
```

- 토큰 만료 여부 확인
- `repo` 또는 `repo:status` scope 필요

### 4. Activity 에러 (조용한 실패)

`updateGitHubStatusIfSet`는 `UpdateGitHubStatusActivity` 실패 시 에러를 무시합니다. Temporal UI에서 해당 Workflow의 Activity History를 확인해 `UpdateGitHubStatusActivity`가 실패했는지 확인할 수 있습니다.

### 5. Temporal UI 접근 가능 여부

`https://temporal.toji.homes`가 외부에서 접근 가능해야 PR의 [Details] 링크가 동작합니다. Traefik 라우팅 및 인증서를 확인하세요.

---

## 관련 파일

| 파일 | 역할 |
|------|------|
| `metaflow_manager/internal/handler/webhook.go` | GitHub payload 파싱, PipelineRequest 생성 (GitHubOwner, GitHubRepo, GitHubSHA, TemporalUIBaseURL) |
| `metaflow_cicd/workflow/manager.go` | ManagerWorkflow - updateGitHubStatusIfSet 호출 |
| `metaflow_cicd/workflow/pipeline.go` | updateGitHubStatusIfSet, DynamicRunnerWorkflow |
| `metaflow_cicd/internal/runner/github_status.go` | UpdateGitHubStatusActivity - GitHub Status API 호출 |
| `lymphhub/config/traefik/dynamic/cicd.yml` | Traefik 라우팅 (temporal-ui, convoy, metaflow_manager) |
