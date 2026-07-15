import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const registry = JSON.parse(await readFile(resolve(import.meta.dirname, "bot-runtime-coverage-registry.json"), "utf8"));
const legacy = await readFile(resolve(root, "game/core/botapi.php"), "utf8");
const goRuntime = await readFile(resolve(root, "backend/internal/infrastructure/mysqlgame/bot_runtime.go"), "utf8");
const runnerSource = await readFile(resolve(import.meta.dirname, registry.runner), "utf8");
const wrapper = await readFile(resolve(import.meta.dirname, "run-golang-migration-qa.sh"), "utf8");
const summary = await readFile(resolve(import.meta.dirname, "golang-migration-qa-summary.mjs"), "utf8");
const reportPath = resolve(root, ".tmp/golang-bot-runtime-coverage.json");

const discovered = [...legacy.matchAll(/function\s+(Bot[A-Za-z0-9_]+)\s*\(/g)].map((match) => match[1]);
const entries = registry.entries ?? [];
const byFunction = new Map(entries.map((entry) => [entry.function, entry]));
const discoveredSet = new Set(discovered);
const missing = discovered.filter((name) => !byFunction.has(name));
const stale = entries.filter((entry) => !discoveredSet.has(entry.function)).map((entry) => entry.function);
const invalid = [];

if (!wrapper.includes(registry.runner)) invalid.push("runner not in full QA");
if (!summary.includes(registry.report)) invalid.push("report not in QA summary");
for (const entry of entries) {
  if (!goRuntime.includes(`case "${entry.function}"`)) invalid.push(`${entry.function}: Go runtime case missing`);
  if (!entry.case || !runnerSource.includes(entry.case.replace("runtime-queue-", ""))) invalid.push(`${entry.function}: differential case missing`);
}
const duplicates = entries.map((entry) => entry.function)
  .filter((name, index, names) => names.indexOf(name) !== index);
const pass = missing.length === 0 && stale.length === 0 && invalid.length === 0 && duplicates.length === 0;
const report = {
  pass,
  metrics: {
    discovered: discovered.length,
    registered: entries.length,
    implemented: entries.length - invalid.filter((item) => item.includes("Go runtime")).length,
    directDifferential: entries.length - invalid.filter((item) => item.includes("differential case")).length,
    missing: missing.length,
    stale: stale.length,
    invalid: invalid.length
  },
  missing,
  stale,
  invalid,
  duplicates,
  entries
};
await writeFile(reportPath, `${JSON.stringify(report, null, 2)}\n`);
console.log(JSON.stringify({ pass, ...report.metrics, report: reportPath }, null, 2));
if (!pass) process.exitCode = 1;
