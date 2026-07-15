import { readdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const registryPath = resolve(import.meta.dirname, "state-mutation-coverage-registry.json");
const reportPath = resolve(root, ".tmp/golang-state-mutation-coverage.json");
const wrapper = await readFile(resolve(import.meta.dirname, "run-golang-migration-qa.sh"), "utf8");
const summary = await readFile(resolve(import.meta.dirname, "golang-migration-qa-summary.mjs"), "utf8");
const registry = JSON.parse(await readFile(registryPath, "utf8"));
const mutationVerbs = /^(Mutate|Update|Create|Delete|Rename|Recruit|Launch|Recall|Dispatch|Jump|Activate|Authenticate|Logout|Recover|Register)/;
const exactMutationMethods = new Set(["GameSessionLookup.GetGameSession"]);

const discovered = [];
for (const scope of ["game", "publicsite"]) {
  const directory = resolve(root, "backend/internal/application", scope);
  for (const name of await readdir(directory)) {
    if (!name.endsWith(".go") || name.endsWith("_test.go")) continue;
    const source = await readFile(resolve(directory, name), "utf8");
    const expression = /func\s+\(\w+\s+\*?([A-Za-z0-9_]+)\)\s+([A-Z][A-Za-z0-9_]*)\s*\(/g;
    for (const match of source.matchAll(expression)) {
      const method = `${match[1]}.${match[2]}`;
      if (mutationVerbs.test(match[2]) || exactMutationMethods.has(method)) {
        discovered.push({ method, source: `backend/internal/application/${scope}/${name}` });
      }
    }
  }
}

const entries = registry.entries ?? [];
const byMethod = new Map(entries.map((entry) => [entry.method, entry]));
const discoveredMethods = new Set(discovered.map((entry) => entry.method));
const missing = discovered.filter((entry) => !byMethod.has(entry.method));
const stale = entries.filter((entry) => !discoveredMethods.has(entry.method)).map((entry) => entry.method);
const invalid = [];
for (const entry of entries) {
  if (!Array.isArray(entry.reports) || entry.reports.length === 0) invalid.push(`${entry.method}: no reports`);
  if (!entry.runner || !wrapper.includes(entry.runner)) invalid.push(`${entry.method}: runner not in full QA`);
  for (const report of entry.reports ?? []) {
    if (!summary.includes(report)) invalid.push(`${entry.method}: report not in summary: ${report}`);
  }
  if (entry.evidence !== "differential" && entry.evidence !== "combined") invalid.push(`${entry.method}: invalid evidence`);
}
const duplicateMethods = entries.map((entry) => entry.method).filter((method, index, methods) => methods.indexOf(method) !== index);
const pass = missing.length === 0 && stale.length === 0 && invalid.length === 0 && duplicateMethods.length === 0;
const report = {
  pass,
  metrics: {
    discovered: discovered.length,
    registered: entries.length,
    differential: entries.filter((entry) => entry.evidence === "differential").length,
    combined: entries.filter((entry) => entry.evidence === "combined").length,
    missing: missing.length,
    stale: stale.length,
    invalid: invalid.length
  },
  missing,
  stale,
  invalid,
  duplicateMethods,
  entries
};
await writeFile(reportPath, `${JSON.stringify(report, null, 2)}\n`);
console.log(JSON.stringify({ pass, ...report.metrics, report: reportPath }, null, 2));
if (!pass) process.exitCode = 1;
