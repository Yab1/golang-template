# Architecture

Go layout follows [Organizing a Go module](https://go.dev/doc/modules/layout) and the proverb **a little copying is better than a little dependency**. Organize by **responsibility / domain**, not by layer type (`models/`, `utils/`, `common/`, `pkg/`).

```
cmd/api                 HTTP process: bootstrap, mount, health
cmd/migrate             schema files only
internal/platform       infrastructure, no domain knowledge
  config db redis ratelimiter httpx storage query authn authz blob
  mailer logger metrics refid
internal/modules        one vertical slice per domain concept
  user/ post/ file/     (later: clinical/patient, fhir/observation)
```

## Rules

- Each module owns its **model + SQL + HTTP**.
- Modules never import each other's stores. Cross-module reads go through a **narrow interface the consumer declares** (see `platform/authz.UserFetcher`).
- FHIR wire shapes belong in a module's `fhir_mapping.go`. They are not DB models.
- `internal/` stays private to this module. Do not invent a `pkg/` folder.

## Env-driven features

Copy `.envrc.example` → `.envrc`, then `direnv allow`. Flip flags without code changes.

| Flag | Default | Effect |
|------|---------|--------|
| `REF_PREFIX` | first 3 of `APP_NAME` | Platform part of human ids (`GTL-USR-A7K2M`) |
| `AUTH_REQUIRED` | true | `false` skips JWT; still needs `AUTH_DEV_USER_ID` for writes |
| `RBAC_ENABLED` | true | `false` skips role/ownership checks after auth |
| `AUTH_TOKEN_EXP` | 15m | Access JWT lifetime |
| `AUTH_REFRESH_EXP` | 168h | Refresh JWT lifetime |
| `AUTH_DEV_TOKEN` | empty | `Authorization: Bearer <token>` impersonates `AUTH_DEV_USER_ID`. **Ignored unless `ENV=development`** |
| `AUTH_DEV_USER_ID` | empty | Real user UUID used by dev-token / auth-off |
| `RATE_LIMIT_ENABLED` | true | Redis limiter on/off |
| `REDIS_ENABLED` | true | Skip Redis connect if false |
| `PUBLIC_BASE_URL` | http://localhost:8080 | Swagger host/schemes + public links |
| `SWAGGER_ENABLED` | true | `/docs` UI |
| `SWAGGER_CONTACT_*` | Yeabsera / yeabsera.dev@gmail.com | Contact block in OpenAPI |
| `SWAGGER_TITLE` / `DESCRIPTION` / `VERSION` | see `.envrc.example` | OpenAPI info |
| `CORS_ENABLED` | true | CORS middleware |
| `FILES_ENABLED` | true | `/api/v1/files` |
| `STORAGE_DRIVER` | local | `local` \| `s3` \| `minio` |
| `MAIL_ENABLED` | true | Welcome mail on register |
| `MAIL_DRIVER` | log | `log` \| `smtp` \| `ses` |
| `SEED_ENABLED` | false | Create admin user on boot if missing |
| `METRICS_ENABLED` | false | `/metrics` Prometheus. Non-dev requires `METRICS_TOKEN` |
| `METRICS_TOKEN` | empty | `X-Metrics-Token` or `Authorization: Bearer` |
| `TRUSTED_PROXIES` | empty | CIDRs allowed to set `X-Forwarded-For` / `X-Real-IP` |
| `LOG_LEVEL` | debug in development | `debug` \| `info` \| `warn` \| `error` |
| `LOG_FORMAT` | console in development | `console` \| `json` |
| `SHUTDOWN_TIMEOUT` | 10s | Drain HTTP on SIGINT/SIGTERM |

### Dev token

1. Register a user, copy `id` (or enable seed and copy logged id).
2. Set `AUTH_DEV_TOKEN=dev-token` and `AUTH_DEV_USER_ID=<uuid>`.
3. Call APIs with `Authorization: Bearer dev-token` (no login).

### Auth tokens

`POST /authentication/token` returns `access_token` (short) + `refresh_token` (long).

| Call | What |
|------|------|
| `POST /authentication/refresh` | body `{ "refresh_token" }` → new pair. Old refresh revoked + Redis jti blacklist. Reuse of a revoked refresh → all sessions killed (`token_version++`) |
| `POST /authentication/logout` | Bearer access + body refresh → revoke that session, blacklist both jtis |
| `POST /authentication/logout-all` | Bearer access → revoke every refresh, bump `token_version` so leftover access dies |

API routes accept **access** tokens only. Refresh JWT in `Authorization` → 401.

Need migration `000004_refresh_tokens`.

### Reference IDs

Human-readable public ids. UUID stays PK.

Format: `{REF_PREFIX}-{MODEL}-{5chars}` e.g. `GTL-USR-A7K2M`, `GTL-PST-9KX2P`.

| Model code | Module |
|------------|--------|
| `USR` | user |
| `PST` | post |

Generated on insert (`internal/platform/refid`). Unique, retry on collision. Frontend shows `reference_id`; posts routes accept UUID **or** reference_id in path.

### Blob storage

- `local`: disk under `STORAGE_LOCAL_DIR`, GET `/api/v1/files/{key}`
- `minio` / `s3`: same S3 client; MinIO uses path-style + `S3_ENDPOINT`

No compose in this repo. Point `REDIS_ADDR`, `S3_ENDPOINT`, `SMTP_HOST` at whatever is already running.

### Mail

Modules call `mailer.Mailer`. Drivers:

| Driver | Use |
|--------|-----|
| `log` | Print to logger (default) |
| `smtp` | Mailpit (`localhost:1025`) or real SMTP. `SMTP_STARTTLS` / `SMTP_TLS` |
| `ses` | AWS SES. Empty `SES_ACCESS_KEY` uses default AWS chain |

Register sends a welcome mail. Failure is logged; HTTP still 201.

Mailpit UI: http://localhost:8025

### Seed admin

`SEED_ENABLED=true` plus email/username/password/role. Idempotent on email. Does not reset password if user already exists.

### Request ID

Chi `X-Request-ID` echoed on every response. Error JSON includes `request_id`. Logs include it too.

### Health

| Path | When 200 |
|------|----------|
| `GET /live` | Process up |
| `GET /ready` | DB ping, and Redis ping if `REDIS_ENABLED` |
| `GET /health` | Same checks as `/ready`, plus flag dump |

Duplicate email/username on register → **409**.

### Trusted proxies

Empty `TRUSTED_PROXIES` → `RemoteAddr` only. Set e.g. `127.0.0.1,10.0.0.0/8` before nginx/caddy so rate-limit keys the client, not the proxy.

### Swagger

`GET /docs` — Swagger UI.

Runtime applies title/description/version/host from env (`PUBLIC_BASE_URL` → host+schemes). Contact/TOS/externalDocs come from `swag` annotations in `cmd/api/main.go` (regen with `make gen-docs`).

UI: deep linking, persist Authorization, filter, tags alpha, collapsed ops (`docExpansion=none`).

Error envelope schema: `httpx.ErrorResponse` (`error` + `request_id`).
Success envelope:
- single: `{ status, result, meta: { version } }` — annotate `httpx.ObjectResponse{result=T}`
- list: `{ status, results, meta: { version, pagination } }` — annotate `httpx.ListResponse{results=[]T}`

`meta.version` defaults to `"1"`. List pagination nests under `meta.pagination` (`total` / `limit` / `offset`).

## Adding a new EMR resource

1. Migration: `make migrate-create create_<resource>` then write `up`/`down` SQL.
2. Module folder: `make new-module name=<resource>`
3. Fill the five files (`model.go`, `store.go`, `service.go`, `handler.go`, `routes.go`).
4. Register `module.Routes(r)` in `cmd/api/api.go` (or `main.go` wiring).
5. `make gen-docs` and `make migrate-up`.

## Rate limiting

Redis sliding-window limiter (`internal/platform/ratelimiter`). Tiers:

| Tier | Default | Key | Redis down |
|------|---------|-----|------------|
| read | 600/min | IP | fail-open |
| write | 120/min | user (JWT) else IP | fail-open |
| auth | 10/min | IP | in-memory fallback |

Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`. 429 includes `Retry-After`.
