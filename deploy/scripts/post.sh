#!/bin/bash
# Terminal POST stage: idempotent seed / post-deploy hooks. Never runs migrations.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/load-config.sh"
load_env_if_present || true

compose_env_args=()
[[ -f "$APP_ENV_FILE" ]] && compose_env_args+=(--env-file "$APP_ENV_FILE")

export DOCKER_BUILD_CONTEXT="$PROJECT_DIR"
export IMAGE_NAME="${IMAGE_NAME:-golang_template:latest}"

cd "$PROJECT_DIR"
docker compose -f "$COMPOSE_FILE" "${compose_env_args[@]}" --profile tools run --rm --no-deps golang_template_post
