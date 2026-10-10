# Database backups

The `db-backup` Compose service writes a PostgreSQL custom-format dump to this
directory at startup and every 24 hours. It keeps the most recent 14 days by
default; set `BACKUP_RETENTION_DAYS` in the ignored deployment environment file
to change that. Files are created with owner-only permissions.

To restore a dump into an empty database, stop the meowGram server, create the
target database, then run `pg_restore --no-owner --no-privileges --dbname="$DATABASE_URL"
<backup-file>` from a PostgreSQL client container or host with access to the
database. Start the server after restore so its normal startup can check/apply
pending migrations. Verify restores periodically in a separate test database.

These backups are stored on the same host as the Compose deployment. Copy them
to a separate host or protected backup service to recover from host or disk loss;
that destination and its credentials must be configured by the deployment owner.
