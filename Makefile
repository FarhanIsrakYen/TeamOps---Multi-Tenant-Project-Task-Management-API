.PHONY: run test lint fmt migrate-up migrate-down

run:
	docker compose up --build

test:
	cd backend && go test ./...
	cd frontend && npm test -- --run

lint:
	cd backend && go vet ./...
	cd frontend && npm run lint

fmt:
	cd backend && gofmt -w .
	cd frontend && npm run format

migrate-up:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down 1
