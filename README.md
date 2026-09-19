# TeamOps

TeamOps is a production-oriented, multi-tenant project and task management portfolio application. It is deliberately a modular monolith: the deployment stays simple while authentication, authorization, persistence, caching, observability, and domain boundaries remain explicit.

## What it demonstrates

- Go/Gin REST API organized into handler, service, repository, DTO, and model boundaries
- PostgreSQL transactions, tenant-scoped queries, optimistic locking, migrations, filtering, sorting, and pagination
- JWT access tokens with opaque, hashed, rotating refresh sessions and reuse-family revocation
- Organization RBAC plus resource-level authorization for projects, tasks, comments, and labels
- Redis caching and fixed-window rate limiting
- Structured logs, request IDs, Prometheus metrics, health/readiness probes, background cleanup, and graceful shutdown
- React/TypeScript/Vite client with Router, TanStack Query, Axios, automatic single-flight token refresh, and protected routes
- Unit and opt-in PostgreSQL integration tests, Docker Compose, and GitHub Actions

## Run locally

1. Copy `.env.example` to `.env` and replace `JWT_SECRET`.
2. Start the stack:

   ```sh
   docker compose up --build
   ```

3. Open `http://localhost:5173`. The API is at `http://localhost:8080/api/v1`.

The root Makefile exposes the common workflows:

```sh
make run
make test
make lint
make fmt
make migrate-up
make migrate-down
```

Operational endpoints:

- Liveness: `GET http://localhost:8080/health`
- Readiness: `GET http://localhost:8080/ready`
- Metrics: `GET http://localhost:8080/metrics`
- Swagger UI: `GET http://localhost:8080/docs/`
- OpenAPI contract: `GET http://localhost:8080/openapi.yaml`

The migration container applies `backend/migrations` before the API starts. Compose health checks prevent dependent services from starting against unavailable infrastructure.

## Development

Requirements: Go 1.27+, Node 24+, PostgreSQL 17+, and Redis 7+.

```sh
cd backend
go test ./...
go vet ./...

cd ../frontend
npm ci
npm run lint
npm test
npm run build
```

Integration tests intentionally require an explicit disposable database:

```sh
cd backend
TEST_DATABASE_URL='postgres://teamops:teamops@localhost:5432/teamops_test?sslmode=disable' \
  go test -tags=integration ./tests/integration
```

The integration test resets the `public` schema, so never point it at a database containing valuable data.

See [architecture](docs/architecture.md), [feature modules and Go dependency injection](docs/modules.md), [database design](docs/database.md), [background concurrency](docs/concurrency.md), [observability](docs/observability.md), [testing strategy](docs/testing.md), [security](docs/security.md), and the [API documentation guide](docs/api.md).
