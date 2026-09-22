#!/usr/bin/env bash
# Scaffold a domain module: model + store + handler + query + events + module.
# Usage: ./scripts/new-module.sh patient
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NAME="${1:-}"
if [[ -z "$NAME" ]]; then
  echo "usage: $0 <name>" >&2
  exit 1
fi

MOD="$(cd "$ROOT" && go list -m)"
DIR="$ROOT/internal/modules/$NAME"
if [[ -e "$DIR" ]]; then
  echo "already exists: $DIR" >&2
  exit 1
fi

mkdir -p "$DIR"

TYPE="$(echo "${NAME}" | awk '{print toupper(substr($0,1,1)) substr($0,2)}')"
# Plural route segment: patient -> patients (simple; fix by hand if irregular).
ROUTE="${NAME}s"
# Short ref code for refid (e.g. patient -> PAT); override in store if needed.
REFCODE="$(echo "${NAME}" | awk '{print toupper(substr($0,1,3))}')"

cat >"$DIR/model.go" <<EOF
package ${NAME}

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ${TYPE} mirrors the table row. Internal cols use json:"-".
type ${TYPE} struct {
	ID          uuid.UUID       \`json:"id"\`
	ReferenceID string          \`json:"reference_id"\`
	Version     int             \`json:"version"\`
	IsVisible   bool            \`json:"is_visible"\`
	Metadata    json.RawMessage \`json:"metadata"\`
	CreatedAt   time.Time       \`json:"created_at"\`
	UpdatedAt   time.Time       \`json:"updated_at"\`
	CreatedBy   *uuid.UUID      \`json:"created_by,omitempty"\`
	UpdatedBy   *uuid.UUID      \`json:"updated_by,omitempty"\`
	DeletedAt   *time.Time      \`json:"-"\`
	DeletedBy   *uuid.UUID      \`json:"-"\`
}
EOF

cat >"$DIR/store.go" <<EOF
package ${NAME}

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"${MOD}/internal/platform/refid"
)

const RefCode = "${REFCODE}"

type Store struct {
	db   *pgxpool.Pool
	refs *refid.Generator
}

func NewStore(db *pgxpool.Pool, refs *refid.Generator) *Store {
	return &Store{db: db, refs: refs}
}

// Add Create/CreateTx, Update/UpdateTx, SoftDelete/SoftDeleteTx so domain
// mutations can share a transaction with outbox.Enqueue when events != nil.
EOF

cat >"$DIR/handler.go" <<EOF
package ${NAME}

// HTTP payloads, validation, status codes, and calls into store / events helpers.
// On mutations: audit.Record + transactional outbox (see events.go).
EOF

cat >"$DIR/query.go" <<EOF
package ${NAME}

import (
	"net/http"

	"${MOD}/internal/platform/query"
)

type ListQuery struct {
	query.Page
	Sort query.Sort
}

func (q ListQuery) Parse(r *http.Request) (ListQuery, error) {
	page, err := query.ParsePage(r)
	if err != nil {
		return q, err
	}
	q.Page = page

	sort, err := query.ParseSort(r, "created_at", "desc", []string{"created_at", "updated_at"})
	if err != nil {
		return q, err
	}
	q.Sort = sort
	return q, nil
}
EOF

cat >"$DIR/events.go" <<EOF
package ${NAME}

// Emit past-tense domain facts in the same DB transaction as the mutation:
//
//	storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
//	    if err := m.store.CreateTx(ctx, tx, row); err != nil { return err }
//	    if m.events == nil { return nil }
//	    e, err := event.New(...)
//	    if err != nil { return err }
//	    return m.events.Enqueue(ctx, tx, m.eventTopic, row.ID.String(), "${NAME}", e, headers)
//	})
//
// Also add:
//   docs/eventing/schemas/<event>.v1.schema.json
//   docs/eventing/examples/<event>.v1.json
//   event.Type* / event.Schema* constants in internal/platform/event/types.go
//   topic wiring in cmd/api (event.Topic(prefix, "<bounded_context>", "${NAME}"))
EOF

cat >"$DIR/module.go" <<EOF
package ${NAME}

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"${MOD}/internal/platform/audit"
	"${MOD}/internal/platform/authz"
	"${MOD}/internal/platform/httpx"
	"${MOD}/internal/platform/outbox"
	"${MOD}/internal/platform/refid"
)

type Module struct {
	db          *pgxpool.Pool
	store       *Store
	respond     *httpx.Responder
	guard       *authz.Guard
	audit       audit.Logger
	log         *zap.SugaredLogger
	events      *outbox.Store
	eventTopic  string
	eventSource string
	writeLimit  func(http.Handler) http.Handler
}

func New(
	db *pgxpool.Pool,
	respond *httpx.Responder,
	guard *authz.Guard,
	refs *refid.Generator,
	writeLimit func(http.Handler) http.Handler,
	auditLog audit.Logger,
	log *zap.SugaredLogger,
	events *outbox.Store,
	eventTopic string,
	eventSource string,
) *Module {
	if writeLimit == nil {
		writeLimit = func(next http.Handler) http.Handler { return next }
	}
	if auditLog == nil {
		auditLog = audit.NewNop()
	}
	return &Module{
		db:          db,
		store:       NewStore(db, refs),
		respond:     respond,
		guard:       guard,
		audit:       auditLog,
		log:         log,
		events:      events,
		eventTopic:  eventTopic,
		eventSource: eventSource,
		writeLimit:  writeLimit,
	}
}

func (m *Module) Routes(r chi.Router) {
	r.Route("/${ROUTE}", func(r chi.Router) {
		// r.With(m.guard.AuthToken, m.writeLimit).Post("/", m.createHandler)
		// r.Get("/", m.listHandler)
	})
}
EOF

echo "created internal/modules/${NAME}/ {model,store,handler,query,events,module}.go"
echo "next:"
echo "  1. migration (stamps, is_visible, metadata, version, soft delete) — keep removable modules last"
echo "  2. fill store Tx methods + handlers; audit.Record on mutations"
echo "  3. if Kafka: schemas/examples + enqueue in events.go; wire New(...) in cmd/api/main.go"
echo "  4. register Routes in cmd/api/api.go; make gen-docs; make lint"
