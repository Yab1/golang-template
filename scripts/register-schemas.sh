#!/usr/bin/env bash
set -euo pipefail

REGISTRY="${KAFKA_SCHEMA_REGISTRY_URL:-http://localhost:8081}"
SCHEMA_DIR="${EVENT_SCHEMA_DIR:-docs/eventing/schemas}"
ENVELOPE="$SCHEMA_DIR/cloudevents-envelope.v1.schema.json"

for schema in "$SCHEMA_DIR"/*.json; do
  subject="$(jq -r '."x-registry-subject" // empty' "$schema")"
  if [[ -z "$subject" ]]; then
    echo "missing x-registry-subject: $schema" >&2
    exit 1
  fi
  inlined="$(jq -c --slurpfile env "$ENVELOPE" '
    def envelope:
      $env[0]
      | del(."$schema")
      | del(."$id")
      | del(."x-registry-subject");
    (if .allOf then
      .allOf |= map(if has("$ref") then envelope else . end)
    else
      .
    end)
    | del(."$schema")
    | del(."x-registry-subject")
  ' "$schema")"
  payload="$(jq -n --arg schema "$inlined" '{schemaType:"JSON",schema:$schema}')"
  curl --fail --silent --show-error \
    -H 'Content-Type: application/vnd.schemaregistry.v1+json' \
    -X POST \
    --data "$payload" \
    "$REGISTRY/subjects/$subject/versions"
  echo
done
