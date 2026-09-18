include .envrc

MIGRATIONS_PATH = ./cmd/migrate/migrations

.PHONY: run
run:
	@air

.PHONY: test
test:
	@go test ./...

.PHONY: gen-docs
gen-docs:
	@swag init -g ./main.go -d cmd/api,internal -o docs --parseDependency --parseInternal && swag fmt

.PHONY: new-module
new-module:
	@test -n "$(name)" || (echo "usage: make new-module name=patient" && exit 1)
	@mkdir -p internal/modules/$(name)
	@printf '%s\n' 'package $(name)' '' 'type $(name) struct {}' > internal/modules/$(name)/model.go
	@printf '%s\n' 'package $(name)' '' 'type Store struct {}' > internal/modules/$(name)/store.go
	@printf '%s\n' 'package $(name)' '' 'type Service struct {}' > internal/modules/$(name)/service.go
	@printf '%s\n' 'package $(name)' '' '// HTTP handlers live here.' > internal/modules/$(name)/handler.go
	@printf '%s\n' 'package $(name)' '' 'import "github.com/go-chi/chi/v5"' '' 'type Module struct {}' '' 'func (m *Module) Routes(r chi.Router) {}' > internal/modules/$(name)/routes.go
	@echo "created internal/modules/$(name)"

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
