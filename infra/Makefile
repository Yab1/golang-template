COMPOSE = docker compose \
	-f compose.yml \
	-f compose.postgres.yml \
	-f compose.redis.yml \
	-f compose.kafka.yml \
	--env-file .env

.PHONY: env up down stop logs ps restart reset

env:
	@test -f .env || cp .env.example .env
	@echo ".env ready"

up: env
	@$(COMPOSE) up -d
	@echo "postgres  → localhost:$${POSTGRES_PORT:-5432}"
	@echo "redis     → localhost:$${REDIS_PORT:-6379}"
	@echo "kafka     → localhost:$${KAFKA_PORT:-9092}"
	@echo "kafka-ui  → http://localhost:$${KAFKA_UI_PORT:-7070}"

down:
	@$(COMPOSE) down

stop:
	@$(COMPOSE) stop

logs:
	@$(COMPOSE) logs -f $(s)

ps:
	@$(COMPOSE) ps

restart:
	@$(COMPOSE) restart $(or $(s),postgres redis kafka kafka-ui)

# Wipe named volumes. Destructive.
reset:
	@$(COMPOSE) down -v
