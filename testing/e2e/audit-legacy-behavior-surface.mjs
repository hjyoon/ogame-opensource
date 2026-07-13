import { createHash } from "node:crypto";
import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { dirname, relative, resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const baselinePath = resolve(root, "testing/e2e/legacy-behavior-surface-baseline.json");
const reportPath = resolve(root, ".tmp/legacy-behavior-surface.json");
const markdownPath = resolve(root, "testing/e2e/COVERAGE-legacy-behavior-surface.md");
const updateBaseline = process.argv.includes("--update-baseline");
const files = (await Promise.all([walk(resolve(root, "game")), walk(resolve(root, "wwwroot"))]))
  .flat()
  .filter((path) => path.endsWith(".php") || path.endsWith(".js"))
  .sort();

const surface = {
  requestInputs: [],
  actionValues: [],
  queueConstants: [],
  sqlMutations: [],
  navigationHandlers: []
};

for (const path of files) {
  const source = await readFile(path, "utf8");
  const file = relative(root, path);
  discoverRequestInputs(source, file, surface.requestInputs);
  discoverActionValues(source, file, surface.actionValues);
  discoverQueueConstants(source, file, surface.queueConstants);
  discoverSQLMutations(source, file, surface.sqlMutations);
  discoverNavigationHandlers(source, file, surface.navigationHandlers);
}

for (const values of Object.values(surface)) values.sort((a, b) => a.id.localeCompare(b.id));
const baseline = {
  version: 1,
  sourceRoots: ["game", "wwwroot"],
  categories: Object.fromEntries(Object.entries(surface).map(([name, values]) => [name, values.map(({ id }) => id)]))
};
baseline.digest = digest(baseline.categories);

if (updateBaseline) {
  await writeFile(baselinePath, JSON.stringify(baseline, null, 2) + "\n");
}

let expected = null;
try {
  expected = JSON.parse(await readFile(baselinePath, "utf8"));
} catch {
  // Missing baseline is reported as drift below.
}
const drift = compareBaseline(expected?.categories ?? {}, baseline.categories);
const pass = expected !== null && drift.added.length === 0 && drift.removed.length === 0;
const report = {
  pass,
  generatedAt: new Date().toISOString(),
  baselineDigest: expected?.digest ?? null,
  currentDigest: baseline.digest,
  counts: Object.fromEntries(Object.entries(surface).map(([name, values]) => [name, {
    total: values.length,
    core: values.filter((item) => !item.file.startsWith("game/mods/")).length,
    mods: values.filter((item) => item.file.startsWith("game/mods/")).length
  }])),
  drift,
  surface
};

await mkdir(dirname(reportPath), { recursive: true });
await writeFile(reportPath, JSON.stringify(report, null, 2) + "\n");
await writeFile(markdownPath, renderMarkdown(report));
console.log(JSON.stringify({ pass, counts: report.counts, drift, reportPath, baselinePath, markdownPath }, null, 2));
if (!pass) process.exitCode = 1;

function discoverRequestInputs(source, file, output) {
  const pattern = /\$_(GET|POST|REQUEST|COOKIE|FILES)\s*\[\s*(["'])([^"']+)\2\s*\]/g;
  for (const match of source.matchAll(pattern)) {
    add(output, `${match[1]}:${match[3]}@${file}`, file, lineOf(source, match.index), { method: match[1], name: match[3] });
  }
}

function discoverActionValues(source, file, output) {
  const patterns = [
    /\$_(?:GET|POST|REQUEST)\s*\[\s*["'](?:action|mode|page|modus)["']\s*\]\s*(?:===|==)\s*(["'])([^"']+)\1/g,
    /case\s+(["'])([^"']+)\1\s*:/g
  ];
  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) {
      add(output, `action:${match[2]}@${file}`, file, lineOf(source, match.index), { value: match[2] });
    }
  }
}

function discoverQueueConstants(source, file, output) {
  const pattern = /const\s+(QTYP_[A-Z0-9_]+)\s*=\s*([^;\r\n]+)\s*;/g;
  for (const match of source.matchAll(pattern)) {
    add(output, `queue:${match[1]}@${file}`, file, lineOf(source, match.index), { name: match[1], expression: compact(match[2]) });
  }
}

function discoverSQLMutations(source, file, output) {
  const pattern = /\b(INSERT\s+INTO|REPLACE\s+INTO|UPDATE|DELETE\s+FROM|ALTER\s+TABLE|CREATE\s+TABLE|DROP\s+TABLE|TRUNCATE\s+TABLE)\b/gi;
  for (const match of source.matchAll(pattern)) {
    const line = lineOf(source, match.index);
    const fragment = compact(source.slice(Math.max(0, match.index - 40), Math.min(source.length, match.index + 180)));
    const verb = match[1].toUpperCase().replace(/\s+/g, " ");
    add(output, `sql:${verb}:${line}@${file}`, file, line, { verb, fragment });
  }
}

function discoverNavigationHandlers(source, file, output) {
  const pattern = /\b(onclick\s*=|window\.open\s*\(|document\.location(?:\.href)?\s*=|location\.href\s*=|fenster\s*\(|doit\s*\()/gi;
  for (const match of source.matchAll(pattern)) {
    const line = lineOf(source, match.index);
    add(output, `nav:${compact(match[1]).toLowerCase()}:${line}@${file}`, file, line, { kind: compact(match[1]).toLowerCase() });
  }
}

function add(output, id, file, line, detail) {
  if (!output.some((item) => item.id === id)) output.push({ id, file, line, ...detail });
}

function compareBaseline(expected, current) {
  const added = [];
  const removed = [];
  const names = new Set([...Object.keys(expected), ...Object.keys(current)]);
  for (const name of names) {
    const before = new Set(expected[name] ?? []);
    const after = new Set(current[name] ?? []);
    for (const id of after) if (!before.has(id)) added.push(`${name}:${id}`);
    for (const id of before) if (!after.has(id)) removed.push(`${name}:${id}`);
  }
  return { added: added.sort(), removed: removed.sort() };
}

function digest(value) {
  return createHash("sha256").update(JSON.stringify(value)).digest("hex");
}

function compact(value) {
  return value.replace(/\s+/g, " ").trim();
}

function lineOf(source, index = 0) {
  return source.slice(0, index).split("\n").length;
}

async function walk(directory) {
  const result = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) result.push(...(await walk(path)));
    else result.push(path);
  }
  return result;
}

function renderMarkdown(value) {
  return `# Legacy Behavior Surface\n\nGenerated from PHP/JS source: ${value.generatedAt}. Keep this file under 4KB.\n\n| Category | Core | Optional Mods | Total |\n| --- | ---: | ---: | ---: |\n| Request inputs | ${value.counts.requestInputs.core} | ${value.counts.requestInputs.mods} | ${value.counts.requestInputs.total} |\n| Action values | ${value.counts.actionValues.core} | ${value.counts.actionValues.mods} | ${value.counts.actionValues.total} |\n| Queue constants | ${value.counts.queueConstants.core} | ${value.counts.queueConstants.mods} | ${value.counts.queueConstants.total} |\n| SQL mutation sites | ${value.counts.sqlMutations.core} | ${value.counts.sqlMutations.mods} | ${value.counts.sqlMutations.total} |\n| Navigation handlers | ${value.counts.navigationHandlers.core} | ${value.counts.navigationHandlers.mods} | ${value.counts.navigationHandlers.total} |\n\n## Baseline Gate\n\nThe tracked baseline freezes discovered behavior IDs. Any added or removed source\nsurface fails migration QA until reviewed. Update deliberately with\n\`bun testing/e2e/audit-legacy-behavior-surface.mjs --update-baseline\`.\n\nThis catalog is a denominator discovery aid, not proof that every listed item is\nfunctionally migrated. Coverage evidence must be attached through differential,\nunit, API, or E2E cases as the registry is expanded.\n\n- Baseline digest: \`${value.baselineDigest ?? "missing"}\`\n- Current digest: \`${value.currentDigest}\`\n- Added: ${value.drift.added.length}\n- Removed: ${value.drift.removed.length}\n- Result: ${value.pass ? "PASS" : "FAIL"}\n`;
}
