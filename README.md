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

## Production-like Docker startup

1. Copy `.env.example` to `.env` and replace `JWT_SECRET`, database credentials,
   and any deployment-specific settings.
2. Build and start the complete stack:

   ```sh
   docker compose up --build
   ```

3. Open `http://localhost:5173`. Nginx serves the production React build and
   proxies `/api/v1` to the backend. The API is also exposed directly at
   `http://localhost:8080/api/v1` for development tools.

Compose starts PostgreSQL and Redis health checks, runs the one-shot migration
service, waits for the backend readiness probe, and then starts the frontend.
Use `docker compose ps` to inspect service health and `docker compose down` to
stop the stack without deleting database volumes.

If an older local volume predates migration tracking, the migration container
may report that an existing database object already exists. Back up any data
you need first. For a disposable development database only, reset it with
`docker compose down -v` and rerun `docker compose up --build`.

The frontend API URL is runtime-configurable through
`FRONTEND_API_BASE_URL`. It defaults to `/api/v1`, avoiding browser CORS in the
Compose topology. Set it to an absolute HTTPS API URL when the frontend and API
are hosted separately. `FRONTEND_BUILD_API_URL` is only the compile-time
fallback for deployments that do not use the Nginx runtime configuration.

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

- Frontend health: `GET http://localhost:5173/health`
- Liveness: `GET http://localhost:8080/health`
- Readiness: `GET http://localhost:8080/ready`
- Metrics: `GET http://localhost:8080/metrics`
- Swagger UI: `GET http://localhost:8080/docs/`
- OpenAPI contract: `GET http://localhost:8080/openapi.yaml`

The migration container applies `backend/migrations` before the backend starts.
The frontend and backend images both use production stages and container health
checks.

## Local development

Requirements: Go 1.27+, Node 24+, PostgreSQL 17+, and Redis 7+.

Start only the infrastructure and migrations:

```sh
docker compose up -d postgres redis migrate
```

Then run the backend with a development secret of at least 32 characters. On a
POSIX shell:

```sh
cd backend
JWT_SECRET=local-development-secret-at-least-32-chars go run ./cmd/api
```

On PowerShell:

```powershell
cd backend
$env:JWT_SECRET = "local-development-secret-at-least-32-chars"
go run ./cmd/api
```

Run Vite in a second terminal. Its development fallback calls the backend at
`http://localhost:8080/api/v1`; set `VITE_API_URL` before starting Vite to use a
different API.

```sh
cd frontend
npm ci
npm run dev
```

Quality checks:

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

See [architecture](docs/architecture.md), [feature modules and Go dependency injection](docs/modules.md), [database design](docs/database.md), [frontend architecture](docs/frontend.md), [background concurrency](docs/concurrency.md), [observability](docs/observability.md), [testing strategy](docs/testing.md), [security](docs/security.md), and the [API documentation guide](docs/api.md).
