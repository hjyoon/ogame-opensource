# Legacy Behavior Surface

Generated from PHP/JS source: 2026-07-14T11:58:46.095Z. Keep this file under 4KB.

| Category | Core | Optional Mods | Total |
| --- | ---: | ---: | ---: |
| Request inputs | 463 | 3 | 466 |
| Action values | 162 | 0 | 162 |
| Queue constants | 19 | 3 | 22 |
| SQL mutation sites | 349 | 41 | 390 |
| Navigation handlers | 149 | 8 | 157 |

## Baseline Gate

The tracked baseline freezes discovered behavior IDs. Any added or removed source
surface fails migration QA until reviewed. Update deliberately with
`bun testing/e2e/audit-legacy-behavior-surface.mjs --update-baseline`.

This catalog is a denominator discovery aid, not proof that every listed item is
functionally migrated. Coverage evidence must be attached through differential,
unit, API, or E2E cases as the registry is expanded.

- Baseline digest: `d6f4f8a245147c82bd4d777e643a87266f9e19237d65fe59d37796c5724dbb35`
- Current digest: `d6f4f8a245147c82bd4d777e643a87266f9e19237d65fe59d37796c5724dbb35`
- Added: 0
- Removed: 0
- Result: PASS
