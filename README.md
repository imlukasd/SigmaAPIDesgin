# CoreBE API

Production-oriented Go API scaffold for a multi-tenant commerce and payment platform.

The project is intentionally built step by step as a production API lab. The goal is to cover the backend concerns that matter in real systems: authentication, authorization, tenant isolation, validation, idempotency, payment workflows, webhooks, rate limiting, observability, auditability, and reliability.

## Run

```powershell
go run ./cmd/api
```

## Local Infrastructure

Start PostgreSQL:

```powershell
docker compose up -d postgres
```

The default local database URL is:

```text
postgres://corebe:corebe_dev_password@localhost:5432/corebe?sslmode=disable
```

Run database migrations:

```powershell
go run ./cmd/migrate up
```

## First endpoints

- `GET /healthz`
- `GET /readyz`
- `GET /v1`
- `GET /v1/payments/capabilities`

## Direction

This scaffold intentionally starts small. Later stages will add auth, validation, rate limit, persistence, observability, and payment processing in separate steps.

## Documentation

- [Project Blueprint](docs/project-blueprint.md)
- [Project Roadmap](docs/project-roadmap.md)
- [Progress Log](docs/progress.md)
- [API Design Notes](docs/api-design.md)
- [Database](docs/database.md)
- [Schema](docs/schema.md)
- [Pagination](docs/pagination.md)
- [Repository Pattern](docs/repository-pattern.md)
