#!/bin/bash
# Bring up shared infra (Postgres, Redis, Mailpit, MinIO; optional Kafka stack).
# Env: infra/.env (Jenkins copies JENKINS_ENV_FILE there).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INFRA_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

WITH_EVENTING=false

while [[ $# -gt 0 ]]; do
	case "$1" in
		--eventing) WITH_EVENTING=true; shift ;;
		-h|--help)
			echo "Usage: $0 [--eventing]"
			exit 0
			;;
		*)
			echo "unknown flag: $1" >&2
			exit 2
			;;
	esac
done

if [ ! -f "$INFRA_DIR/.env" ]; then
	echo "[ERROR] Missing $INFRA_DIR/.env — copy from .env.example or Jenkins env file" >&2
	exit 1
fi

cd "$INFRA_DIR"

if [ "$WITH_EVENTING" = true ]; then
	echo "[INFO] Starting infra with eventing (Kafka + schema-registry)..."
	make up-eventing
else
	echo "[INFO] Starting core infra (postgres redis mailpit minio)..."
	make up
fi

echo "[INFO] Waiting for containers to become healthy..."
# Compose healthchecks cover postgres/redis; give them time before exit.
deadline=$((SECONDS + 120))
while true; do
	unhealthy="$(docker compose \
		-f compose.yml \
		-f compose.postgres.yml \
		-f compose.redis.yml \
		-f compose.kafka.yml \
		-f compose.schema-registry.yml \
		-f compose.mailpit.yml \
		-f compose.minio.yml \
		--env-file .env \
		--profile eventing \
		ps --format '{{.Name}} {{.Health}}' 2>/dev/null | awk '$2 == "unhealthy" {print}' || true)"

	starting="$(docker compose \
		-f compose.yml \
		-f compose.postgres.yml \
		-f compose.redis.yml \
		-f compose.kafka.yml \
		-f compose.schema-registry.yml \
		-f compose.mailpit.yml \
		-f compose.minio.yml \
		--env-file .env \
		--profile eventing \
		ps --format '{{.Name}} {{.Health}}' 2>/dev/null | awk '$2 == "starting" {print}' || true)"

	if [ -n "$unhealthy" ]; then
		echo "[ERROR] Unhealthy services:" >&2
		echo "$unhealthy" >&2
		make ps || true
		exit 1
	fi

	if [ -z "$starting" ]; then
		break
	fi

	if [ "$SECONDS" -ge "$deadline" ]; then
		echo "[ERROR] Timed out waiting for healthy infra" >&2
		make ps || true
		exit 1
	fi

	sleep 3
done

echo "[SUCCESS] Infra is up"
make ps
