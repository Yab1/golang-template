#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PREFIX="${KAFKA_TOPIC_PREFIX:-development.golang-template}"
CONSUMER="${EVENT_CONSUMER_NAME:-event-logger}"
PARTITIONS="${KAFKA_TOPIC_PARTITIONS:-6}"
REPLICATION="${KAFKA_TOPIC_REPLICATION_FACTOR:-1}"
# Host clients: localhost:9092. Docker network (infra): kafka:9093.
BOOTSTRAP="${KAFKA_BOOTSTRAP_SERVER:-kafka:9093}"
ADMIN_IMAGE="${KAFKA_ADMIN_IMAGE:-apache/kafka:4.0.0}"

sources=(
  "${PREFIX}.identity.user.events.v1"
  "${PREFIX}.content.post.events.v1"
  "${PREFIX}.files.file.events.v1"
)

create_topic() {
  local topic="$1"
  local retention="$2"
  docker run --rm --network infra "$ADMIN_IMAGE" \
    /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server "$BOOTSTRAP" \
    --create \
    --if-not-exists \
    --topic "$topic" \
    --partitions "$PARTITIONS" \
    --replication-factor "$REPLICATION" \
    --config "retention.ms=$retention"
}

for source in "${sources[@]}"; do
  create_topic "$source" 604800000
  create_topic "${source}.${CONSUMER}.retry.v1" 604800000
  create_topic "${source}.${CONSUMER}.dlq.v1" 2592000000
done
