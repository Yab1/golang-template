include .envrc

MIGRATIONS_PATH = ./cmd/migrate/migrations

# Pinned CLI tools (install with: make install-tools)
SWAG_VERSION           ?= v1.16.6
AIR_VERSION            ?= v1.63.0
MIGRATE_VERSION        ?= v4.19.1
GOLANGCI_LINT_VERSION  ?= v2.13.2

.PHONY: run
run:
	@air

.PHONY: worker
worker:
	@go run ./cmd/worker

.PHONY: post
post:
	@go run ./cmd/post

.PHONY: infra-up
infra-up:
	@$(MAKE) -C infra up

.PHONY: infra-down
infra-down:
	@$(MAKE) -C infra down

.PHONY: kafka-up
kafka-up: infra-up

.PHONY: kafka-down
kafka-down: infra-down

.PHONY: kafka-topics
kafka-topics:
	@./scripts/kafka-topics.sh

.PHONY: kafka-schemas
kafka-schemas:
	@./scripts/register-schemas.sh

.PHONY: kafka-replay
kafka-replay:
	@test -n "$(topic)" || (echo "usage: make kafka-replay topic=<dlq-topic> [limit=100]" && exit 1)
	@go run ./cmd/replay -topic "$(topic)" -limit "$(or $(limit),0)"

.PHONY: release
release:
	@./scripts/release.sh

.PHONY: deploy
deploy:
	@./deploy/scripts/deploy.sh

.PHONY: deploy-post
deploy-post:
	@./deploy/scripts/post.sh

.PHONY: deploy-status
deploy-status:
	@./deploy/scripts/orchestrate.sh status

.PHONY: test
test:
	@go test ./...

.PHONY: lint
lint:
	@golangci-lint run ./...

.PHONY: install-tools
install-tools:
	@echo "installing swag@$(SWAG_VERSION)"
	@go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	@echo "installing air@$(AIR_VERSION)"
	@go install github.com/air-verse/air@$(AIR_VERSION)
	@echo "installing migrate@$(MIGRATE_VERSION) (postgres)"
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)
	@echo "installing golangci-lint@$(GOLANGCI_LINT_VERSION)"
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "tools ready: swag air migrate golangci-lint"

.PHONY: tools-versions
tools-versions:
	@echo "swag=$(SWAG_VERSION)"
	@echo "air=$(AIR_VERSION)"
	@echo "migrate=$(MIGRATE_VERSION)"
	@echo "golangci-lint=$(GOLANGCI_LINT_VERSION)"

.PHONY: gen-docs
gen-docs:
	@swag init -g ./main.go -d cmd/api,internal -o docs --parseDependency --parseInternal && swag fmt

.PHONY: rename
rename:
	@test -n "$(MODULE)" || (echo "usage: make rename MODULE=github.com/acme/myapp" && exit 1)
	@./scripts/rename-module.sh "$(MODULE)"

.PHONY: new-module
new-module:
	@test -n "$(name)" || (echo "usage: make new-module name=patient" && exit 1)
	@./scripts/new-module.sh "$(name)"

.PHONY: migrate-create
migrate-create:
	@migrate create -seq -ext sql -dir $(MIGRATIONS_PATH) $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_ADDR) up

.PHONY: migrate-down
migrate-down:
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_ADDR) down $(filter-out $@,$(MAKECMDGOALS))

# Allow: make migrate-create create_users
%:
	@:
