# Deploy DB Permission Fix Design

**Problem**

`deploy/deploy-local.sh` creates the `nukara` database and runtime DSN for the `nukara` role, but applies migrations as `postgres`. That makes newly created tables owned by `postgres`, which can leave the runtime role without table privileges and breaks admin readiness on `/api/admin/providers` with `pq: permission denied for table providers (42501)`.

**Scope**

- Fix local systemd deployment in `deploy/deploy-local.sh`
- Align Docker-based deployment defaults in `deploy/deploy.sh` and `deploy/docker-compose.yml`
- Align local backend dev entry points that currently mix `postgres` and `nukara` DSNs

**Chosen Approach**

1. Keep role/database creation under `postgres`
2. Repair schema ownership and grants for existing local databases
3. Run migrations as `nukara` with `ON_ERROR_STOP=1`
4. Change Docker/local dev defaults so the application role is consistently `nukara`

**Why This Approach**

- Fixes the current broken local environment
- Prevents future redeploys from recreating the same owner mismatch
- Keeps runtime privileges least-privileged instead of using `postgres`
- Minimizes behavior changes outside DB bootstrap and DSN defaults

**Validation**

- Add a repository verification script that asserts the intended ownership/grant strategy is present
- Run the verification script before changes to confirm failure
- Run the verification script after changes to confirm success
- Run `bash -n` on changed shell scripts for syntax validation
