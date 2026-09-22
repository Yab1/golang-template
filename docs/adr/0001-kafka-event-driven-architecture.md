# ADR 0001: Kafka event-driven architecture

- Status: Accepted
- Date: 2026-09-22
- Owners: Platform and domain teams

## Context

The API needs durable domain-change notifications for asynchronous consumers without coupling modules to consumer implementations. The initial producers are the user, post, and file modules.

The contract must support independent consumer deployment, replay, duplicate delivery, schema evolution, and failure isolation. Event payloads must not become a second database or leak credentials, personal data, post content, original filenames, or storage URLs.

## Decision

Use Kafka for asynchronous domain events. Producers publish through a transactional outbox committed with the domain mutation; a relay publishes outbox rows to Kafka. This ADR defines the contract only and does not prescribe a Go implementation.

Events use CloudEvents 1.0 structured JSON with `application/cloudevents+json`. The shared envelope and event data contracts are JSON Schema Draft 2020-12 files under [`docs/eventing/schemas`](../eventing/schemas). Event types and topics are listed in the [topic catalog](../eventing/topic-catalog.md).

Key decisions:

- Partition keys are stable aggregate identifiers so changes for one aggregate remain ordered.
- Delivery is at least once. Consumers must be idempotent by CloudEvent `id`.
- Producers never reuse an event `id`, including after a retry.
- Consumers must tolerate duplicate delivery and unrelated aggregate interleaving.
- Events carry identifiers, state needed for routing, and changed-field names. Consumers fetch protected or detailed data through an authorized API when needed.
- Timestamps are UTC RFC 3339 values.
- Retry topics and dead-letter queues isolate poison messages; operational handling is defined in the [runbook](../eventing/runbook.md).

## Compatibility

JSON Schema files are the producer contract. Local validation uses `event.FileValidator`. Runtime workers fetch the same subjects from the schema registry after `make kafka-schemas`.

`x-registry-subject` on each schema file is the Confluent subject name. Additive optional fields may be introduced within a major version.

The major contract version appears in both the event type and topic name.

- Additive, optional fields may be introduced within a major version.
- Consumers must ignore unknown fields in the CloudEvents envelope. Event `data` schemas are strict for producers; consumers should deserialize tolerantly.
- A required field cannot be removed, renamed, narrowed, or given a new meaning within a major version.
- Enum values are contract surface. Adding one requires consumer-readiness review; consumers must handle unknown values safely.
- Breaking changes require a new event type ending in `.v2`, a `.v2` schema, and a `v2` topic. Producers dual-publish during migration.
- Existing schema files are immutable after publication except for clarifications that do not change validation.
- Events already retained in Kafka remain valid against the schema identified by `dataschema`.

## Consequences

Consumers gain replayable, versioned integration contracts and can evolve independently. Producers and consumers must implement idempotency, observability, retention-aware replay, and schema compatibility checks. The outbox and relay add operational components and eventual consistency.

Kafka events are notifications, not authorization grants. A consumer must still enforce access policy before exposing or acting on protected data.
