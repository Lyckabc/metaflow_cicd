# metaflow_ci.go 테스트 방법

`flows/runner/metaflow_ci.go`는 metaflow_cicd Self CI를 Temporal SDK로 직접 실행하는 테스트입니다.
`test_pipeline.sh`와 동일한 build test를 Trigger API 없이 Temporal 클라이언트로 트리거합니다.

CI Runner는 `flows/metaflow-ci.toml` [build]에서 `metaflow_ci.go run`을 사용합니다.

## 사전 요구사항

- metaflow_cicd DB (projects, secrets)
- Temporal 서버
- metaflow_cicd Worker (ci-task-queue)
- metaflow_cicd API 서버 (프로젝트 등록용, `-create-project` 사용 시)

## 실행 방법

### 1. 로컬에서 (Temporal이 localhost:7233에 노출된 경우)

```bash
cd metaflow_cicd

# 워크플로우만 트리거 (비동기)
go run ./flows/runner/metaflow_ci.go

# 완료까지 대기 (동기)
go run ./flows/runner/metaflow_ci.go -wait

# 옵션 예시
go run ./flows/runner/metaflow_ci.go -branch dev -mode cd -wait
```

### 2. Docker 네트워크 내에서 (neunexus 네트워크)

Temporal이 Docker 내부에만 노출된 경우, `neunexus` 네트워크에 연결된 컨테이너에서 실행:

```bash
cd metaflow_cicd

docker run --rm --network neunexus \
  -v "$(pwd)":/app -w /app \
  -e TEMPORAL_ADDRESS=temporal:7233 \
  -e METAFLOW_CICD_API_URL=http://metaflow_manager:8080 \
  golang:1.24 go run ./flows/runner/metaflow_ci.go -create-project=false -wait
```

## 옵션

| 옵션 | 기본값 | 설명 |
|------|--------|------|
| `-branch` | dev | 대상 브랜치 |
| `-mode` | ci | build mode: ci 또는 cd |
| `-wait` | false | 워크플로우 완료까지 대기 |
| `-create-project` | true | API로 프로젝트 생성/갱신 |

## 환경 변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `TEMPORAL_ADDRESS` | localhost:7233 | Temporal 서버 주소 |
| `METAFLOW_CICD_API_URL` | http://localhost:8059 | API 서버 (프로젝트 등록용) |

## 성공 시 출력 예시

```
==============================================
 metaflow_cicd Self CI Test (Temporal SDK)
==============================================
...
=== 3. Waiting for workflow completion ===
 Success: true
 ExitCode: 0
...
Build test passed.
```
