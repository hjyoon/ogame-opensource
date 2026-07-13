# Legacy Migration Inventory

Generated from legacy source: 2026-07-13T01:39:13.106Z. Keep this file under 4KB.

| Surface | Discovered | Covered | Missing |
| --- | ---: | ---: | --- |
| Game router keys | 37 | 37 | None |
| Admin router modes | 26 | 26 | None |
| Public PHP entrypoints | 32 | 32 | None |
| Bot API functions | 14 | 14 | None |
| Optional PHP Mods | 4 | policy decision | 37 executable hooks |

## Mod Runtime Decision

The Go server never executes arbitrary PHP. The bundled optional PHP Mods are
legacy-only and are not functionally migrated by this decision. Go must reject
new installation of these Mods; an already-enabled Mod makes full functional
parity unavailable until a native Go adapter exists. This decision closes an
unsafe execution ambiguity, not the absolute migration gap.

## Enforcement

Run `bun testing/e2e/audit-legacy-migration-inventory.mjs`. The audit fails for
unmapped game router keys or Admin modes. Mod hook methods remain listed in the
JSON artifact so native adapter work cannot disappear from the denominator.
