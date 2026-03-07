# Deploy DB Permission Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make local and Docker deployment defaults consistently use the `nukara` application role so provider-table permissions cannot drift from runtime access.

**Architecture:** Keep bootstrap DDL that requires superuser access under `postgres`, then normalize ownership/grants and execute migrations as `nukara`. Align generated env files and compose defaults so runtime services use the same application role everywhere.

**Tech Stack:** Bash, PostgreSQL, Docker Compose

---

### Task 1: Add regression verification

**Files:**
- Create: `scripts/verify_deploy_db_permissions.sh`

**Step 1: Write the failing test**

Add a shell verification script that asserts:
- `deploy/deploy-local.sh` repairs ownership/grants for old databases
- `deploy/deploy-local.sh` runs migrations as `nukara`
- `deploy/deploy.sh` emits `POSTGRES_USER=nukara`
- Docker/local dev DSNs default to `nukara`

**Step 2: Run test to verify it fails**

Run: `bash scripts/verify_deploy_db_permissions.sh`
Expected: FAIL because current scripts still use `postgres` defaults and lack ownership repair.

### Task 2: Fix local systemd deployment

**Files:**
- Modify: `deploy/deploy-local.sh`

**Step 1: Write minimal implementation**

- Add helper(s) for `postgres` and `nukara` psql execution
- Add ownership/grant repair for existing objects in `public`
- Run migrations as `nukara` with `ON_ERROR_STOP=1`
- Fail deployment immediately if any migration fails

**Step 2: Run test to verify it passes locally**

Run: `bash scripts/verify_deploy_db_permissions.sh`
Expected: local-deploy assertions pass.

### Task 3: Align Docker and local dev defaults

**Files:**
- Modify: `deploy/deploy.sh`
- Modify: `deploy/docker-compose.yml`
- Modify: `Nukara_Backend/configs/docker-compose.dev.yml`
- Modify: `Nukara_Backend/.env.example`

**Step 1: Write minimal implementation**

- Make generated/declarative defaults use `nukara`
- Keep behavior idempotent for existing explicit env overrides

**Step 2: Run test to verify it passes**

Run: `bash scripts/verify_deploy_db_permissions.sh`
Expected: all assertions pass.

### Task 4: Syntax verification

**Files:**
- Verify only

**Step 1: Run syntax checks**

Run: `bash -n deploy/deploy-local.sh deploy/deploy.sh scripts/verify_deploy_db_permissions.sh`
Expected: exit 0 with no output.

**Step 2: Re-run regression verification**

Run: `bash scripts/verify_deploy_db_permissions.sh`
Expected: PASS.

Note: no git commit is included in this session because the higher-priority task instructions forbid committing unless explicitly requested.
