-include .envrc

MIGRATIONS_PATH = ./cmd/migrate/migrations

# Pinned CLI tools (install with: make install-tools)
SWAG_VERSION           ?= v1.16.6
AIR_VERSION            ?= v1.63.0
MIGRATE_VERSION        ?= v4.19.1
GOLANGCI_LINT_VERSION  ?= v2.13.2

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' Makefile | awk -F'## ' '{ name=$$1; sub(/:.*/,"",name); printf "  %-18s %s\n", name, $$2 }'

.PHONY: run
run: ## Run API with air
	@air

.PHONY: worker
worker: ## Run Kafka event worker
	@go run ./cmd/worker

.PHONY: post
post: ## Run terminal POST/seed on the host
	@go run ./cmd/post

.PHONY: infra-up
infra-up: ## Start infra stack (postgres redis kafka registry mailpit minio)
	@$(MAKE) -C infra up

.PHONY: infra-down
infra-down: ## Stop infra stack
	@$(MAKE) -C infra down

.PHONY: kafka-up
kafka-up: infra-up ## Alias for infra-up

.PHONY: kafka-down
kafka-down: infra-down ## Alias for infra-down

.PHONY: kafka-topics
kafka-topics: ## Create domain + retry + DLQ topics
	@./scripts/kafka-topics.sh

.PHONY: kafka-schemas
kafka-schemas: ## Register JSON schemas with the schema registry
	@./scripts/register-schemas.sh

.PHONY: kafka-replay
kafka-replay: ## Replay a DLQ topic (topic=… [limit=100])
	@test -n "$(topic)" || (echo "usage: make kafka-replay topic=<dlq-topic> [limit=100]" && exit 1)
	@go run ./cmd/replay -topic "$(topic)" -limit "$(or $(limit),0)"

.PHONY: release
release: ## Full release pipeline (POST last)
	@./scripts/release.sh

.PHONY: deploy
deploy: ## Blue-green deploy API (+ worker if Kafka on)
	@./deploy/scripts/deploy.sh

.PHONY: deploy-post
deploy-post: ## Seed in Docker (after deploy)
	@./deploy/scripts/post.sh

.PHONY: deploy-status
deploy-status: ## Show blue-green deploy status
	@./deploy/scripts/orchestrate.sh status

.PHONY: test
test: ## Run go test ./...
	@go test ./...

.PHONY: lint
lint: ## Run golangci-lint
	@golangci-lint run ./...

.PHONY: install-tools
install-tools: ## Install swag air migrate golangci-lint
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
tools-versions: ## Print pinned tool versions
	@echo "swag=$(SWAG_VERSION)"
	@echo "air=$(AIR_VERSION)"
	@echo "migrate=$(MIGRATE_VERSION)"
	@echo "golangci-lint=$(GOLANGCI_LINT_VERSION)"

.PHONY: install-hooks
install-hooks: ## Point git at .githooks (blocks direct push to main)
	@git config core.hooksPath .githooks
	@chmod +x .githooks/pre-push
	@echo "hooks path set to .githooks (pre-push blocks push to main)"

.PHONY: gen-docs
gen-docs: ## Regenerate Swagger docs
	@swag init -g ./main.go -d cmd/api,internal -o docs --parseDependency --parseInternal && swag fmt

.PHONY: rename
rename: ## Rename module path (MODULE=github.com/acme/myapp)
	@test -n "$(MODULE)" || (echo "usage: make rename MODULE=github.com/acme/myapp" && exit 1)
	@./scripts/rename-module.sh "$(MODULE)"

.PHONY: new-module
new-module: ## Scaffold a domain module (name=patient)
	@test -n "$(name)" || (echo "usage: make new-module name=patient" && exit 1)
	@./scripts/new-module.sh "$(name)"

.PHONY: migrate-create
migrate-create: ## Create a new SQL migration pair
	@migrate create -seq -ext sql -dir $(MIGRATIONS_PATH) $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up: ## Apply migrations
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_ADDR) up

.PHONY: migrate-down
migrate-down: ## Roll back migrations
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_ADDR) down $(filter-out $@,$(MAKECMDGOALS))

# Allow: make migrate-create create_users
%:
	@:
