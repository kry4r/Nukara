# Core Schema Migration Design

**Problem**

A fresh database created by `deploy/deploy-local.sh --reset-data` runs `Nukara_Backend/migrations/*.sql` in filename order. `004_create_analysis_tables.sql` references `users`, `bots`, and `conversations`, but those core tables are not currently defined anywhere inside `Nukara_Backend/migrations/`. They only exist in `Nukara_Backend/deploy/sql/001_init.sql` or the runtime Go bootstrap schema, so first-time deployments fail before services can even start.

**Chosen Approach**

1. Add a new idempotent migration at the start of `Nukara_Backend/migrations/` that creates the core application tables required by later migrations.
2. Base it on the existing foundational schema in `Nukara_Backend/deploy/sql/001_init.sql` so deploy-time SQL and migration-time SQL stay aligned.
3. Keep the deploy script unchanged so any runner that executes `migrations/*.sql` benefits from the fix automatically.

**Why This Approach**

- Fixes the real dependency issue in the migration set rather than patching around it in one script.
- Preserves existing deployment logic and ordering.
- Makes empty-database bootstrap deterministic and idempotent.

**Validation**

- Add a regression script that requires a leading core-schema migration file and checks it defines `users`, `bots`, and `conversations`.
- Run the script before implementation to confirm failure.
- Run it after implementation, along with existing deploy verification scripts.
