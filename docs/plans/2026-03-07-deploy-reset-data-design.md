# Deploy Reset Data Design

**Problem**

`deploy/deploy-local.sh` can rebuild services, but it cannot explicitly wipe Nukara's existing application data before redeploy. When local state is corrupted or the operator wants a clean environment, they currently need to perform manual PostgreSQL / Redis / Qdrant / Neo4j cleanup outside the deploy flow.

**Scope**

- Add a local deploy flag to reset Nukara application data before redeploy
- Cover PostgreSQL, Redis, Qdrant, and Neo4j data used by the local native deployment
- Keep the reset targeted to Nukara data rather than destroying the underlying services
- Document the new flag and its safety behavior

**Chosen Approach**

1. Add `--reset-data` to `deploy/deploy-local.sh`
2. Require an explicit confirmation in interactive mode; skip prompt only with `--non-interactive`
3. Stop app services before reset so database connections and writers are quiesced
4. Reset PostgreSQL by recreating the `nukara` database
5. Reset Redis by deleting only `nukara:*` keys
6. Reset Qdrant by deleting the configured collection
7. Reset Neo4j by deleting all graph data in the configured database
8. Continue with normal deployment so schema, collections, and provider bootstrap are recreated

**Why This Approach**

- Solves the user need directly in the deployment entrypoint
- Keeps the blast radius limited to Nukara-managed data
- Reuses existing configuration and readiness helpers already present in the deployment scripts
- Makes the destructive action explicit and automatable

**Validation**

- Add a repository verification script that checks the new flag, usage text, confirmation gate, and reset function wiring
- Run the script before implementation to confirm failure
- Run the script after implementation to confirm success
- Run `bash -n` on the changed shell script
- Update deployment documentation with the new flag and behavior
