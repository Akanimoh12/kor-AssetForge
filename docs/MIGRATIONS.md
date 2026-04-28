# Database Migrations

This project uses `golang-migrate` to manage database schema changes and version control.

## Migration Structure

Migrations are located in `backend/migrations/sql/`. Each migration consists of two files:
- `XXXX_name.up.sql`: SQL to apply the change.
- `XXXX_name.down.sql`: SQL to roll back the change.

Where `XXXX` is a 4-digit sequence number (e.g., `0001`).

## Running Migrations

You can run migrations using the provided script in the `scripts/` directory.

### Apply all pending migrations
```bash
./scripts/migrate.sh up
```

### Roll back the last migration
```bash
./scripts/migrate.sh down
```

### Roll back multiple migrations
```bash
./scripts/migrate.sh down 3
```

### Check current version
```bash
./scripts/migrate.sh version
```

### Force a version
If a migration fails and the database is marked as "dirty", you may need to force a version after fixing the issue manually.
```bash
./scripts/migrate.sh force 5
```

## Seed Data

Seed data is located in `backend/migrations/sql/seed/`. These are SQL files applied manually (usually during development).

### Apply seed data
```bash
./scripts/migrate.sh seed
```

## Creating New Migrations

To create a new migration, add two files to `backend/migrations/sql/` following the naming convention.

Example:
- `0006_add_new_table.up.sql`
- `0006_add_new_table.down.sql`

## Automated Backups

The `scripts/backup_db.sh` script can be used to backup the database before running migrations in production.

```bash
./scripts/backup_db.sh
```

## Production Strategy

1. **Backup**: Always run `./scripts/backup_db.sh` before applying migrations.
2. **Apply**: Run `./scripts/migrate.sh up`.
3. **Verify**: Check application logs and database state.
4. **Rollback**: If issues occur, run `./scripts/migrate.sh down` to revert the last change.
