# syntax=docker/dockerfile:1

# migrate does not depend on app source, so this stage stays cached
# until the pin or the Go base image changes.
FROM golang:1.26-bookworm AS migrate
ENV CGO_ENABLED=0
RUN go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out \
 && go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/api ./cmd/worker ./cmd/post

FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl tzdata \
 && addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /out/worker /app/worker
COPY --from=builder /out/post /app/post
COPY --from=migrate /go/bin/migrate /app/migrate
COPY cmd/migrate/migrations /app/migrations

USER app

EXPOSE 8080 8082

CMD ["/app/api"]
