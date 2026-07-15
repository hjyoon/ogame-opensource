import { access, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const registry = JSON.parse(await readFile(resolve(import.meta.dirname, "queue-runtime-coverage-registry.json"), "utf8"));
const definitions = await readFile(resolve(root, "game/core/defs.php"), "utf8");
const queue = await readFile(resolve(root, "game/core/queue.php"), "utf8");
const wrapper = await readFile(resolve(import.meta.dirname, "run-golang-migration-qa.sh"), "utf8");
const summary = await readFile(resolve(import.meta.dirname, "golang-migration-qa-summary.mjs"), "utf8");
const reportPath = resolve(root, ".tmp/golang-queue-runtime-coverage.json");

const discovered = [...definitions.matchAll(/const\s+(QTYP_[A-Z0-9_]+)\s*=\s*"([^"]+)"/g)]
  .map((match) => ({ constant: match[1], type: match[2] }));
const dispatched = new Set([...queue.matchAll(/case\s+(QTYP_[A-Z0-9_]+)\s*:/g)].map((match) => match[1]));
const entries = registry.entries ?? [];
const byConstant = new Map(entries.map((entry) => [entry.constant, entry]));
const discoveredConstants = new Set(discovered.map((entry) => entry.constant));
const missing = discovered.filter((entry) => !byConstant.has(entry.constant));
const stale = entries.filter((entry) => !discoveredConstants.has(entry.constant)).map((entry) => entry.constant);
const invalid = [];

for (const item of discovered) {
  const entry = byConstant.get(item.constant);
  if (!dispatched.has(item.constant)) invalid.push(`${item.constant}: not dispatched by UpdateQueue`);
  if (entry && entry.type !== item.type) invalid.push(`${item.constant}: expected type ${item.type}, registered ${entry.type}`);
}
for (const entry of entries) {
  if (!entry.runner || !wrapper.includes(entry.runner)) invalid.push(`${entry.constant}: runner not in full QA`);
  if (!entry.report || !summary.includes(entry.report)) invalid.push(`${entry.constant}: report not in summary`);
  if (entry.runner) {
    try {
      await access(resolve(import.meta.dirname, entry.runner));
    } catch {
      invalid.push(`${entry.constant}: runner file missing`);
    }
  }
}

const duplicates = entries.map((entry) => entry.constant)
  .filter((constant, index, constants) => constants.indexOf(constant) !== index);
const pass = missing.length === 0 && stale.length === 0 && invalid.length === 0 && duplicates.length === 0;
const report = {
  pass,
  metrics: {
    discovered: discovered.length,
    dispatched: dispatched.size,
    registered: entries.length,
    differential: entries.length,
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
