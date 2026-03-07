# Neo4j Reset Fallback Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `deploy/deploy-local.sh --reset-data` continue one-click deployment even when the existing local Neo4j password is unknown.

**Architecture:** Keep the current application-data reset behavior for PostgreSQL, Redis, and Qdrant. For Neo4j, first try the configured credentials; if authentication/readiness fails, wipe the local Neo4j data/auth directories and allow the later install step to recreate them with the current deployment password.

**Tech Stack:** Bash, systemd, Neo4j cypher-shell

---

### Task 1: Extend regression verification

**Files:**
- Modify: `scripts/verify_reset_data_flag.sh`

**Step 1: Write the failing test**

Require the reset-data verification script to assert:
- a `reset_neo4j_auth_and_data` helper exists
- `reset_neo4j_data` falls back when Neo4j readiness/auth fails
- docs mention the fallback behavior

**Step 2: Run test to verify it fails**

Run: `bash scripts/verify_reset_data_flag.sh`
Expected: FAIL before the fallback is implemented.

### Task 2: Implement Neo4j fallback

**Files:**
- Modify: `deploy/deploy-local.sh`
- Modify: `deploy/lib/memory-infra.sh`

**Step 1: Write minimal implementation**

- make `wait_for_neo4j_ready` return non-zero instead of exiting directly
- keep install-time failure behavior by making the installer call `err` explicitly
- add a helper that stops Neo4j and clears local data/auth state
- call that helper from `reset_neo4j_data` when Neo4j does not accept current credentials

**Step 2: Run test to verify it passes**

Run: `bash scripts/verify_reset_data_flag.sh`
Expected: PASS.

### Task 3: Document one-click server behavior

**Files:**
- Modify: `docs/deployment-guide.md`

**Step 1: Add fallback note**

Document that during `--reset-data`, a Neo4j auth mismatch causes the script to wipe local Neo4j auth/data and continue with a clean reinitialization.

**Step 2: Re-run verification**

Run: `bash scripts/verify_reset_data_flag.sh`
Expected: PASS.

### Task 4: Verification

**Files:**
- Verify only

**Step 1: Run shell checks**

Run: `bash -n deploy/deploy-local.sh deploy/lib/memory-infra.sh scripts/verify_reset_data_flag.sh`
Expected: exit 0.

**Step 2: Re-run regressions**

Run: `bash scripts/verify_reset_data_flag.sh && bash scripts/verify_deploy_db_permissions.sh`
Expected: PASS.
