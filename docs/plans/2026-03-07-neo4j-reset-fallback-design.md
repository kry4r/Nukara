# Neo4j Reset Fallback Design

**Problem**

`deploy/deploy-local.sh --reset-data` currently assumes the configured `NUKARA_NEO4J_PASSWORD` can authenticate against the existing local Neo4j instance. On a reused server, the real password may be unknown or drifted from the current deployment config, causing the reset flow to abort before the one-click redeploy can continue.

**Chosen Approach**

1. Keep the current fast path: if Neo4j accepts the configured credentials, clear graph data with Cypher.
2. If readiness/authentication fails during `--reset-data`, treat it as a local-state mismatch.
3. Automatically stop Neo4j, wipe local Neo4j data/auth state, and let the later `install_memory_infra` step reinitialize Neo4j with the current deployment password.

**Why This Approach**

- Preserves one-click behavior for dedicated Nukara servers.
- Avoids requiring the operator to know historic Neo4j credentials.
- Reuses the existing Neo4j install flow to recreate auth and database state cleanly.

**Validation**

- Extend `scripts/verify_reset_data_flag.sh` to assert the fallback helper and docs exist.
- Run verification before implementation to confirm failure.
- Run verification and shell syntax checks after implementation.
