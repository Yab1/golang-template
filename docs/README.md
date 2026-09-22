# Docs

Index for project docs. Style and how to run the app stay split: this folder is law and design; the root [README](../README.md) is how to start the processes.

| Doc | What |
|-----|------|
| [style.md](style.md) | Lint/editor setup and every project style rule |
| [architecture.md](architecture.md) | Layout, env flags, auth, pagination, health, rate limit |
| [adr/0001-kafka-event-driven-architecture.md](adr/0001-kafka-event-driven-architecture.md) | Why outbox + Kafka |
| [eventing/topic-catalog.md](eventing/topic-catalog.md) | Event types and topics |
| [eventing/runbook.md](eventing/runbook.md) | Retry, DLQ, replay |
| [eventing/schemas/](eventing/schemas/) | JSON Schema contracts |
| [eventing/examples/](eventing/examples/) | Sample CloudEvents payloads |

Swagger generated into this folder (`docs.go`, `swagger.json`, `swagger.yaml`) by `make gen-docs`. Do not hand-edit those.

Related, outside `docs/`:

- [infra/README.md](../infra/README.md) — Postgres, Redis, Kafka compose
- [deploy/README.md](../deploy/README.md) — blue-green + Jenkins
- [CONTRIBUTING.md](../CONTRIBUTING.md) — PRs and new modules
