#!/bin/bash
# Load deploy configuration and detect environment (Jenkins vs local)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_DIR="$(cd "$DEPLOY_DIR/.." && pwd)"

if [ ! -f "$SCRIPT_DIR/config.sh" ]; then
    echo "[ERROR] config.sh not found" >&2
    exit 1
fi
# shellcheck disable=SC1091
source "$SCRIPT_DIR/config.sh"

if [ -z "$PROJECT_NAME" ] || [ -z "$IMAGE_NAME" ] || [ -z "$COMPOSE_FILE" ]; then
    echo "[ERROR] Missing required config: PROJECT_NAME, IMAGE_NAME, COMPOSE_FILE" >&2
    exit 1
fi

if [ -n "${JENKINS_HOME:-}" ] || [ -n "${WORKSPACE:-}" ]; then
    export IS_JENKINS=true
    export ENV_FILE="${ENV_FILE:-$JENKINS_ENV_FILE}"
    export SECRETS_FILE="${SECRETS_FILE:-$JENKINS_SECRETS_FILE}"
    export PROJECT_DIR="${WORKSPACE:-$PROJECT_DIR}"

    if [ -z "$JENKINS_ENV_FILE" ] || [ -z "$JENKINS_SECRETS_FILE" ]; then
        echo "[ERROR] Missing Jenkins config: JENKINS_ENV_FILE, JENKINS_SECRETS_FILE" >&2
        exit 1
    fi
else
    export IS_JENKINS=false
fi

if [[ "$COMPOSE_FILE" != /* ]]; then
    export COMPOSE_FILE="$PROJECT_DIR/$COMPOSE_FILE"
fi

# App process env. Keys match root .envrc.example. You create this file.
export APP_ENV_FILE="${APP_ENV_FILE:-$PROJECT_DIR/deploy/.env}"

load_env_if_present() {
    if [[ -f "$APP_ENV_FILE" ]]; then
        set -a
        # shellcheck disable=SC1091
        source "$APP_ENV_FILE"
        set +a
        return 0
    fi
    if [ "$IS_JENKINS" = "true" ]; then
        echo "[ERROR] .env file not found: $APP_ENV_FILE" >&2
        return 1
    fi
    return 0
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    load_env_if_present
fi
