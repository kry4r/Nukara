#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INIT_SQL="$ROOT/Nukara_Backend/deploy/sql/001_init.sql"
MIGRATION_SQL="$ROOT/Nukara_Backend/migrations/007_email_auth_schema.sql"

[[ -f "$INIT_SQL" ]] || { echo "missing init sql: $INIT_SQL"; exit 1; }
[[ -f "$MIGRATION_SQL" ]] || { echo "missing migration: $MIGRATION_SQL"; exit 1; }

if ! rg -n "email\s+VARCHAR" "$INIT_SQL" >/dev/null; then
  echo "users schema does not define email"
  exit 1
fi

if rg -n "phone\s+VARCHAR" "$INIT_SQL" >/dev/null; then
  echo "users schema still defines phone"
  exit 1
fi

if ! rg -n "CREATE TABLE IF NOT EXISTS email_codes" "$INIT_SQL" "$MIGRATION_SQL" >/dev/null; then
  echo "email_codes table definition missing"
  exit 1
fi

echo "email auth schema verified"
