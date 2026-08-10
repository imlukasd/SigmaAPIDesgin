# CoreBE API

Production-oriented Go API scaffold for a multi-tenant commerce and payment platform.

The project is intentionally built step by step as a production API lab. The goal is to cover the backend concerns that matter in real systems: authentication, authorization, tenant isolation, validation, idempotency, payment workflows, webhooks, rate limiting, observability, auditability, and reliability.

## Run

```powershell
$env:AUTH_ACCESS_TOKEN_SECRET="local-development-secret-must-be-at-least-32-bytes"
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

`AUTH_ACCESS_TOKEN_SECRET` must be provided through environment variables or a secret manager. It must be at least 32 bytes. Do not commit real secrets.

## First endpoints

- `GET /healthz`
- `GET /readyz`
- `GET /v1`
- `POST /v1/auth/register`
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout`
- `GET /v1/auth/me`
- `POST /v1/organizations`
- `GET /v1/organizations/{organization_id}`
- `POST /v1/organizations/{organization_id}/products`
- `GET /v1/organizations/{organization_id}/products/{product_id}`
- `GET /v1/payments/capabilities`

## Direction

This scaffold intentionally grows in production-sized stages. Current auth work covers password hashing, registration, login, refresh token rotation, logout, HTTP handlers, and access token middleware. Current tenancy work covers organization creation, membership authorization, tenant context middleware, protected organization reads, and the tenant data isolation convention. Current catalog work covers product schema, tenant-safe product repository queries, product service validation, and tenant-scoped create/read HTTP routes. Later stages will add product listing, order flows, payments, rate limiting, and observability.

## Documentation

- [Project Blueprint](docs/project-blueprint.md)
- [Project Roadmap](docs/project-roadmap.md)
- [Progress Log](docs/progress.md)
- [API Design Notes](docs/api-design.md)
- [Authentication](docs/auth.md)
- [Tenancy](docs/tenancy.md)
- [Tenant Data Isolation](docs/tenant-data-isolation.md)
- [Catalog](docs/catalog.md)
- [Database](docs/database.md)
- [Schema](docs/schema.md)
- [Testing](docs/testing.md)
- [Pagination](docs/pagination.md)
- [Repository Pattern](docs/repository-pattern.md)


