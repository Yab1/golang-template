#!/usr/bin/env bash
# Scaffold a domain module: model + store + handler + query + module.
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

cat >"$DIR/model.go" <<EOF
package ${NAME}

import "time"

type ${TYPE} struct {
	ID        string     \`json:"id"\`
	CreatedAt time.Time  \`json:"created_at"\`
	UpdatedAt time.Time  \`json:"updated_at"\`
	DeletedAt *time.Time \`json:"-"\`
}
EOF

cat >"$DIR/store.go" <<EOF
package ${NAME}

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}
EOF

cat >"$DIR/handler.go" <<EOF
package ${NAME}
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

cat >"$DIR/module.go" <<EOF
package ${NAME}

import (
	"${MOD}/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Module struct {
	store   *Store
	respond *httpx.Responder
}

func New(db *pgxpool.Pool, respond *httpx.Responder) *Module {
	return &Module{
		store:   NewStore(db),
		respond: respond,
	}
}

func (m *Module) Routes(r chi.Router) {
	r.Route("/${NAME}s", func(r chi.Router) {
	})
}
EOF

echo "created internal/modules/${NAME} {model,store,handler,query,module}.go"
echo "next: register Routes in cmd/api/api.go, add migration, make gen-docs"
