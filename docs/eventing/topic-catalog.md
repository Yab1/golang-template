# Kafka topic catalog

This catalog is the source of truth for public event names, routing, and ownership. See [ADR 0001](../adr/0001-kafka-event-driven-architecture.md) for the architecture and compatibility policy.

## Naming

Primary topics use:

```text
<environment>.<bounded-context>.<aggregate>.events.v<major>
```

- `environment`: `dev`, `staging`, or `prod`
- segments: lowercase ASCII words separated by dots
- retries: `<source_topic>.<consumer_name>.retry.v1`
- dead-letter queue: `<source_topic>.<consumer_name>.dlq.v1`

Examples:

```text
prod.identity.user.events.v1
prod.identity.user.events.v1.event-logger.retry.v1
prod.identity.user.events.v1.event-logger.dlq.v1
```

Do not put tenant IDs, user IDs, email addresses, or other identifiers in topic names.

## CloudEvents envelope

Messages are CloudEvents 1.0 structured JSON. Kafka record values are UTF-8 JSON and use content type `application/cloudevents+json`. Producers set:

- Kafka key: aggregate UUID as lowercase canonical text
- `id`: globally unique UUID for this event
- `source`: stable producer URI, currently `/services/api`
- `type`: event type from this catalog
- `subject`: aggregate URN, such as `urn:user:<uuid>`
- `time`: mutation commit time in UTC
- `datacontenttype`: `application/json`
- `dataschema`: repository-relative contract URI, such as `/docs/eventing/schemas/user-registered.v1.schema.json`
- `correlationid`: inbound request ID or workflow ID; generate a UUID when neither exists
- `aggregateversion`: monotonic aggregate version from the producing transaction

The Kafka key and `subject` identify the same aggregate. The envelope schema is [`cloudevents-envelope.v1.schema.json`](./schemas/cloudevents-envelope.v1.schema.json).

## Topics and event types

| Topic pattern | Partition key | Owner | Event types |
|---|---|---|---|
| `<environment>.identity.user.events.v1` | `user_id` | user module | `dev.yab1.golangtemplate.user.registered.v1`, `dev.yab1.golangtemplate.user.activated.v1`, `dev.yab1.golangtemplate.user.password-changed.v1`, `dev.yab1.golangtemplate.user.visibility-changed.v1` |
| `<environment>.content.post.events.v1` | `post_id` | post module | `dev.yab1.golangtemplate.post.created.v1`, `dev.yab1.golangtemplate.post.updated.v1`, `dev.yab1.golangtemplate.post.visibility-changed.v1`, `dev.yab1.golangtemplate.post.deleted.v1` |
| `<environment>.files.file.events.v1` | `file_id` | file module | `dev.yab1.golangtemplate.file.uploaded.v1`, `dev.yab1.golangtemplate.file.deleted.v1` |

`file_id` is the generated opaque object key. It must not be an original client filename or a URL.

## Data minimization

Contracts intentionally exclude:

- email addresses, usernames, passwords, hashes, tokens, and role descriptions
- post titles, bodies, free-form tags, and arbitrary metadata
- original filenames, file bytes, storage URLs, bucket names, and credentials
- IP addresses, user agents, and audit snapshots

Identifiers are pseudonymous, not anonymous. Treat every topic as internal confidential data, restrict ACLs to named producers and consumer groups, and use TLS in transit and broker encryption at rest.

## Publication and consumption

- Publish only after the domain transaction commits, through the transactional outbox.
- Preserve outbox order for each aggregate.
- Validate produced events against their named schema in tests and CI.
- Consumers commit offsets only after successful processing or successful forwarding to the next retry/DLQ topic.
- Consumers deduplicate on event `id`; business writes should also be naturally idempotent where possible.
- Consumers must not infer global ordering across partitions.
