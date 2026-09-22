# Infra

Third-party services for this template. App code stays under `cmd/` / `internal/`; this folder owns shared dependencies.

## Layout

| File | Role |
|------|------|
| `compose.yml` | Shared project name + `infra` network |
| `compose.postgres.yml` | PostgreSQL |
| `compose.redis.yml` | Redis |
| `compose.kafka.yml` | Kafka + Kafka UI |
| `compose.schema-registry.yml` | Confluent Schema Registry |
| `compose.mailpit.yml` | Mailpit (SMTP + UI) |
| `compose.minio.yml` | MinIO + bucket init |
| `.env.example` | Defaults (copy to `.env`) |

Secrets / roles via `env_file` + `.env` (Postgres, Redis, MinIO). Kafka / schema-registry broker settings stay in compose `environment:` (no secrets).

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
| Schema Registry | `http://localhost:${SCHEMA_REGISTRY_PORT}` / `http://schema-registry:8081` (Docker) |
| Mailpit UI | `http://localhost:${MAILPIT_UI_PORT}` |
| Mailpit SMTP | `localhost:${MAILPIT_SMTP_PORT}` |
| MinIO API | `localhost:${MINIO_API_PORT}` |
| MinIO Console | `http://localhost:${MINIO_CONSOLE_PORT}` |

App `.envrc` tips:

- `REDIS_PW` = `REDIS_PASSWORD`
- `KAFKA_SCHEMA_REGISTRY_URL=http://localhost:8081`
- `EVENT_WORKER_ADDR=:8082` (must not share 8081 with the registry)
- SMTP: `MAIL_DRIVER=smtp`, `SMTP_HOST=localhost`, `SMTP_PORT=1025`
- MinIO: `STORAGE_DRIVER=minio`, `S3_ENDPOINT=localhost:9000`, `S3_ACCESS_KEY` / `S3_SECRET_KEY` match `MINIO_ROOT_*`, `S3_BUCKET=uploads`
