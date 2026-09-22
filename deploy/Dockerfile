FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && go build -trimpath -ldflags="-s -w" -o /out/post ./cmd/post \
 && go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 \
 && cp /go/bin/migrate /out/migrate

FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl tzdata \
 && addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /out/worker /app/worker
COPY --from=builder /out/post /app/post
COPY --from=builder /out/migrate /app/migrate
COPY cmd/migrate/migrations /app/migrations

USER app

EXPOSE 8080

CMD ["/app/api"]
