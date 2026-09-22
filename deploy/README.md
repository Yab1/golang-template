# Deploy

Blue-green API + worker, Jenkins-ready. Infra (Postgres/Redis/Kafka/schema-registry/Mailpit/MinIO) stays in [`../infra`](../infra).

App env keys: root [`.envrc.example`](../.envrc.example). Create **`deploy/.env`** yourself (Compose/`env_file` — no `export`). Same names; inside Docker use hostnames `postgres`, `redis`, `kafka:9093`, `schema-registry:8081` not `localhost`. Set `EVENT_WORKER_ADDR=:8082` (registry owns `:8081`).

## Layout

| Path | Role |
|------|------|
| `Dockerfile` | Multi-stage: `api`, `worker`, `post`, `migrate` |
| `docker-compose.yml` | `api_blue`, `api_green`, `worker`, `proxy` |
| `nginx/app-active.conf.template` | Proxy upstream placeholder |
| `scripts/deploy.sh` | Migrate once → new slot → `/ready` → switch → stop old → worker |
| `scripts/post.sh` | Terminal POST/seed |
| `Jenkinsfile` | Checkout → PRECHECK → env → deploy → POST |

API and worker **never** migrate. Seed **never** runs on API boot. Worker `/ready` checks DB + Kafka + schema registry.

## Host flow

```bash
cd infra && make up
# create deploy/.env  (keys from .envrc.example)
make deploy
make deploy-post
```

Proxy publishes `APP_PORT` (default 8080) → active slot.

## Jenkins

Job: `deploy/Jenkinsfile`. Agent needs Go + golangci-lint for PRECHECK. Copies `JENKINS_ENV_FILE` → `deploy/.env`, then `deploy.sh` then `post.sh`.
