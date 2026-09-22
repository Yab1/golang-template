# Contributing

## Style

1. Read [docs/style.md](docs/style.md).
2. Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) for Go language conventions.
3. Follow this repo’s **module file layout** and **model = table + `json:"-"`** rules in `docs/style.md` (they override Uber when they conflict).

```bash
make install-tools
make lint
make test
```

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

Infra: `make infra-up` (Postgres, Redis, Kafka, Kafka UI under `infra/`). Eventing: `make kafka-topics && make kafka-schemas` (schema registry still separate until added to `infra/`). Worker: `make worker`. Terminal POST/seed: `make post`. Never seed from API boot.

Checklist: new DB column → migration + model field + store Scan. New event → schema + example + outbox enqueue in the domain transaction.

## PRs

- Keep diffs focused.
- Link Uber or [Code Review Comments](https://go.dev/wiki/CodeReviewComments) when discussing style.
- Do not commit `.envrc` or secrets.
