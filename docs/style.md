# Style and tooling setup

This file is the **project style law** plus **how to set it up**. README is “how to run the app.” This is “how code must look and how lint/editor is wired.”

Language default: **[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)**.  
Offline clone (optional): sibling repo `uber-go/` → `style.md`.

If something is still fuzzy: [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), then [Effective Go](https://go.dev/doc/effective_go).

---

## 1. Authority order

When rules conflict, pick the first that applies:

1. **This document** (module layout, model = table row, eventing, HTTP envelopes)
2. **Uber Go Style Guide**
3. **Go Code Review Comments**
4. **Effective Go** (learning — not day-to-day veto)

Do not re-litigate Uber in PRs. Link the Uber heading. For “where does this file go?” / `json:"-"` / outbox, link **this** file.

---

## 2. Tooling setup (do this once)

### 2.1 Install pinned CLIs

Versions live in the root `Makefile` (`SWAG_VERSION`, `AIR_VERSION`, `MIGRATE_VERSION`, `GOLANGCI_LINT_VERSION`). Do not `go install @latest` for these.

```bash
make install-tools
make tools-versions   # confirm pins
```

Installs:

| Tool | Why |
|------|-----|
| `swag` | OpenAPI from `cmd/api` comments (`make gen-docs`) |
| `air` | Live-reload API (`make run`, config `.air.toml`) |
| `migrate` | Postgres migrations (`make migrate-up`) — **must** be built with `-tags postgres` |
| `golangci-lint` v2 | `make lint` |

`PATH` must include your Go bin dir (`$(go env GOPATH)/bin` or `~/go/bin`).

### 2.2 golangci-lint

Config: [`.golangci.yml`](../.golangci.yml) (version `"2"`).

**Enabled linters:** `errcheck`, `govet`, `ineffassign`, `misspell`, `revive`, `staticcheck`, `unused`, plus the `standard` default set.

**Formatters:** `gofmt` + `goimports`.

`goimports` **local prefix** must match `go.mod`:

```yaml
formatters:
  settings:
    goimports:
      local-prefixes:
        - github.com/Yab1/golang-template
```

`make rename MODULE=…` updates this prefix. If you change `go.mod` by hand, change this too or imports group wrong.

**Revive:** `exported` and `package-comments` are **off** (Uber-style comments, not GoDoc-on-everything). `error-naming`, `error-strings`, `var-naming`, `dot-imports` stay on.

**errcheck excludes:** `zap.SugaredLogger.Sync`, pgx/`database/sql` `Rollback` (already deferred).

**Exclusions:**

- `docs/` — generated swagger; all linters skipped
- `*_test.go` — no `errcheck` / `revive` (stdlib tests still must compile)

Run:

```bash
make lint    # golangci-lint run ./...
```

Timeout: 5m. CI / `scripts/release.sh` PRECHECK runs `go test ./... && make lint`.

### 2.3 Editor (required)

- Format **on save** with **goimports** (not gofmt alone).
- Same local prefix as `.golangci.yml`.
- Go language server (`gopls`) on.
- Tab width 8 for Go (gofmt). Do not retab to 2/4 in `.go` files.
- Do not enable a second formatter that fights golangci (e.g. random prettier on `.go`).

VS Code / Cursor example (`settings.json`):

```json
{
  "go.formatTool": "goimports",
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  },
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package"
}
```

If `goimports` is missing, `make install-tools` does not install it separately — golangci runs it as a formatter. For on-save, install:

```bash
go install golang.org/x/tools/cmd/goimports@latest
```

### 2.4 direnv

```bash
cp .envrc.example .envrc
direnv allow
```

Shell gets `export`’d vars. **Docker Compose does not read `.envrc`.** Deploy uses `deploy/.env` (`KEY=value`, no `export`). One key catalog: `.envrc.example`.

### 2.5 Tests

```bash
make test
```

Conventions:

- Same package as production code (`package post`, not `package post_test`) unless you need to avoid import cycles
- Stdlib `testing` only — no testify
- `t.Parallel()` when the test has no shared mutable globals
- `t.Helper()` on helpers
- `t.Fatal` / `t.Fatalf` with the value you got
- Fakes/interfaces for Kafka/outbox in unit tests; real Postgres only if you add a gated helper later

---

## 3. Uber rules we actually enforce

Full text is upstream. These show up in review here:

| Topic | Do |
|-------|-----|
| Errors | Wrap: `fmt.Errorf("enqueue event: %w", err)`. Sentinel `ErrXxx` / `errXxx`. Handle **once** (log **or** return, not both unless you add context) |
| Error strings | Lowercase, no punctuation at end (`revive` `error-strings`) |
| Interfaces | Accept interfaces, return concrete. Pass interface **by value**, not `*Interface`. `var _ Foo = (*Bar)(nil)` when useful |
| `init()` | Avoid. Wire in `main` / `New` |
| Mutex | Zero value fine. Do not copy a struct after first `Lock` |
| Time | `time` package; store UTC; RFC3339 in APIs/events |
| Slices/maps | Mutable; copy at trust boundaries (handler → store) if you keep the input |
| Channels | Buffer 0 or 1 unless you can justify |
| Imports | Three groups: stdlib / third party / `github.com/Yab1/golang-template/...` (`goimports`) |
| Nesting | Early return. No deep `else` |
| Structs | Named fields on composite literals. Omit obvious zeros |
| Goroutines | Know how they exit (context cancel). No fire-and-forget in handlers |
| Context | First arg `ctx context.Context`. Don’t store ctx on structs |
| Concurrency | Don’t pass mutex-containing structs by value after use |

---

## 4. Project layout (overrides Uber if they fight)

### 4.1 Repository

```
cmd/<process>/           one main per binary
internal/platform/       reusable infra, zero domain types
internal/modules/<name>/ one bounded context
infra/                   third-party compose
deploy/                  app image + blue-green
docs/                    human docs + event schemas
```

**Forbidden dumps:** `pkg/`, `utils/`, `common/`, `helpers/`, `models/` as catch-alls. Uber already hates `utils`/`common` as package names.

`internal/` is private to this module. Other repos do not import it.

### 4.2 Domain module files

Every resource module:

```
internal/modules/<name>/
  model.go      table/domain row
  store.go      SQL only (*Tx when outbox is used)
  handler.go    HTTP payloads, validate, status, call store
  module.go     New + Routes + middleware
  query.go      optional list URL → struct
  events.go     optional transactional outbox (nil-safe if Kafka off)
```

Split extra files **by concern** (`auth.go`, `token_store.go`), never by layer (`controllers/`, `repositories/`).

Scaffold:

```bash
make new-module name=patient
```

Then: migration (stamps, `is_visible`, `metadata`, `version`, soft delete), fill store/handlers, register `Routes` in `cmd/api/api.go`, `make gen-docs`. Sample `post` migration stays **last** (`000006`) so forks can delete it.

### 4.3 Platform

`internal/platform/*` has no `User`/`Post` types. Domain depends on small interfaces (`outbox.Store`, `audit.Logger`, `authz.UserFetcher`), not Kafka client types.

---

## 5. Model = full table row

Non-negotiable for CRUD entities.

1. New DB column → migration + model field + every `Scan`/`SELECT` that loads the row.
2. JSON for clients: `json:"snake_case"`.
3. Internal columns: `json:"-"` (`password`, `deleted_at`, `deleted_by`, `token_version`, …).
4. Request bodies live on the **handler** (`CreateXPayload`). Never embed payloads on the model.
5. If the HTTP shape ≠ row → DTO in the handler. Do not make a second “light” model for the same table.
6. Soft delete: `DeletedAt *time.Time \`json:"-"\`` + `DeletedBy *uuid.UUID \`json:"-"\``; store lists `WHERE deleted_at IS NULL`.
7. Visibility: `IsVisible bool`. Lists hide `is_visible=false`; get-by-id still returns the row so owners can unhide.
8. `Metadata json.RawMessage` → jsonb `{}` default.
9. Stamps: `CreatedBy` / `UpdatedBy` public `omitempty`; set from `authz.Principal` via `stamp.Ptr`. Owner FK (`UserID`) is not the same as “who clicked”.
10. `Version` (or `version`) for optimistic concurrency / event `aggregateversion`.
11. `ReferenceID` human id via `internal/platform/refid` where the module uses public codes (`GTL-USR-…`).

Exceptions: join-heavy read models, FHIR wire types in `fhir_mapping.go`. Core entity stays full-row.

Review question: **“New column? Migration + model + Scan?”**

---

## 6. HTTP

- Chi router. Module `Routes` mounts under `/api/v1`.
- Validate with `httpx.Validate` / playground validator tags on payloads.
- Errors: `httpx.Responder` (`BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `InternalServerError`). Envelope: `{ "error", "request_id" }`.
- Success:
  - one object: `{ status, result, meta.version }` → swagger `httpx.ObjectResponse{result=T}`
  - list: `{ status, results, meta.pagination }` → `httpx.ListResponse{results=[]T}`
- Pagination: `internal/platform/query` — offset (`limit`/`offset`) or cursor (`cursor`, `sort_by=created_at`). Module `query.go` composes parsers. Domain filters stay in the module.
- Auth: `guard.AuthToken`. Writes: `OwnershipOrRole("moderator"|"admin", ownerFn)`.
- After a **successful** mutation: `audit.Record(...)`. Audit failure is logged; **do not** fail the HTTP request.
- Rate limits: don’t invent a second limiter; use `writeLimit` / `authLimit` from `cmd/api`.

Do not put SQL in handlers. Do not put `http.ResponseWriter` in stores.

---

## 7. Transactions and events

When `events != nil` (`KAFKA_ENABLED`):

```go
storage.WithTx(m.db, ctx, func(tx pgx.Tx) error {
    if err := m.store.CreateTx(ctx, tx, row); err != nil {
        return err
    }
    e, err := event.New(...)
    if err != nil {
        return err
    }
    return m.events.Enqueue(ctx, tx, m.eventTopic, row.ID.String(), "aggregate", e, headers)
})
```

Rules:

- Domain write + outbox **same transaction**. Rollback → neither row nor event.
- `events == nil` → store methods without Kafka (plain `Create`).
- Events are **past-tense facts**, not commands.
- Payload: ids + routing facts. No passwords, tokens, email unless a named contract requires it, no file bytes, no post body dump.
- New event: JSON Schema + example under `docs/eventing/` + `event.Type*` / `Schema*` + enqueue + catalog line.
- Audit is **not** a substitute for the outbox.

API never publishes to Kafka directly. Worker never auto-migrates. Seed never in API `main`.

---

## 8. Naming

| Thing | Pattern |
|-------|---------|
| Packages | short, lowercase, singular (`post`, not `posts`) |
| Files | `snake` not used; Go files `query.go`, `store.go` |
| Exported types | `Post`, `Module`, `Store` |
| Handlers | `createPostHandler` |
| Errors | `ErrNotFound`, `errInvalidCursor` |
| Env vars | `SCREAMING_SNAKE` matching `internal/platform/config` |
| SQL | snake_case columns matching JSON |
| Event types | `dev.yab1.golangtemplate.<aggregate>.<fact>.v1` |
| Topics | `<env>.<context>.<aggregate>.events.v1` |

`make rename` updates the Go module path; event type prefix is still in `internal/platform/event/types.go` — change that when you fork if you care.

---

## 9. Comments

- No comments that restate the next line (`// increment i` / `// return error`).
- Comments explain **why**, invariants, or non-obvious constraints (e.g. POST-last, no migrate in API).
- Exported API used from other packages can have a short GoDoc sentence. We do **not** require a comment on every exported symbol (`revive` `exported` disabled).
- Do not comment out blocks of dead code; delete them.

---

## 10. Migrations

- Sequential `cmd/migrate/migrations/NNNNNN_name.{up,down}.sql`.
- App binaries **never** `migrate` on start. Operators / `deploy/scripts/deploy.sh` / `make migrate-up`.
- Eventing tables before sample posts (`000005` then `000006`).
- `down.sql` must actually reverse `up.sql`.
- Data required for **new code correctness** is a migration, not a POST hook.
- POST (`cmd/post`, `deploy/scripts/post.sh`) is seed/backfill/cleanup only, **last** in `scripts/release.sh`.

---

## 11. Security (style-level)

- Secrets only in `.envrc` / `deploy/.env` / `infra/.env` — gitignored.
- Never log tokens, passwords, or raw event PII.
- Event examples in git must stay fake ids.
- `AUTH_DEV_TOKEN` ignored unless `ENV=development`.

---

## 12. PR checklist

- [ ] `make lint` and `make test` clean
- [ ] New column → migration + model + Scan
- [ ] New HTTP field → payload + swagger (`make gen-docs` if public API)
- [ ] New event → schema + example + outbox in the same tx
- [ ] No `utils/` / `pkg/` added
- [ ] Diff focused; Uber/this doc linked if the discussion is style
- [ ] No secrets committed

Keep [CONTRIBUTING.md](../CONTRIBUTING.md) short; **this file** is the detail.
