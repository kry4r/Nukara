# Admin Memory Graph + Email Auth Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a real admin memory graph explorer and switch the whole product from phone/SMS auth to email verification with admin-managed SMTP settings.

**Architecture:** Keep existing `user.id` and all business relations intact, but migrate the auth identity field from `phone` to `email` and replace SMS code storage/sending with SMTP-backed email codes. Build the admin memory graph from `memory_items` plus topic links, using `memory_items` as the required source of truth and Neo4j topic expansion only as an optional enhancement.

**Tech Stack:** Go backend, PostgreSQL, Vue 3 admin frontend, Vue 3 web frontend, Swift iOS app, existing admin REST API, Go tests, Node test script.

---

### Task 1: Add failing schema and store regression coverage for email auth

**Files:**
- Modify: `Nukara_Backend/internal/store/store.go`
- Modify: `Nukara_Backend/internal/api/auth_session_test.go`
- Create: `scripts/verify_email_auth_schema.sh`
- Modify: `Nukara_Backend/deploy/sql/001_init.sql`
- Modify: `Nukara_Backend/internal/store/postgres_store.go`

**Step 1: Write the failing schema verification script**

Create `scripts/verify_email_auth_schema.sh` that asserts:
- `users` schema defines `email` and does not define `phone`
- verification-code schema defines `email_codes`
- admin/user queries can still join on unchanged `user_id`

**Step 2: Run script to verify it fails**

Run: `bash scripts/verify_email_auth_schema.sh`
Expected: FAIL before schema changes exist.

**Step 3: Write the failing store/auth compile regression**

Update `Nukara_Backend/internal/api/auth_session_test.go` to create users through email-based store methods instead of phone-based methods.

**Step 4: Run focused test to verify it fails**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run TestAuthUserIDRejectsDeletedUserToken -count=1`
Expected: FAIL because store/auth code still expects phone methods.

### Task 2: Implement email identity + email code storage

**Files:**
- Modify: `Nukara_Backend/internal/store/store.go`
- Modify: `Nukara_Backend/internal/store/postgres_store.go`
- Modify: `Nukara_Backend/deploy/sql/001_init.sql`
- Modify: `Nukara_Backend/scripts/smoke_backend.sh`
- Modify: `Nukara_Backend/scripts/verify_ios_api_contract.sh`
- Modify: `Nukara_Backend/scripts/e2e_full_test.sh`
- Create: `Nukara_Backend/migrations/00x_email_auth_schema.sql` (pick next safe migration number)

**Step 1: Replace phone identity with email in the in-memory store**

Implement:
- `type User struct { Email string }`
- `type EmailCode struct { Email, Purpose, Code string; ExpiresAt time.Time }`
- `FindUserByEmail(email string)`
- `CreateUser(email, nickname string)`
- `SaveEmailCode(email, purpose, code string, ttl time.Duration)`
- `ValidateEmailCode(email, purpose, code string) bool`

**Step 2: Replace phone identity with email in the postgres store**

Update SQL reads/writes so user lookup, creation, and verification-code persistence use `email` consistently.

**Step 3: Add migration and deploy schema changes**

Update bootstrap SQL and add an idempotent migration so empty and existing databases both converge on the email-based schema.

**Step 4: Re-run focused checks**

Run:
- `bash scripts/verify_email_auth_schema.sh`
- `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store ./internal/api -run TestAuthUserIDRejectsDeletedUserToken -count=1`
Expected: PASS.

### Task 3: Add failing backend tests for SMTP-backed email auth handlers

**Files:**
- Modify: `Nukara_Backend/internal/api/server.go`
- Create: `Nukara_Backend/internal/api/email_auth_test.go`
- Create: `Nukara_Backend/internal/mail/smtp.go`
- Create: `Nukara_Backend/internal/mail/smtp_test.go`

**Step 1: Write failing handler tests**

Add tests covering:
- `POST /api/v1/auth/email/send` returns 400 when SMTP config is missing
- `POST /api/v1/auth/register` accepts `{ email, email_code, nickname }`
- `POST /api/v1/auth/login` accepts `{ email, email_code }`
- wrong/expired email code returns 400

**Step 2: Run tests to verify they fail**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestEmailAuth' -count=1`
Expected: FAIL because email auth endpoints and SMTP sender do not exist.

### Task 4: Implement SMTP settings + email auth handlers

**Files:**
- Modify: `Nukara_Backend/internal/api/server.go`
- Modify: `Nukara_Backend/internal/bootstrap/bootstrap.go`
- Modify: `Nukara_Backend/internal/store/store.go`
- Modify: `Nukara_Backend/internal/store/postgres_store.go`
- Create: `Nukara_Backend/internal/mail/smtp.go`
- Create: `Nukara_Backend/internal/mail/smtp_test.go`

**Step 1: Add SMTP config loader and sender**

Create a small mail package that:
- loads `smtp_host`, `smtp_port`, `smtp_username`, `smtp_password`, `smtp_from_email`, `smtp_from_name`, `email_code_ttl_seconds`
- sends verification codes
- sends test mail

**Step 2: Replace `/api/v1/auth/sms/send` with `/api/v1/auth/email/send`**

Server behavior:
- accept `{ email, purpose }`
- validate email + purpose
- generate six-digit code
- persist email code
- send email via SMTP

**Step 3: Update login/register handlers**

Change request payloads to email-based fields while keeping token issuance logic tied to `user.id`.

**Step 4: Run focused backend auth tests**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api ./internal/mail -run 'TestEmailAuth|TestSMTP' -count=1`
Expected: PASS.

### Task 5: Add failing admin backend tests for memory graph and email settings

**Files:**
- Modify: `Nukara_Backend/internal/admin/server.go`
- Create: `Nukara_Backend/internal/admin/email_settings_handler.go`
- Create: `Nukara_Backend/internal/admin/memory_graph_handler.go`
- Create: `Nukara_Backend/internal/admin/email_settings_handler_test.go`
- Create: `Nukara_Backend/internal/admin/memory_graph_handler_test.go`
- Modify: `Nukara_Backend/internal/admin/user_provider_handler.go`

**Step 1: Write failing email-settings tests**

Cover:
- `GET /api/admin/settings/email-auth` returns stored SMTP values
- `PUT /api/admin/settings/email-auth` persists values
- `POST /api/admin/settings/email-auth/test` triggers test mail path

**Step 2: Write failing memory-graph tests**

Cover:
- list users returns email and nickname
- list bots by user ID returns that user’s bots
- memory graph returns `memory` and `topic` nodes plus `memory_topic` edges
- graph falls back to store-only data when Neo4j enhancement is unavailable

**Step 3: Run tests to verify they fail**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/admin -run 'TestEmailSettings|TestMemoryGraph' -count=1`
Expected: FAIL because routes/handlers are missing.

### Task 6: Implement admin backend routes for email settings and memory graph

**Files:**
- Modify: `Nukara_Backend/internal/admin/server.go`
- Create: `Nukara_Backend/internal/admin/email_settings_handler.go`
- Create: `Nukara_Backend/internal/admin/memory_graph_handler.go`
- Modify: `Nukara_Backend/internal/admin/user_provider_handler.go`
- Modify: `Nukara_Backend/internal/store/postgres_agentx_data.go`
- Modify: `Nukara_Backend/internal/store/agentx_data.go`

**Step 1: Add email-settings routes**

Expose:
- `GET /api/admin/settings/email-auth`
- `PUT /api/admin/settings/email-auth`
- `POST /api/admin/settings/email-auth/test`

**Step 2: Add memory-graph routes**

Expose:
- `GET /api/admin/users`
- `GET /api/admin/users/{userId}/bots`
- `GET /api/admin/users/{userId}/bots/{botId}/memory-graph`

**Step 3: Build graph payloads from store data**

Graph builder rules:
- one node per memory item
- one node per unique topic
- one edge per memory-topic relation
- optional topic-topic expansion if Neo4j returns related topics
- empty graph on no data, not an error

**Step 4: Re-run backend admin tests**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/admin -run 'TestEmailSettings|TestMemoryGraph' -count=1`
Expected: PASS.

### Task 7: Add targeted admin frontend regression coverage

**Files:**
- Modify: `Nukara_Admin_Web/src/api/admin.js`
- Modify: `Nukara_Admin_Web/tests/provider-api.spec.mjs`
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/style.css`

**Step 1: Add failing admin API helper assertions**

Extend `provider-api.spec.mjs` (or split into a new adjacent test file) to cover:
- email settings payload normalization
- memory-graph fetch helpers
- user/bot fetch helper paths

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Admin_Web && node tests/provider-api.spec.mjs`
Expected: FAIL before new helpers exist.

### Task 8: Implement admin UI for memory graph and SMTP config

**Files:**
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/api/admin.js`
- Modify: `Nukara_Admin_Web/src/style.css`

**Step 1: Add email-settings state and actions**

Implement UI state for SMTP host/port/username/password/from-address/from-name/TTL/test email, plus save and test-mail actions.

**Step 2: Add user → robot selectors for memory graph**

Implement:
- user search
- user cards
- robot cards for selected user
- memory graph fetch on robot selection

**Step 3: Render graph + node detail panel**

Use a lightweight graph renderer or a simple force-layout/canvas/SVG implementation, but ensure:
- memory nodes show a readable label
- clicking a node shows full content
- graph supports basic pan/zoom or scrollable canvas

**Step 4: Re-run targeted admin frontend test**

Run: `cd Nukara_Admin_Web && node tests/provider-api.spec.mjs`
Expected: PASS.

### Task 9: Add failing web auth tests/checks for email flow

**Files:**
- Modify: `Nukara_Web/src/stores/auth.js`
- Modify: `Nukara_Web/src/views/AuthView.vue`
- Modify: `Nukara_Web/tests/pencil-core-pages.spec.ts` only if a stable auth test belongs there; otherwise document manual verification

**Step 1: Define the smallest regression check**

If no stable automated auth test harness exists, create a focused manual verification checklist in the implementation notes. If an adjacent stable test exists, add a failing assertion that auth view labels and required fields are email-based.

**Step 2: Run the smallest available check**

Run the project-appropriate command for the chosen check.
Expected: FAIL before auth UI is updated.

### Task 10: Implement web email auth flow

**Files:**
- Modify: `Nukara_Web/src/stores/auth.js`
- Modify: `Nukara_Web/src/views/AuthView.vue`
- Modify: `Nukara_Web/src/style.css` only if layout labels need adjustment

**Step 1: Replace store methods with email-based payloads**

Update request helpers to call:
- `POST /api/v1/auth/email/send`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/register`

with `email` and `email_code` payloads.

**Step 2: Replace auth view copy and fields**

Update labels, placeholders, disabled-state validation, and button copy from phone/SMS to email verification.

**Step 3: Run focused verification**

Run the smallest stable web check available; otherwise perform manual verification against the local auth page.
Expected: Email auth UI behaves correctly.

### Task 11: Add failing iOS auth regression coverage

**Files:**
- Modify: `Nukara_App/NukaraTests/NukaraTests.swift`
- Modify: `Nukara_App/Nukara/Core/Protocols/AuthRepositoryProtocol.swift`
- Modify: `Nukara_App/Nukara/Core/Network/APIEndpoint.swift`
- Modify: `Nukara_App/Nukara/Core/Repositories/Mock/MockAuthRepository.swift`

**Step 1: Update or add failing unit expectations**

Change tests and compile-time expectations so auth repository methods require `email` and `emailCode` semantics.

**Step 2: Run the smallest available iOS test/build check**

Run the existing project-appropriate test command if available in the repo; otherwise at least use a compile-oriented check if one is already documented.
Expected: FAIL before implementation because signatures and payload fields still use phone/SMS.

### Task 12: Implement iOS email auth flow

**Files:**
- Modify: `Nukara_App/Nukara/Core/Protocols/AuthRepositoryProtocol.swift`
- Modify: `Nukara_App/Nukara/Core/Repositories/Real/RealAuthRepository.swift`
- Modify: `Nukara_App/Nukara/Core/Repositories/Mock/MockAuthRepository.swift`
- Modify: `Nukara_App/Nukara/Core/Network/APIEndpoint.swift`
- Modify: `Nukara_App/Nukara/App/SessionStore.swift`
- Modify: `Nukara_App/Nukara/Features/Auth/AuthView.swift`
- Modify: `Nukara_App/NukaraTests/NukaraTests.swift`

**Step 1: Update protocol and repository signatures**

Switch all auth methods from phone/SMS parameters to email/email-code parameters.

**Step 2: Update DTOs and endpoints**

Ensure request bodies encode `email` and `email_code`, and the send-code endpoint targets `/api/v1/auth/email/send`.

**Step 3: Update auth UI**

Replace phone field UI with email field UI while preserving existing register/login UX.

**Step 4: Run the chosen iOS regression check again**

Expected: PASS.

### Task 13: Full verification before completion

**Files:**
- Verify only

**Step 1: Backend verification**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store ./internal/api ./internal/admin ./internal/mail ./internal/agentx/memory -count=1`
Expected: PASS.

**Step 2: Admin frontend verification**

Run: `cd Nukara_Admin_Web && node tests/provider-api.spec.mjs`
Expected: PASS.

**Step 3: Schema verification**

Run: `bash scripts/verify_email_auth_schema.sh`
Expected: PASS.

**Step 4: Product verification**

- Admin: save SMTP config, send test email, browse user, select robot, inspect memory node detail
- Web: send email code, register, login
- iOS: validate auth build/test path used in Task 12

**Step 5: Review diff**

Confirm scope is limited to:
- admin memory graph
- admin SMTP settings
- email auth migration
- web/iOS auth field updates
