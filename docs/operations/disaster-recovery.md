# Disaster Recovery Proof

`proof/backup-restore.sh --execute` creates a real PostgreSQL custom-format dump,
restores it into a temporary database, and compares selected durable table row
counts. It does not overwrite the source database.

The proof intentionally does not claim a complete semantic recovery, RPO, or RTO
unless the executed artifact records those measurements and assertions.

Raw dump files remain under the ignored proof evidence directory.
