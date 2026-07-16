# Conversation Documentation

Updated: 2026-07-16. Keep this index under 4KB.

This index preserves the work requested across the PHP stabilization and React/Go migration conversation. Historical observations are labeled by date and must not be treated as current defects without reproduction.

## Historical PHP Work

The 2026-06-15 `hjyoon/fix` work covered:

- comparison with `master`, branch updates, commits, and pushes
- Docker environment-driven Master DB and Universe installation
- legacy service startup and public/login route inspection
- direct signup, session creation, and authenticated route classification
- creation of the original login-flow and conversation reports

Details are split to respect the Markdown limit:

- [Setup and branch history](./CONVERSATION_SETUP_2026-06-15.md)
- [Legacy route audit](./CONVERSATION_ROUTES_2026-06-15.md)
- [Original login-flow report](./LOGIN_FLOW_CHECK_REPORT.md)

## React/Go Migration Work

The later `hjyoon/golang` work migrated the discovered non-Mod base product to React 19/Bun 1.3 and Go 1.25. It introduced Clean Architecture, natural CSR routes with legacy aliases, JSON logging, environment bootstrap, MCP/OAuth, and a Go-served React production build.

QA expanded from legacy Docker E2E into PHP/Go snapshot/restore differential checks, source drift audits, API/user-role tests, queue and combat lifecycle checks, and deterministic Chromium/Firefox visual and dynamic comparison.

Current authoritative documents:

- [Migration status](./MIGRATION_STATUS.md)
- [Backend API](./backend/API_ENDPOINTS.md)
- [MCP server](./MCP.md)
- [Migration QA](./testing/e2e/README.md)
- [Absolute coverage model](./testing/e2e/COVERAGE-absolute.md)
- [Modernization backlog](./MODERNIZATION_OPTIONS.md)

## Current Verified State

The 2026-07-16 full wrapper run passed `89/89` groups with no failures or skips. Go internal coverage was 97.0%; 48 PHP/Go differential groups produced 433 passing result cases. Authenticated dynamic behavior passed 98 cases per browser. Navigation exact diff passed 1,947 matched edges and 162 representatives per browser with zero changed pixels.

The precise completion statement is: all currently discovered and registered non-Mod base-product features are migrated and pass the proportional QA inventory. This is not an exhaustive proof of every possible legacy runtime state. Four optional PHP Mods and 37 hooks remain explicitly outside the Go product scope.
