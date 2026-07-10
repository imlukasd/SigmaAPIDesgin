# Migrations

Migration files live in this directory and are applied in ascending version order.

## Naming

Use this format:

```text
000001_create_users.up.sql
000002_create_organizations.up.sql
000003_create_orders.up.sql
```

Rules:

- Versions are six digits.
- Versions must be unique.
- Use lowercase names with underscores.
- Keep each migration focused on one schema change.
- Prefer forward-only production migrations.

## Commands

Check migration status:

```powershell
go run ./cmd/migrate status
```

Apply pending migrations:

```powershell
go run ./cmd/migrate up
```

The migration runner creates and maintains this table automatically:

```text
schema_migrations
```

## Production Notes

Production migrations should be reviewed like application code. Avoid destructive schema changes without a staged rollout plan.

For example, prefer:

```text
1. Add nullable column
2. Deploy app writing both old and new paths
3. Backfill data
4. Deploy app reading new path
5. Remove old column in a later release
```

Rollback is not implemented in this initial runner because production rollback is often data-dependent. We will discuss rollback strategy separately before adding destructive changes.
