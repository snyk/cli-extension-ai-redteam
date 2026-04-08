GOCI_LINT_V?=v1.64.7
PRE_COMMIT_V?=3.8.0

.PHONY: setup
setup: ## Install golangci-lint & pre-commit (Go tools are managed via go.mod tool directives)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOCI_LINT_V}
	uv tool install pre-commit==${PRE_COMMIT_V}
	pre-commit install --hook-type commit-msg --hook-type pre-commit

.PHONY: lint
lint: ## Run golangci linters
	golangci-lint run -v ./...

.PHONY: format
format: ## Format source code based on golangci configuration
	golangci-lint run --fix -v ./...

.PHONY: test
test: ## Run unit tests
	go test -v ./...

.PHONY: generate
generate: ## Run commands described by //go:generate directives within source code
	go generate ./...

.PHONY: test-wizard
test-wizard: ## Run setup wizard frontend unit tests
	cd web && npm ci && npx vitest run

.PHONY: build-wizard
build-wizard: ## Build setup wizard React frontend
	cd web && npm ci && npm run build
	rm -rf internal/wizard/dist && cp -r web/dist internal/wizard/dist

.PHONY: dev-wizard
dev-wizard: ## Run setup wizard frontend (HMR) + Go API server for development
	cd web && npm install --silent
	./scripts/dev-web.sh

.PHONY: wizard
wizard: build-wizard ## Run full production-like setup wizard for testing
	go run ./cmd/develop redteam setup --experimental

.PHONY: auth
auth: ## Authenticate with Snyk (required before running the setup wizard locally)
	go run ./cmd/develop auth
