#!/bin/bash
# Test metaflow_cicd pipeline
# Prerequisites: DB running, Temporal server running, API server and Worker running
# Usage: ./scripts/test_pipeline.sh
# From host: TRIGGER_URL=http://localhost:8080 (default)
# From docker: TRIGGER_URL=http://metaflow_manager:8080

set -e
API_URL="${METAFLOW_CICD_API_URL:-http://localhost:8059}"
TRIGGER_URL="${TRIGGER_URL:-http://localhost:8080}"

echo "=== 1. Create project (metaflow_cicd) ==="
curl -s -X POST "$API_URL/projects" \
  -H "Content-Type: application/json" \
  -d '{
    "project_name": "metaflow_cicd",
    "main_repo_url": "https://github.com/lyckabc/metaflow_cicd",
    "target_branches": ["main", "dev", "^feature/.*"],
    "ci_config_path": "metaflow_ci.py",
    "cd_config_path": "metaflow_ci.py",
    "description": "Self CI/CD test"
  }' | jq . || true

echo ""
echo "=== 2. Trigger workflow ==="
RESP=$(curl -s -X POST "$TRIGGER_URL/trigger" \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "metaflow_cicd",
    "repo_url": "https://github.com/lyckabc/metaflow_cicd",
    "branch": "main",
    "build_mode": "ci"
  }')
echo "$RESP" | jq .
WF_ID=$(echo "$RESP" | jq -r '.workflow_id')
echo ""
echo "Workflow ID: $WF_ID"
echo "Check Temporal UI for execution status."
