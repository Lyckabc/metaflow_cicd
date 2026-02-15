# CI/CD 워크플로우 (PR / Merge)

## 이벤트별 동작

| GitHub 이벤트 | 조건 | BuildMode | 동작 |
|---------------|------|-----------|------|
| `pull_request` | action=opened, synchronize | CI | PR head 브랜치 clone → run_command (테스트) → docker build + push |
| `pull_request` | action=closed, merged=true | CD | target 브랜치 clone → docker build + push |
| `push` | - | CD | push된 브랜치 clone → docker build + push |

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
