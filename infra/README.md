# Infra

Third-party services for this template. App code stays under `cmd/` / `internal/`; this folder owns shared dependencies.

## Layout

| File | Role |
|------|------|
| `compose.yml` | Shared project name + `infra` network |
| `compose.postgres.yml` | PostgreSQL |
| `compose.redis.yml` | Redis |
| `compose.kafka.yml` | Kafka + Kafka UI |
| `.env.example` | Defaults (copy to `.env`) |

Secrets / roles via `env_file` + `.env` (Postgres, Redis). Kafka broker settings stay in compose `environment:` (no secrets).

## Usage

```bash
cd infra
cp .env.example .env   # once
make up
make logs s=redis      # optional
make down
make reset             # down + delete volumes
```

| Service | Host |
|---------|------|
| Postgres | `localhost:${POSTGRES_PORT}` |
| Redis | `localhost:${REDIS_PORT}` (password = `REDIS_PASSWORD`) |
| Kafka | `localhost:${KAFKA_PORT}` (from host) / `kafka:9093` (from Docker) |
| Kafka UI | `http://localhost:${KAFKA_UI_PORT}` |

App `.envrc`: set `REDIS_PW` to the same value as `REDIS_PASSWORD`. Schema registry not in this stack yet — worker schema ping still needs a registry or a later compose add-on.
