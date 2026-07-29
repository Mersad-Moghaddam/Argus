# Database migration and rollback guide

Argus migrations are ordered, additive SQL pairs in `db/migrations/`. The
application applies pending `*.up.sql` files during startup and records them in
the schema migration ledger. Review every release's migration set before
deploying to production.

## Fresh installation

1. Provision a dedicated MySQL database and least-privilege application user.
2. Set `MYSQL_DSN` with `parseTime=true` and configure the other required
   service dependencies.
3. Start Argus once; it applies migrations in lexical order.
4. Confirm the application starts successfully and back up the new database
   before loading production traffic.

Do not apply SQL manually out of order. The route, telemetry, private-agent,
and secret-rotation tables rely on their earlier project/environment tables.

## Upgrades

1. Read the new migration files and their matching `*.down.sql` files.
2. Take a restorable, consistency-checked database backup.
3. Test the upgrade on a copy of representative data, including legacy routes
   with request headers and existing telemetry/private-agent records.
4. Deploy one application version and let it apply the ordered migrations.
5. Verify the migration ledger, application health, and the relevant project
   flows before proceeding with a second rollout.

Migrations are additive to preserve existing data. Some changes intentionally
retain a compatibility window—for example canonical route identity dual-write
and encrypted route-header fallback—until an operator completes the associated
backfill/cutover procedure.

## Route-secret encryption migration and key rotation

Set `ROUTE_SECRET_ENCRYPTION_KEY` before writing non-empty route headers. For
legacy plaintext rows, first inspect the operation without writes:

```bash
go run ./cmd/migrate-route-secrets -dry-run
```

Run it without `-dry-run` only after backup and review. To rotate keys, use the
same command's rotation mode with the new active key and the old key available
only for decryption. Its durable key-fingerprinted checkpoints make progress
restartable. Do not remove an old key until the command reports no remaining
records encrypted with it.

## Rollback

Rollback is an operational decision, not an automatic retry. Stop new deploys,
capture the exact migration error, and restore or test against a database copy
before applying a paired down migration. Prefer restoring a verified backup
when the failed migration has already been partially exercised by a newer
application version. Never roll back while other instances can still write a
newer schema.

## Verification boundary

Repository migration-parser tests run in CI and locally with `go test ./...`.
Live fresh-install and representative legacy-upgrade evidence requires an
operator-provided `MYSQL_TEST_DSN`; it was not configured in this workspace,
so this guide does not claim that runtime check has occurred here.
