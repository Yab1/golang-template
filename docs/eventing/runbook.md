# Kafka eventing runbook

This runbook covers the topics in the [topic catalog](./topic-catalog.md).

## Delivery model

Delivery is at least once. A relay may publish the same outbox event more than once, and a consumer may process a record again after a crash. Exactly-once business effects are not assumed.

Every consumer stores the CloudEvent `id` with its result, or uses an equivalent atomic idempotency guard. Seeing an existing successful `id` is a successful no-op. Do not deduplicate by aggregate ID because distinct changes share that ID.

Ordering is guaranteed only within a partition. The aggregate UUID is the record key. A consumer that detects a stale aggregate `version` should no-op it; a gap should trigger bounded retry or reconciliation rather than applying events out of order.

## Retry and DLQ

Retry and DLQ topics are consumer-specific:

- retry: `<source_topic>.<consumer_name>.retry.v1`
- DLQ: `<source_topic>.<consumer_name>.dlq.v1`

Transient failures keep the original Kafka key and unchanged CloudEvent value. Default delayed retry tiers are `1m`, `10m`, then `1h`. Exhausted or permanent failures go to the consumer DLQ.

Kafka does not provide delayed delivery by itself. Retry consumers must honor `x-not-before` before invoking business logic.

Retry metadata belongs in Kafka headers, not the signed contract value:

- `x-retry-attempt`
- `x-source-topic`
- `x-not-before`
- `x-failed-consumer`
- `x-error-class` (`transient` or `permanent`)
- `x-first-failed-at`
- `x-last-failed-at`

Never place exception text, payload fragments, tokens, connection strings, or personal data in headers. Logs may include event ID, type, topic, partition, offset, aggregate ID, attempt, and bounded failure class.

## Failure classification

- Retry: timeout, throttling, broker interruption, lock contention, or temporary dependency failure.
- DLQ immediately: invalid JSON, schema mismatch, unsupported event type/version, impossible invariant, or authorization/configuration denial that needs operator action.
- Stop and alert: suspected credential exposure, personal-data exposure, widespread schema rejection, or sustained outbox relay failure.

## DLQ handling

1. Alert on any DLQ arrival and on retry-rate thresholds.
2. Inspect metadata and sanitized logs; do not copy production payloads into tickets or chat.
3. Correct the consumer, dependency, or producer contract issue.
4. Validate the original unchanged event against the schema named by `dataschema`.
5. Replay to the original primary topic with the original key and CloudEvent `id`. Idempotency protects already-completed effects.
6. Record operator, reason, event IDs, time range, source DLQ, and replay destination in the incident record.

Do not edit an event in place. If bad source data requires correction, publish a new domain event with a new `id`.

## Replay

Before replaying:

- confirm topic retention still covers the requested range
- identify event IDs, partitions, offsets, and consumer group
- pause only the affected consumer group when necessary
- verify the consumer is idempotent and compatible with every schema version in range
- estimate downstream load and rate-limit the replay

Replay into the primary topic when normal ordering matters. Use a dedicated replay consumer group for read-model rebuilds that do not need republishing.

## Operational checks

Monitor:

- outbox oldest-unpublished age and publish failures
- producer error rate and acknowledgement latency
- consumer lag by group and partition
- retry and DLQ ingress by event type and failure class
- schema-validation failures
- duplicate and stale-version counts

Do not use topic retention as the only audit record or backup. Retention, partition count, replication factor, and minimum in-sync replicas are environment configuration and must be reviewed before production changes.
