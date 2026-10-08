# Infra

Third-party services for this template. App code stays under `cmd/` / `internal/`; this folder owns shared dependencies.

## Layout

| File | Role |
|------|------|
| `compose.yml` | Shared project name + `infra` network |
| `compose.postgres.yml` | PostgreSQL |
| `compose.redis.yml` | Redis |
| `compose.mailpit.yml` | Mailpit (SMTP + UI) |
| `compose.minio.yml` | MinIO (create bucket in console) |
| `compose.kafka.yml` | Kafka + Kafka UI (`profile: eventing`) |
| `compose.schema-registry.yml` | Schema Registry (`profile: eventing`) |
| `.env.example` | Defaults (copy to `.env`) |
| `Jenkinsfile` | Jenkins: Checkout → PRECHECK → env → deploy → POST |
| `scripts/config.sh` | Jenkins env path (`JENKINS_ENV_FILE`) |
| `scripts/deploy.sh` | `make up` / `make up-eventing` + health wait |

Default `make up` = postgres + redis + mailpit + minio.  
Kafka / schema-registry stay off until you need events (`make up-eventing`). Keep `KAFKA_ENABLED=false` until then.

## Usage

```bash
cd infra
cp .env.example .env   # once
make up                # core deps
# make up-eventing     # when you need Kafka + registry
# make schemas-clear   # wipe registry subjects (files on disk stay)
make logs s=redis
make down
make reset             # down + delete volumes
```

| Service | Host | Started by |
|---------|------|------------|
| Postgres | `localhost:${POSTGRES_PORT}` | `make up` |
| Redis | `localhost:${REDIS_PORT}` | `make up` |
| Mailpit UI / SMTP | `${MAILPIT_UI_PORT}` / `${MAILPIT_SMTP_PORT}` | `make up` |
| MinIO | `${MINIO_API_PORT}` / console `${MINIO_CONSOLE_PORT}` | `make up` |
| Kafka | `${KAFKA_PORT}` (host) / `kafka:9093` (Docker) | `make up-eventing` |
| Schema Registry | `${SCHEMA_REGISTRY_PORT}` / `http://schema-registry:8081` | `make up-eventing` |
| Kafka UI | `${KAFKA_UI_PORT}` | `make up-eventing` |

App `.envrc` tips:

- `REDIS_PW` = `REDIS_PASSWORD`
- `DB_ADDR` matches `POSTGRES_*`
- `KAFKA_ENABLED=false` until events exist
- `EVENT_WORKER_ADDR=:8082` (must not share 8081 with the registry)
- SMTP: `MAIL_DRIVER=smtp`, `SMTP_HOST=localhost`, `SMTP_PORT=1025`
- MinIO: create bucket in console once (`MINIO_BUCKET`)

## Jenkins

Job: `infra/Jenkinsfile`. Agent needs Docker + Compose (no Go).

Stages: Checkout → **Prepare Environment** → PRECHECK → Deploy → POST.

1. Put host env on the agent at path from `scripts/config.sh` (default `/var/lib/jenkins/api/golang-template/golang_template_infra.env`) — same keys as `.env.example` (remap ports if 5432/9000/9001 are taken).
2. Pipeline copies that file → `infra/.env` **before** compose validate (services use `env_file: .env`).
3. Parameter **`WITH_EVENTING`** (default false): when true, also starts Kafka + schema-registry.

```bash
# Manual equivalent on the agent
cp /var/lib/jenkins/api/golang-template/golang_template_infra.env infra/.env
./infra/scripts/deploy.sh            # core
./infra/scripts/deploy.sh --eventing # + Kafka stack
```

Run this job **before** `deploy/Jenkinsfile` so Postgres/Redis/etc exist for the API.
