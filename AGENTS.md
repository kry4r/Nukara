# Repository Guidelines

## Project Structure & Module Organization
`Nukara_Backend/` contains the Go services: entrypoints in `cmd/`, core packages in `internal/`, schema changes in `migrations/`, and helper scripts in `scripts/`. `Nukara_Web/` is the Vue 3 app with source in `src/`, Playwright tests in `tests/`, and deploy assets in `deploy/`. `Nukara_Admin_Web/` is the admin Vue app; API regressions live in `tests/*.spec.mjs`. `Nukara_App/` holds the SwiftUI iOS client, `NukaraTests/`, `NukaraUITests/`, and assets in `Nukara/Assets.xcassets`. `scripts/`, `deploy/`, and `docs/plans/` support local deployment and design tracking.

## Build, Test, and Development Commands
- `cd Nukara_Backend && ./scripts/dev_up.sh`: start Postgres/Redis and run the gateway locally.
- `cd Nukara_Backend && go test ./...`: run backend tests.
- `cd Nukara_Web && npm install && npm run dev`: start the web app with Vite.
- `cd Nukara_Web && npm run build`: verify the web build.
- `cd Nukara_Web && npx playwright test`: run web E2E regressions.
- `cd Nukara_Admin_Web && npm install && npm run build`: verify the admin build.
- `cd Nukara_Admin_Web && node tests/provider-api.spec.mjs`: run direct admin API regression specs.
- `cd Nukara_App && xcodebuild -project Nukara.xcodeproj -scheme Nukara test`: run iOS tests.
- `bash scripts/smoke_full_stack_local.sh`: run the full-stack smoke flow after broad changes.

## Coding Style & Naming Conventions
Use language-native formatting: `gofmt` for Go, Xcode formatting for Swift, and existing 2-space indentation in Vue and plain JS files. Keep Go package names lowercase. Use PascalCase for Vue components and Swift types, and lowerCamelCase for JS utility and store filenames such as `auth.js` or `time.js`. Follow existing directory boundaries rather than mixing backend, web, and app concerns.

## Testing Guidelines
There is no repo-wide coverage threshold. Add targeted regression coverage for each behavior change: Go tests in `*_test.go`, Playwright specs in `Nukara_Web/tests/*.spec.ts`, admin Node specs in `Nukara_Admin_Web/tests/*.spec.mjs`, and iOS tests in `NukaraTests/` or `NukaraUITests/`. Prefer focused test names that describe the scenario under test.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commit prefixes such as `feat:`, `fix:`, and `docs:`, with optional scopes like `feat(store):`. Keep commits narrow and subsystem-specific. PRs should list affected areas, verification commands, schema or env changes, and screenshots for UI work. For larger behavior changes, link the issue and update the relevant note in `docs/plans/`.

## Security & Configuration Tips
Start from `Nukara_Backend/.env.example` and keep secrets out of git. Before running full-stack smoke scripts, set `NUKARA_ADMIN_USERNAME` and `NUKARA_ADMIN_PASSWORD`.
