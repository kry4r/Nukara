# Deploy Reset Data Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `--reset-data` mode to the local native deployment so Nukara application data is wiped before a clean redeploy.

**Architecture:** Extend `deploy/deploy-local.sh` with a dedicated destructive flag and a small reset pipeline that reuses the existing deployment configuration. The reset logic will target only Nukara-managed data stores: PostgreSQL database contents, Redis `nukara:*` keys, the configured Qdrant collection, and the configured Neo4j graph contents.

**Tech Stack:** Bash, PostgreSQL, Redis CLI, curl, Neo4j cypher-shell

---

### Task 1: Add a failing regression check

**Files:**
- Create: `scripts/verify_reset_data_flag.sh`

**Step 1: Write the failing test**

Create a shell verification script that asserts:
- `deploy/deploy-local.sh` accepts `--reset-data`
- usage text documents `--reset-data`
- a confirmation helper exists
- a reset helper exists and is invoked from `main`
- documentation mentions the new flag

**Step 2: Run test to verify it fails**

Run: `bash scripts/verify_reset_data_flag.sh`
Expected: FAIL because the flag and helpers do not exist yet.

### Task 2: Implement reset-data behavior

**Files:**
- Modify: `deploy/deploy-local.sh`

**Step 1: Write minimal implementation**

- Parse `--reset-data`
- Stop app services before reset
- Add interactive confirmation unless `--non-interactive` is set
- Recreate PostgreSQL database
- Delete Redis `nukara:*` keys
- Delete configured Qdrant collection
- Delete graph data from the configured Neo4j database
- Call the reset flow before normal deploy rebuilds begin

**Step 2: Run test to verify it passes**

Run: `bash scripts/verify_reset_data_flag.sh`
Expected: PASS.

### Task 3: Document operator behavior

**Files:**
- Modify: `docs/deployment-guide.md`

**Step 1: Add usage and safety notes**

Document:
- what `--reset-data` deletes
- how to combine it with `--force-clean` and `--non-interactive`
- that it is destructive and intended for clean local rebuilds

**Step 2: Re-run verification**

Run: `bash scripts/verify_reset_data_flag.sh`
Expected: PASS.

### Task 4: Syntax verification

**Files:**
- Verify only

**Step 1: Run shell syntax checks**

Run: `bash -n deploy/deploy-local.sh scripts/verify_reset_data_flag.sh`
Expected: exit 0.

**Step 2: Re-run regression verification**

Run: `bash scripts/verify_reset_data_flag.sh`
Expected: PASS.
