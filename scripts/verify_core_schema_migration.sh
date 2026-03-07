#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATION="$ROOT_DIR/Nukara_Backend/migrations/001_create_core_tables.sql"
ANALYSIS_MIGRATION="$ROOT_DIR/Nukara_Backend/migrations/004_create_analysis_tables.sql"

[ -f "$MIGRATION" ] || { echo "missing migration: $MIGRATION" >&2; exit 1; }

rg -q 'CREATE TABLE IF NOT EXISTS users' "$MIGRATION" || { echo "users table missing from core migration" >&2; exit 1; }
rg -q 'CREATE TABLE IF NOT EXISTS bots' "$MIGRATION" || { echo "bots table missing from core migration" >&2; exit 1; }
rg -q 'CREATE TABLE IF NOT EXISTS conversations' "$MIGRATION" || { echo "conversations table missing from core migration" >&2; exit 1; }
rg -q 'REFERENCES users\(id\)' "$ANALYSIS_MIGRATION" || { echo "analysis migration no longer references users" >&2; exit 1; }
rg -q 'REFERENCES bots\(id\)' "$ANALYSIS_MIGRATION" || { echo "analysis migration no longer references bots" >&2; exit 1; }
rg -q 'REFERENCES conversations\(id\)' "$ANALYSIS_MIGRATION" || { echo "analysis migration no longer references conversations" >&2; exit 1; }

echo "verify_core_schema_migration: PASS"
