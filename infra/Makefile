COMPOSE = docker compose \
	-f compose.yml \
	-f compose.postgres.yml \
	-f compose.redis.yml \
	-f compose.kafka.yml \
	-f compose.schema-registry.yml \
	-f compose.mailpit.yml \
	-f compose.minio.yml \
	--env-file .env

.PHONY: env up down stop logs ps restart reset help

help:
	@echo "infra targets:"
	@echo "  make up       start postgres redis kafka schema-registry mailpit minio"
	@echo "  make down     stop and remove containers"
	@echo "  make stop     stop containers"
	@echo "  make logs s=X follow logs (e.g. s=redis)"
	@echo "  make ps       list containers"
	@echo "  make restart  restart services"
	@echo "  make reset    down + delete volumes (destructive)"

env:
	@test -f .env || cp .env.example .env
	@echo ".env ready"

up: env
	@$(COMPOSE) up -d
	@echo "postgres         → localhost:$${POSTGRES_PORT:-5432}"
	@echo "redis            → localhost:$${REDIS_PORT:-6379}"
	@echo "kafka            → localhost:$${KAFKA_PORT:-9092}"
	@echo "kafka-ui         → http://localhost:$${KAFKA_UI_PORT:-7070}"
	@echo "schema-registry  → http://localhost:$${SCHEMA_REGISTRY_PORT:-8081}"
	@echo "mailpit UI       → http://localhost:$${MAILPIT_UI_PORT:-8025}"
	@echo "mailpit SMTP     → localhost:$${MAILPIT_SMTP_PORT:-1025}"
	@echo "minio API        → localhost:$${MINIO_API_PORT:-9000}"
	@echo "minio console    → http://localhost:$${MINIO_CONSOLE_PORT:-9001}"

down:
	@$(COMPOSE) down

stop:
	@$(COMPOSE) stop

logs:
	@$(COMPOSE) logs -f $(s)

ps:
	@$(COMPOSE) ps

restart:
	@$(COMPOSE) restart $(or $(s),postgres redis kafka kafka-ui schema-registry mailpit minio)

# Wipe named volumes. Destructive.
reset:
	@$(COMPOSE) down -v
