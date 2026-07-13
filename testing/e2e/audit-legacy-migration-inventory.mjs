import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const gameRouter = JSON.parse(await readFile(resolve(root, "game/router.json"), "utf8"));
const adminRouter = JSON.parse(await readFile(resolve(root, "game/pages_admin/admin_router.json"), "utf8"));
const gameRoutes = await readFile(resolve(root, "frontend/src/gameRoutes.ts"), "utf8");
const gameUI = await readFile(resolve(root, "frontend/src/LegacyGameOverview.tsx"), "utf8");
const server = await readFile(resolve(root, "backend/internal/delivery/http/server.go"), "utf8");
const publicManifest = await readFile(resolve(root, "frontend/src/publicRouteManifest.ts"), "utf8");
const botAPI = await readFile(resolve(root, "game/core/botapi.php"), "utf8");
const botRuntime = await readFile(resolve(root, "backend/internal/infrastructure/mysqlgame/bot_runtime.go"), "utf8");

const aliases = new Set([...gameRoutes.matchAll(/\["([^"]+)", "\/game\//g)].map((match) => match[1]));
const routeDecisions = new Map([["pranger", "/game/pranger.php"]]);
const missingRoutes = Object.keys(gameRouter).filter((key) => !aliases.has(key) && !server.includes(routeDecisions.get(key) ?? "\0"));

const adminModes = new Set(["Home", ...[...gameUI.matchAll(/admin\.mode === "([^"]+)"/g)].map((match) => match[1])]);
const missingAdminModes = Object.keys(adminRouter).filter((mode) => !adminModes.has(mode));

const publicEntries = await discoverPublicEntries();
const publicDecisions = new Set(["/ip.php", "/stat.php", "/game/install.php"]);
const missingPublicEntries = publicEntries.filter((path) =>
  !server.includes(`"${path}"`) && !publicManifest.includes(`"${path}"`) && !gameRoutes.includes(`"${path}"`) && !publicDecisions.has(path)
);

const botFunctions = [...botAPI.matchAll(/function\s+(Bot[A-Za-z0-9_]+)/g)].map((match) => match[1]);
const missingBotFunctions = botFunctions.filter((name) => !botRuntime.includes(`case "${name}"`));

const modFiles = ["BogusMod", "DeepSpaceHorror", "GalaxyTool", "SpaceStorm"];
const modMethods = [];
for (const mod of modFiles) {
  const source = await readFile(resolve(root, `game/mods/${mod}/main.php`), "utf8");
  for (const match of source.matchAll(/public function\s+([A-Za-z0-9_]+)/g)) {
    if (!["install", "uninstall", "init"].includes(match[1])) modMethods.push(`${mod}.${match[1]}`);
  }
}
const modDecision = "legacy_php_runtime_excluded_from_go";
const report = {
  pass: missingRoutes.length === 0 && missingAdminModes.length === 0 && missingPublicEntries.length === 0 && missingBotFunctions.length === 0,
  generatedAt: new Date().toISOString(),
  routes: { total: Object.keys(gameRouter).length, covered: Object.keys(gameRouter).length - missingRoutes.length, missing: missingRoutes },
  adminModes: { total: Object.keys(adminRouter).length, covered: Object.keys(adminRouter).length - missingAdminModes.length, missing: missingAdminModes },
  publicEntries: { total: publicEntries.length, covered: publicEntries.length - missingPublicEntries.length, missing: missingPublicEntries },
  botFunctions: { total: botFunctions.length, covered: botFunctions.length - missingBotFunctions.length, missing: missingBotFunctions },
  modRuntime: { mods: modFiles.length, hooks: modMethods.length, decision: modDecision, methods: modMethods }
};

const jsonPath = resolve(root, ".tmp/legacy-migration-inventory.json");
const markdownPath = resolve(root, "testing/e2e/COVERAGE-legacy-inventory.md");
await mkdir(dirname(jsonPath), { recursive: true });
await writeFile(jsonPath, JSON.stringify(report, null, 2));
await writeFile(markdownPath, renderMarkdown(report));
console.log(JSON.stringify({ ...report, jsonPath, markdownPath }, null, 2));
if (!report.pass) process.exitCode = 1;

function renderMarkdown(value) {
  return `# Legacy Migration Inventory

Generated from legacy source: ${value.generatedAt}. Keep this file under 4KB.

| Surface | Discovered | Covered | Missing |
| --- | ---: | ---: | --- |
| Game router keys | ${value.routes.total} | ${value.routes.covered} | ${value.routes.missing.join(", ") || "None"} |
| Admin router modes | ${value.adminModes.total} | ${value.adminModes.covered} | ${value.adminModes.missing.join(", ") || "None"} |
| Public PHP entrypoints | ${value.publicEntries.total} | ${value.publicEntries.covered} | ${value.publicEntries.missing.join(", ") || "None"} |
| Bot API functions | ${value.botFunctions.total} | ${value.botFunctions.covered} | ${value.botFunctions.missing.join(", ") || "None"} |
| Optional PHP Mods | ${value.modRuntime.mods} | policy decision | ${value.modRuntime.hooks} executable hooks |

## Mod Runtime Decision

The Go server never executes arbitrary PHP. The bundled optional PHP Mods are
legacy-only and are not functionally migrated by this decision. Go must reject
new installation of these Mods; an already-enabled Mod makes full functional
parity unavailable until a native Go adapter exists. This decision closes an
unsafe execution ambiguity, not the absolute migration gap.

## Enforcement

Run \`bun testing/e2e/audit-legacy-migration-inventory.mjs\`. The audit fails for
unmapped game router keys or Admin modes. Mod hook methods remain listed in the
JSON artifact so native adapter work cannot disappear from the denominator.
`;
}

async function discoverPublicEntries() {
  const helperNames = new Set(["common.php", "db.php", "loca.php", "loca_startpage.php", "products.php", "uni.php"]);
  const result = [];
  for (const name of await readdir(resolve(root, "wwwroot"))) {
    if (name.endsWith(".php") && !helperNames.has(name)) result.push(`/${name}`);
  }
  for (const dir of ["reg", "feed"]) {
    for (const name of await readdir(resolve(root, "game", dir))) {
      if (name.endsWith(".php")) result.push(`/game/${dir}/${name}`);
    }
  }
  for (const name of await readdir(resolve(root, "game"))) {
    if (name.endsWith(".php")) result.push(`/game/${name}`);
  }
  return result.sort();
}
