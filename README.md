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

## Auth Configuration

Token and password settings are loaded from environment variables:

```text
AUTH_PASSWORD_BCRYPT_COST=12
AUTH_ACCESS_TOKEN_ISSUER=corebe-api
AUTH_ACCESS_TOKEN_AUDIENCE=corebe-api
AUTH_ACCESS_TOKEN_SECRET=
AUTH_ACCESS_TOKEN_TTL=15m
AUTH_REFRESH_TOKEN_TTL=720h
```

`AUTH_ACCESS_TOKEN_SECRET` must be provided through environment variables or a secret manager before token issuing is wired into runtime code. Do not commit real secrets.

## First endpoints

- `GET /healthz`
- `GET /readyz`
- `GET /v1`
- `GET /v1/payments/capabilities`

## Direction

This scaffold intentionally grows in production-sized stages. Current auth work covers password hashing, registration, and login credential verification; later stages will add token design, validation, rate limit, observability, and payment processing.

## Documentation

- [Project Blueprint](docs/project-blueprint.md)
- [Project Roadmap](docs/project-roadmap.md)
- [Progress Log](docs/progress.md)
- [API Design Notes](docs/api-design.md)
- [Authentication](docs/auth.md)
- [Database](docs/database.md)
- [Schema](docs/schema.md)
- [Testing](docs/testing.md)
- [Pagination](docs/pagination.md)
- [Repository Pattern](docs/repository-pattern.md)


