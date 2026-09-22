#!/usr/bin/env bash
set -euo pipefail

PREFIX="${KAFKA_TOPIC_PREFIX:-development.golang-template}"
CONSUMER="${EVENT_CONSUMER_NAME:-event-logger}"
PARTITIONS="${KAFKA_TOPIC_PARTITIONS:-6}"
REPLICATION="${KAFKA_TOPIC_REPLICATION_FACTOR:-1}"
BOOTSTRAP="${KAFKA_BOOTSTRAP_SERVER:-kafka:29092}"

sources=(
  "${PREFIX}.identity.user.events.v1"
  "${PREFIX}.content.post.events.v1"
  "${PREFIX}.files.file.events.v1"
)

create_topic() {
  local topic="$1"
  local retention="$2"
  docker compose -f compose.kafka.yml exec -T kafka kafka-topics \
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
