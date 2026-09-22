# Golang Template

Enterprise-style Go HTTP API you clone and rename. Vertical domain modules, JWT access+refresh, RBAC, Postgres, optional Redis/Kafka, files, mail, audit, Prometheus.

## Prerequisites

- Go 1.26+
- Docker & Docker Compose
- [direnv](https://direnv.net/) (loads `.envrc` into the shell)
- `make`

```bash
make install-tools   # swag, air, migrate (postgres), golangci-lint
make install-hooks   # once: blocks direct push to main
```

## Environment Variables

Copy [`.envrc.example`](.envrc.example) to `.envrc`, fill secrets, then `direnv allow`.

Deploy uses **`deploy/.env`**: same keys, no `export`, Docker DNS (`postgres`, `redis`, `kafka:9093`, `schema-registry:8081`). You create that file; there is no second example. Set `EVENT_WORKER_ADDR=:8082` so it does not collide with the schema registry on `:8081`.

Infra brokers: copy [`infra/.env.example`](infra/.env.example) to `infra/.env`. `REDIS_PW` in the app env must match `REDIS_PASSWORD` there.

## Quick Start

Commands live in the [Makefile](Makefile) — run `make help`.

API and worker **do not** run migrations on boot. Seed **never** runs on API boot.

### Option A: Run on the host

Infra in Docker. API on your machine.

```bash
cd infra && cp .env.example .env && make up && cd ..

cp .envrc.example .envrc
# set AUTH_TOKEN_SECRET, SEED_ADMIN_PASSWORD, REDIS_PW, …
direnv allow

make migrate-up
make run          # air → cmd/api, default :8080
```

- Live: http://localhost:8080/live
- Ready: http://localhost:8080/ready
- Swagger: http://localhost:8080/docs

Optional:

```bash
make post         # seed admin (SEED_ENABLED=true)
make kafka-topics && make kafka-schemas
make worker       # Kafka outbox/inbox (KAFKA_ENABLED=true)
```

### Option B: Run with Docker (deploy stack)

Blue-green API + worker. Infra still from `infra/`. See **[deploy/README.md](deploy/README.md)**.

```bash
cd infra && make up && cd ..
# create deploy/.env  (keys from .envrc.example; docker hostnames)
make deploy
make deploy-post   # seed last
```

## Contributing

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for style, new modules, and PRs. Run `make lint` and `make test` before you open a PR.

**CI (GitHub Actions):** On every push and pull request to `main`, the [CI workflow](.github/workflows/ci.yml) runs `make lint` and `make test`. Require this workflow to pass before merging (GitHub branch protection).

```bash
make rename MODULE=github.com/acme/myapp
```

Drop sample posts when forking: remove `internal/modules/post`, `000006_create_posts`, and wiring in `cmd/api`. Keep `000005_eventing`.

## Documentation

Project docs (style, architecture, eventing) live in **[docs/](docs/)**. Start at [docs/README.md](docs/README.md).

## Deployment

Deployment assets live in **[deploy/](deploy/)**:

- **[Dockerfile](deploy/Dockerfile)** and **[docker-compose.yml](deploy/docker-compose.yml)** for containerized runs
- **[Jenkinsfile](deploy/Jenkinsfile)** for CI/CD (PRECHECK → deploy → POST)
- **[scripts/](deploy/scripts/)** for build and deploy (`deploy.sh`, `post.sh`, `orchestrate.sh`, `config.sh`)

Shared dependencies (Postgres, Redis, Kafka, Schema Registry, Mailpit, MinIO) live in **[infra/](infra/)**. See the [infra README](infra/README.md) and [deploy README](deploy/README.md).

## Contact

For technical questions or contributions, contact:

- **Developer**: [Yeabsera](https://github.com/Yab1)

## License

Apache 2.0. See the [LICENSE](LICENSE) file for details.
