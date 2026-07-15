import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const game = resolve(root, "game");
const baselineFiles = ["engine.md5", "page_admin.md5", "page.md5", "reg.md5"];
const pairPattern = /s:\d+:"([^"]+)";s:\d+:"([0-9a-fA-F]{32})";/g;
const entries = [];
const invalid = [];

for (const baseline of baselineFiles) {
  const source = await readFile(resolve(game, "temp", baseline), "utf8");
  const matches = [...source.matchAll(pairPattern)];
  if (matches.length === 0) invalid.push(`${baseline}: no checksum entries`);
  for (const match of matches) {
    const path = match[1];
    const expected = match[2].toLowerCase();
    try {
      const actual = createHash("md5").update(await readFile(resolve(game, path))).digest("hex");
      entries.push({ baseline, path, expected, actual, pass: expected === actual });
    } catch (error) {
      invalid.push(`${baseline}:${path}: ${error instanceof Error ? error.message : String(error)}`);
    }
  }
}

const keys = entries.map(({ baseline, path }) => `${baseline}:${path}`);
const duplicates = keys.filter((key, index) => keys.indexOf(key) !== index);
const mismatches = entries.filter((entry) => !entry.pass);
const pass = invalid.length === 0 && duplicates.length === 0 && mismatches.length === 0;
const report = {
  pass,
  metrics: { baselines: baselineFiles.length, entries: entries.length, mismatches: mismatches.length, invalid: invalid.length },
  mismatches,
  invalid,
  duplicates
};
const reportPath = resolve(root, ".tmp/golang-checksum-baseline-audit.json");
await writeFile(reportPath, `${JSON.stringify(report, null, 2)}\n`);
console.log(JSON.stringify({ pass, ...report.metrics, report: reportPath }, null, 2));
if (!pass) process.exitCode = 1;
