# Local/Remote MariaDB Install and Switch Design

## Summary

Unify first-install and runtime database switching so users choose between local and remote MariaDB before credentials are collected. Remote MariaDB only validates and saves business connection info. Local MariaDB collects business database name, username, and password first, then installs or starts MariaDB, ensures the business database and business user exist, grants privileges, validates the business connection, and finally writes only the business credentials into x-ui settings.

## Requirements

- Fresh install asks whether MariaDB is local or remote before collecting MariaDB credentials.
- Remote MariaDB does not create databases or users.
- Local MariaDB ensures the target database exists, the business user exists, and privileges are granted on the target database.
- Local MariaDB does not persist admin credentials in x-ui config.
- `x-ui.sh` database switching follows the same local/remote behavior.
- Existing SQLite flow remains available.

## Design

### Fresh install

- After `dbType=mariadb`, prompt for `local` or `remote`.
- For `remote`, collect `host`, `port`, `db_name`, `db_user`, and `db_pass`, validate a direct business connection to the target database, then save those values.
- For `local`, collect `db_name`, `db_user`, and `db_pass` first, default to `127.0.0.1:3306`, then ensure local MariaDB is installed and running.
- Local setup first attempts admin access through local root socket. If that fails, prompt for an admin username and password.
- Local setup runs idempotent SQL to create the database, create the business user if missing, and grant privileges on the target database.

### Runtime switch

- `x-ui.sh` keeps the existing MariaDB install/start helpers.
- Add the same local/remote prompt to the SQLite -> MariaDB switch path.
- Remote switch only validates the business connection and saves settings before migration.
- Local switch ensures local MariaDB resources exist before migration.

### Safety

- Store only business credentials in the x-ui JSON config.
- Keep admin credentials in local shell variables only.
- Validate or safely quote database and username identifiers before issuing SQL.

## Testing

- Add a shell-level regression test that checks both scripts expose the new local/remote prompts and helper entry points.
- Run `bash -n install.sh x-ui.sh`.
- Run `go test ./...` to ensure script changes do not break Go-based integration assumptions.
