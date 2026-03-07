# Core Schema Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure empty-database deployments succeed by adding a leading migration that creates the core application tables before dependent migrations run.

**Architecture:** Add a new idempotent SQL migration under `Nukara_Backend/migrations/` that mirrors the foundational tables currently only available in deploy-time bootstrap SQL. Because the deploy script already runs migrations alphabetically, this preserves existing deployment flow while fixing the dependency chain for later migrations.

**Tech Stack:** SQL, Bash

---

### Task 1: Add a failing regression check

**Files:**
- Create: `scripts/verify_core_schema_migration.sh`

**Step 1: Write the failing test**

Add a shell verification script that asserts:
- `Nukara_Backend/migrations/001_create_core_tables.sql` exists
- it defines `users`, `bots`, and `conversations`
- the analysis migration still references those tables, proving the dependency is now satisfied by earlier migration order

**Step 2: Run test to verify it fails**

Run: `bash scripts/verify_core_schema_migration.sh`
Expected: FAIL before the new migration exists.

### Task 2: Add the core schema migration

**Files:**
- Create: `Nukara_Backend/migrations/001_create_core_tables.sql`

**Step 1: Write minimal implementation**

Create an idempotent migration for the foundational tables required by downstream migrations, based on the existing deploy bootstrap schema.

**Step 2: Run test to verify it passes**

Run: `bash scripts/verify_core_schema_migration.sh`
Expected: PASS.

### Task 3: Verification

**Files:**
- Verify only

**Step 1: Run verification scripts**

Run: `bash scripts/verify_core_schema_migration.sh && bash scripts/verify_reset_data_flag.sh && bash scripts/verify_deploy_db_permissions.sh`
Expected: PASS.
