# Deploy

Blue-green API + worker, Jenkins-ready. Infra (Postgres/Redis/Kafka) stays in [`../infra`](../infra).

App env keys: root [`.envrc.example`](../.envrc.example). Create **`deploy/.env`** yourself (Compose/`env_file` — no `export`). Same names; inside Docker use hostnames `postgres`, `redis`, `kafka:9093` not `localhost`.

## Layout

| Path | Role |
|------|------|
| `Dockerfile` | Multi-stage: `api`, `worker`, `post`, `migrate` |
| `docker-compose.yml` | `api_blue`, `api_green`, `worker`, `proxy` |
| `nginx/app-active.conf.template` | Proxy upstream placeholder |
| `scripts/deploy.sh` | Migrate once → new slot → `/ready` → switch → stop old → worker |
| `scripts/post.sh` | Terminal POST/seed |
| `Jenkinsfile` | Checkout → env → deploy → POST |

API and worker **never** migrate. Seed **never** runs on API boot.

## Host flow

```bash
cd infra && make up
# create deploy/.env  (keys from .envrc.example)
make deploy
make deploy-post
```

Proxy publishes `APP_PORT` (default 8080) → active slot.

## Jenkins

Job: `deploy/Jenkinsfile`. Copies `JENKINS_ENV_FILE` → `deploy/.env`, then `deploy.sh` then `post.sh`.
