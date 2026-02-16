#!/bin/bash
# metaflow_cicd Self CI Test Script
#
# metaflow_cicd 프로젝트의 CI 파이프라인을 직접 테스트하는 스크립트입니다.
# GIT_WEBHOOK_FLOW.md (metaflow_manager/docs/) 참고.
#
# Prerequisites:
#   - metaflow_cicd DB (projects, secrets)
#   - Temporal 서버
#   - metaflow_manager (Trigger)
#   - metaflow_cicd Worker (ci-task-queue)
#   - metaflow_cicd API 서버 (프로젝트/시크릿 등록용)
#
# Usage:
#   ./scripts/test_pipeline.sh
#
# Env:
#   METAFLOW_CICD_API_URL  - API 서버 (default: http://localhost:8059)
#   TRIGGER_URL            - Trigger URL (default: http://localhost:8080)
#   TRIGGER_VIA_DOCKER     - 1이면 docker run으로 metaflow_manager:8080 호출 (trigger가 localhost에 없을 때)

set -e

# --- metaflow_cicd 샘플 프로젝트 설정 (GIT_WEBHOOK_FLOW.md §6) ---
PROJECT_NAME="metaflow_cicd"
REPO_URL="https://github.com/Lyckabc/metaflow_cicd"
BRANCH="dev"
BUILD_MODE="${BUILD_MODE:-ci}"
TARGET_BRANCHES='["main", "dev", "^feature/.*", "^release-.*"]'
CI_CONFIG_PATH="flows/metaflow-ci.toml"

API_URL="${METAFLOW_CICD_API_URL:-http://localhost:8059}"
TRIGGER_URL="${TRIGGER_URL:-http://localhost:8080}"
TRIGGER_VIA_DOCKER="${TRIGGER_VIA_DOCKER:-0}"

echo "=============================================="
echo " metaflow_cicd Self CI Test"
echo "=============================================="
echo " Project: $PROJECT_NAME"
echo " Repo:    $REPO_URL"
echo " Branch:  $BRANCH"
echo " Mode:    $BUILD_MODE"
echo " API:     $API_URL"
echo " Trigger: $TRIGGER_URL"
echo "=============================================="
echo ""

# --- 1. Create/Update project ---
echo "=== 1. Create/Update project ($PROJECT_NAME) ==="
PROJECT_RESP=$(curl -s -X POST "$API_URL/projects" \
  -H "Content-Type: application/json" \
  -d "{
    \"project_name\": \"$PROJECT_NAME\",
    \"main_repo_url\": \"$REPO_URL\",
    \"target_branches\": $TARGET_BRANCHES,
    \"ci_config_path\": \"$CI_CONFIG_PATH\",
    \"cd_config_path\": \"$CI_CONFIG_PATH\",
    \"description\": \"metaflow_cicd self CI/CD (sample)\"
  }" 2>/dev/null || echo "")

if command -v jq >/dev/null 2>&1; then
  echo "$PROJECT_RESP" | jq . 2>/dev/null || echo "$PROJECT_RESP"
else
  echo "$PROJECT_RESP"
fi
# duplicate key 등은 정상 (이미 등록됨)
echo ""

# --- 2. Trigger workflow (POST /trigger) ---
echo "=== 2. Trigger workflow (simulate CI) ==="
if command -v jq >/dev/null 2>&1; then
  TRIGGER_PAYLOAD=$(jq -n \
    --arg sn "$PROJECT_NAME" \
    --arg url "$REPO_URL" \
    --arg br "$BRANCH" \
    --arg mode "$BUILD_MODE" \
    '{service_name: $sn, repo_url: $url, branch: $br, build_mode: $mode}')
else
  TRIGGER_PAYLOAD="{\"service_name\":\"$PROJECT_NAME\",\"repo_url\":\"$REPO_URL\",\"branch\":\"$BRANCH\",\"build_mode\":\"$BUILD_MODE\"}"
fi

if [ "$TRIGGER_VIA_DOCKER" = "1" ]; then
  RESP=$(docker run --rm --network neunexus curlimages/curl:latest -s -X POST http://metaflow_manager:8080/trigger \
    -H "Content-Type: application/json" -d "$TRIGGER_PAYLOAD" 2>/dev/null || echo '{"error":"trigger failed"}')
else
  RESP=$(curl -s -X POST "$TRIGGER_URL/trigger" \
    -H "Content-Type: application/json" \
    -d "$TRIGGER_PAYLOAD" 2>/dev/null || echo '{"error":"connection refused - try TRIGGER_VIA_DOCKER=1"}')
fi

if command -v jq >/dev/null 2>&1; then
  echo "$RESP" | jq .
  WF_ID=$(echo "$RESP" | jq -r '.workflow_id // empty')
  RUN_ID=$(echo "$RESP" | jq -r '.run_id // empty')
else
  echo "$RESP"
  WF_ID=$(echo "$RESP" | grep -o '"workflow_id"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)
  RUN_ID=$(echo "$RESP" | grep -o '"run_id"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)
fi
echo ""

# --- 3. Result ---
if [ -n "$WF_ID" ]; then
  echo "=== 3. Result ==="
  echo " Workflow ID: $WF_ID"
  [ -n "$RUN_ID" ] && echo " Run ID:      $RUN_ID"
  echo ""
  echo " Check Temporal UI for execution status."
  echo " Flow: PreFlightCheck -> RunnerWorkflow (metaflow_ci.py run) -> DockerBuildPush (on success)"
else
  echo "=== 3. Result ==="
  echo " Trigger failed. Check:"
  echo "  - TRIGGER_URL ($TRIGGER_URL) reachable?"
  echo "  - Or set TRIGGER_VIA_DOCKER=1 when trigger is only inside Docker network"
  exit 1
fi
