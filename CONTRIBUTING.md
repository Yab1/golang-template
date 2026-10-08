# Contributing

## Style

Full setup (tools, editor, lint) and every project rule: **[docs/style.md](docs/style.md)**.

Uber Go Guide for language. This repo’s module layout and model=`json:"-"` rules **win** if they conflict with Uber.

```bash
make install-tools
make install-hooks   # once: blocks direct push to main
make lint
make test
```

**CI (GitHub Actions):** On every push and pull request to `main`, [`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs `make lint` and `make test`. Require this workflow to pass before merging (GitHub branch protection).

## Adding a module

```bash
make new-module name=patient
# fill model (full table row + json:"-" for internal)
# fill store (Scan every model column); handler payloads; optional query.go
# migration: make migrate-create create_patients
# register Routes in cmd/api/api.go
make gen-docs
make migrate-up
make lint
```

Infra: `make infra-up` (Postgres, Redis, Mailpit, MinIO). Kafka stack: `make infra-up-eventing` then `make kafka-topics && make kafka-schemas`. Worker: `make worker` (`EVENT_WORKER_ADDR=:8082`). Seed: `make seed` (or `make deploy-seed` after Docker deploy). Never seed from API boot. Blue-green: `make deploy` then `make deploy-seed`. Jenkins: `infra/Jenkinsfile` (deps) then `deploy/Jenkinsfile` (app).

Checklist: new DB column → migration + model field + store Scan. New event → schema + example + outbox enqueue in the domain transaction.

## PRs

- Branch from `main`; no direct push to `main` (see `make install-hooks`).
- Keep diffs focused.
- Run `make lint` and `make test` before opening a PR.
- Link Uber or [Code Review Comments](https://go.dev/wiki/CodeReviewComments) when discussing style.
- Do not commit `.envrc` or secrets.
