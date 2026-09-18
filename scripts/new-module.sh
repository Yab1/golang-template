#!/usr/bin/env bash
# Scaffold a domain module matching post/user layout: model + store + handler + module.
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

cat >"$DIR/model.go" <<EOF
package ${NAME}

// Resource is the domain model for this module.
type Resource struct{}
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

// HTTP handlers live here. Keep SQL in store.go.
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
		// Wire handlers here, then register Module.Routes in cmd/api/api.go.
	})
}
EOF

echo "created internal/modules/${NAME} {model,store,handler,module}.go"
echo "next: register module.Routes in cmd/api/api.go, add migration, make gen-docs"
