#!/usr/bin/env bash
set -euo pipefail

run_step() {
  local name="$1"
  local command="$2"
  echo "==> $name"
  bash -o pipefail -c "$command"
}

run_step PRECHECK "${PRECHECK_CMD:-go test ./... && make lint}"
run_step BACKUP "${BACKUP_CMD:-:}"
run_step DB_MIGRATE "${DB_MIGRATE_CMD:-make migrate-up}"
run_step BROKER_PROVISION "${BROKER_PROVISION_CMD:-./scripts/kafka-topics.sh}"
run_step SCHEMA_REGISTER "${SCHEMA_REGISTER_CMD:-./scripts/register-schemas.sh}"
run_step DEPLOY_API "${DEPLOY_API_CMD:-./deploy/scripts/deploy.sh --skip-migrate}"
run_step DEPLOY_WORKER "${DEPLOY_WORKER_CMD:-:}"
run_step VERIFY "${VERIFY_CMD:-:}"
run_step ENABLE "${ENABLE_CMD:-:}"
run_step POST "${POST_CMD:-./deploy/scripts/post.sh}"
