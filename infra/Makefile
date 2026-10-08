COMPOSE = docker compose \
	-f compose.yml \
	-f compose.postgres.yml \
	-f compose.redis.yml \
	-f compose.kafka.yml \
	-f compose.schema-registry.yml \
	-f compose.mailpit.yml \
	-f compose.minio.yml \
	--env-file .env

.PHONY: env up up-eventing down stop logs ps restart reset help schemas-clear

help:
	@echo "infra targets:"
	@echo "  make up            start postgres redis mailpit minio (default; no Kafka yet)"
	@echo "  make up-eventing   also start kafka kafka-ui schema-registry"
	@echo "  make schemas-clear delete all subjects from schema registry (if running)"
	@echo "  make down          stop and remove containers"
	@echo "  make stop          stop containers"
	@echo "  make logs s=X      follow logs (e.g. s=redis)"
	@echo "  make ps            list containers"
	@echo "  make restart       restart core services"
	@echo "  make reset         down + delete volumes (destructive)"

env:
	@test -f .env || cp .env.example .env
	@echo ".env ready"

up: env
	@$(COMPOSE) up -d
	@echo "postgres         → localhost:$${POSTGRES_PORT:-5432}"
	@echo "redis            → localhost:$${REDIS_PORT:-6379}"
	@echo "mailpit UI       → http://localhost:$${MAILPIT_UI_PORT:-8025}"
	@echo "mailpit SMTP     → localhost:$${MAILPIT_SMTP_PORT:-1025}"
	@echo "minio API        → localhost:$${MINIO_API_PORT:-9000}"
	@echo "minio console    → http://localhost:$${MINIO_CONSOLE_PORT:-9001}"
	@echo "(kafka/schema-registry: make up-eventing when you add events)"

up-eventing: env
	@$(COMPOSE) --profile eventing up -d
	@echo "postgres         → localhost:$${POSTGRES_PORT:-5432}"
	@echo "redis            → localhost:$${REDIS_PORT:-6379}"
	@echo "kafka            → localhost:$${KAFKA_PORT:-9092}"
	@echo "kafka-ui         → http://localhost:$${KAFKA_UI_PORT:-7070}"
	@echo "schema-registry  → http://localhost:$${SCHEMA_REGISTRY_PORT:-8081}"
	@echo "mailpit UI       → http://localhost:$${MAILPIT_UI_PORT:-8025}"
	@echo "mailpit SMTP     → localhost:$${MAILPIT_SMTP_PORT:-1025}"
	@echo "minio API        → localhost:$${MINIO_API_PORT:-9000}"
	@echo "minio console    → http://localhost:$${MINIO_CONSOLE_PORT:-9001}"

# Wipe every subject in the registry. Schema files on disk are left alone.
schemas-clear:
	@url="$${KAFKA_SCHEMA_REGISTRY_URL:-http://localhost:$${SCHEMA_REGISTRY_PORT:-8081}}"; \
	subjects=$$(curl -fsS "$$url/subjects" 2>/dev/null || echo '[]'); \
	if [ "$$subjects" = '[]' ] || [ -z "$$subjects" ]; then \
	  echo "schema registry empty or not running ($$url)"; \
	  exit 0; \
	fi; \
	echo "$$subjects" | tr -d '[]"' | tr ',' '\n' | while read -r s; do \
	  [ -z "$$s" ] && continue; \
	  echo "deleting subject $$s"; \
	  curl -fsS -X DELETE "$$url/subjects/$$s" >/dev/null || true; \
	  curl -fsS -X DELETE "$$url/subjects/$$s?permanent=true" >/dev/null || true; \
	done; \
	echo "schema registry cleared"

down:
	@$(COMPOSE) --profile eventing down

stop:
	@$(COMPOSE) --profile eventing stop

logs:
	@$(COMPOSE) logs -f $(s)

ps:
	@$(COMPOSE) --profile eventing ps

restart:
	@$(COMPOSE) restart $(or $(s),postgres redis mailpit minio)

# Wipe named volumes. Destructive.
reset:
	@$(COMPOSE) --profile eventing down -v
