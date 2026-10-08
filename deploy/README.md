# Deploy

Blue-green API + worker, Jenkins-ready. Infra (Postgres/Redis/Kafka/schema-registry/Mailpit/MinIO) stays in [`../infra`](../infra).

App env keys: root [`.envrc.example`](../.envrc.example). Create **`deploy/.env`** yourself (Compose/`env_file` — no `export`). Same names; inside Docker use hostnames `postgres`, `redis`, `kafka:9093`, `schema-registry:8081` not `localhost`. Set `EVENT_WORKER_ADDR=:8082` (registry owns `:8081`).

## Layout

| Path | Role |
|------|------|
| `Dockerfile` | Multi-stage: separate `migrate` install (cache-friendly), BuildKit module/build caches, `api` / `worker` / `seed` |
| `docker-compose.yml` | `api_blue`, `api_green`, `worker`, `proxy` |
| `nginx/app-active.conf.template` | Proxy upstream placeholder |
| `scripts/deploy.sh` | Migrate when schema is behind → new slot → `/ready` → switch → stop old → worker |
| `scripts/seed.sh` | Seed |
| `Jenkinsfile` | Checkout → env → deploy → POST |

API and worker **never** migrate. Seed **never** runs on API boot. Worker `/ready` checks DB + Kafka + schema registry.

## Host flow

```bash
cd infra && make up
# create deploy/.env  (keys from .envrc.example)
make deploy
make deploy-seed
```

Proxy publishes `APP_PORT` (default 8080) → active slot.

## Jenkins

Job: `deploy/Jenkinsfile`. Lint and test stay on GitHub Actions (`.github/workflows/ci.yml`); require that workflow before merging so `main` is already green. Jenkins copies `JENKINS_ENV_FILE` → `deploy/.env`, then `deploy.sh` then `seed.sh`. `deploy.sh` skips the migrate container when `infra_postgres` `schema_migrations` is already at the latest `*.up.sql` version and not dirty.

After a healthy switch, `deploy.sh` deletes `golang_template:<git sha>` tags that no `golang_template_*` container still uses. It keeps `:latest`, the commit just built, and the stopped slot's image. It does not run `docker image prune` or `docker builder prune` — other jobs share this Docker daemon, and the BuildKit cache is what makes the next build fast.
