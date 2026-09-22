# Style guide

Language style for this template follows the **[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)**.

Do not re-debate Uber rules in PRs. Link the section. If something is ambiguous, also use [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).

Local clone (optional, offline reading): sibling repo `uber-go/` → open `style.md`.

## Authority order

When rules conflict:

1. **This document** (module / API layout for *this* template)
2. **Uber Go Style Guide**
3. **Go Code Review Comments**
4. **Effective Go** (idioms / learning — not day-to-day law)

## Tooling (Uber lint baseline)

Uber recommends: `errcheck`, `goimports`, `revive` (ex-golint), `govet`, `staticcheck`, via **golangci-lint**.

This repo:

```bash
make install-tools   # once
make lint            # golangci-lint run ./...
```

Config: [`.golangci.yml`](../.golangci.yml) — `gofmt` + `goimports` with local prefix `github.com/Yab1/golang-template`.

Editor: format on save with `goimports` (same local prefix).

## Uber rules we care about most (cheat sheet)

Full text stays upstream. These show up often in this codebase:

| Topic | Expectation |
|-------|-------------|
| Errors | Wrap with context; name `ErrXxx` / `errXxx`; handle once |
| Interfaces | Pass by value, not `*Interface`; verify compliance with `var _ I = (*T)(nil)` when useful |
| `init()` | Avoid; prefer explicit wiring in `main` / `New` |
| Mutex | Zero value OK; do not copy after first use |
| Time | Use `time` package; be explicit about location |
| Slices/maps | Treat as mutable; copy at trust boundaries when needed |
| Channels | Buffer size 0 or 1 unless justified |
| Imports | Stdlib / external / local groups (`goimports`) |
| Nesting | Early return; avoid deep `else` |
| Structs | Named fields on init; omit zero fields when clear |
| Goroutines | No fire-and-forget; know how they exit |

## Project rules (this template)

These are **not** in Uber — every module must follow them.

### Layout

```
internal/modules/<name>/
  model.go      # table/domain row
  store.go      # SQL only
  handler.go    # HTTP: payloads, validate, status, call store
  module.go     # New + Routes (+ middleware wiring)
  query.go      # optional: list URL filters → struct for store
```

Extras by concern (see `user/`): `auth.go`, `token_store.go`, … — **not** by layer (`models/`, `utils/`).

### Model = full table row

Enterprise rule: **one pattern every module** — model mirrors the table; hide internals with `json:"-"`.

1. Columns in migration → fields on the model struct.
2. Client-facing → normal `json:"snake_case"`.
3. Internal (`password`, `deleted_at`, `token_version`, …) → `json:"-"`.
4. Store always `SELECT`/`Scan` those columns when loading a row (field alone is not enough).
5. If API response ≠ row someday → response DTO in handler; do not invent a second partial model.

Companion:

- Request bodies stay in **handler** (`CreateXPayload`) — never on the model.
- Soft-delete resources: `DeletedAt *time.Time \`json:"-"\`` + `DeletedBy *uuid.UUID \`json:"-"\`` + filter `deleted_at IS NULL` in store.
- Visibility: `IsVisible bool` — list filters `is_visible`; get-by-id still returns hidden rows so owners can unhide.
- Extensible bag: `Metadata json.RawMessage` (DB `jsonb`, default `{}`).
- Actor stamps: `CreatedBy` / `UpdatedBy` public (`omitempty`); set from `authz.Principal` via `stamp.Ptr` in handlers. Owner (`UserID`) stays separate.
- Audit trail: call `audit.Record` after successful mutations (do not fail the request on audit error).
- Code review: “new column? migration + model + store Scan?”

Break the rule for join-heavy read models / FHIR wire shapes (`fhir_mapping.go`). Core CRUD entity stays full-row.

Example: `post.Post` includes `DeletedAt`/`DeletedBy` with `json:"-"`; list filters soft-deleted + hidden rows.

### Boundaries

- Modules never import each other's stores.
- Cross-module reads → narrow interface declared by the consumer (see `platform/authz.UserFetcher`).
- Platform code stays in `internal/platform/` — no domain knowledge.
- No new top-level `pkg/` or `utils/` dumping ground.

### Query helpers (list URL params)

Shared parsers live in **`internal/platform/query`** — not `common/` / `utils/` (Uber forbids those package names).

Already there:

- `ParsePage` / cursor encode-decode
- `ParseSort`
- `OptionalString`, `ParseCSV`, `ParseUUID`, `ParseBool`, `ParseInt`
- `ListResult[T]` — store return for lists (`Items`, `Total`, `NextCursor`); not the HTTP envelope

Module `query.go` composes parsers into a domain `ListQuery` (search/tags/filters stay module-specific).

Do not invent a mega “filter builder” until a second module needs the same SQL pattern.

## PR review

Prefer linking:

- Uber section (e.g. [Error Wrapping](https://github.com/uber-go/guide/blob/master/style.md#error-wrapping))
- [Code Review Comments](https://go.dev/wiki/CodeReviewComments) for naming / slice / export nits
- This file for module-file or model/json disputes

Run `make lint` before opening a PR.
