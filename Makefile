.PHONY: run test test-race test-integration lint static-analysis fmt migrate-up migrate-down

GOLANGCI_LINT_VERSION ?= v2.13.2

ifeq ($(OS),Windows_NT)
INTEGRATION_TEST_COMMAND = powershell -NoProfile -Command "$$env:TEST_DATABASE_URL='postgres://teamops:teamops@localhost:5432/teamops?sslmode=disable'; $$env:TEST_REDIS_ADDR='localhost:6379'; Set-Location backend; go test -tags=integration ./tests/integration"
else
INTEGRATION_TEST_COMMAND = cd backend && TEST_DATABASE_URL="postgres://teamops:teamops@localhost:5432/teamops?sslmode=disable" TEST_REDIS_ADDR="localhost:6379" go test -race -tags=integration ./tests/integration
endif

run:
	docker compose up --build

test:
	cd backend && go test ./...
	cd frontend && npm test -- --run

test-race:
	cd backend && go test -race ./...

test-integration:
	docker compose up -d --wait postgres redis
	$(INTEGRATION_TEST_COMMAND)

lint:
	cd backend && go vet ./...
	$(MAKE) static-analysis
	cd frontend && npm run typecheck
	cd frontend && npm run lint

static-analysis:
	cd backend && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

fmt:
	cd backend && gofmt -w .
	cd frontend && npm run format

migrate-up:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down 1
