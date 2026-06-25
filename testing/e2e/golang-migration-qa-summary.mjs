import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";

const rootDir = resolve(import.meta.dirname, "../..");
const outputJSON = resolve(rootDir, ".tmp/golang-migration-qa-summary.json");
const outputMarkdown = resolve(rootDir, ".tmp/golang-migration-qa-summary.md");
const browserNames = uniqueWords(process.env.OGAME_QA_SUMMARY_BROWSERS ?? "chromium firefox");

const artifacts = [
  { name: "go compatibility smoke", path: ".tmp/golang-compat-smoke.json" },
  { name: "go user type API QA", path: ".tmp/golang-user-type-qa.json" },
  ...browserNames.map((browser) => ({
    name: `go user type Playwright ${browser}`,
    path: `.tmp/playwright-user-types/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `auth visual ${browser}`,
    path: `.tmp/playwright-auth-visual/auth/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `empire visual ${browser}`,
    path: `.tmp/playwright-auth-visual/empire/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `alliance visual ${browser}`,
    path: `.tmp/playwright-auth-visual/alliance/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `overview fleet visual ${browser}`,
    path: `.tmp/playwright-overview-fleet-visual/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `overview fleet countdown ${browser}`,
    path: `.tmp/playwright-overview-fleet-countdown/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `overview all-cases ${browser}`,
    path: `.tmp/playwright-overview-all-cases/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `fleet continue visual ${browser}`,
    path: `.tmp/playwright-fleet-continue-visual/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `fleet all-cases ${browser}`,
    path: `.tmp/playwright-fleet-all-cases/${browser}/report.json`
  }))
];

const results = [];
for (const artifact of artifacts) {
  results.push(await summarizeArtifact(artifact));
}

const report = {
  generatedAt: new Date().toISOString(),
  goBaseURL: process.env.OGAME_GO_BASE_URL ?? null,
  allPass: results.every((result) => result.status !== "fail"),
  passed: results.filter((result) => result.status === "pass").length,
  failed: results.filter((result) => result.status === "fail").length,
  skipped: results.filter((result) => result.status === "skip").length,
  results
};

await mkdir(dirname(outputJSON), { recursive: true });
await writeFile(outputJSON, JSON.stringify(report, null, 2));
await writeFile(outputMarkdown, renderMarkdown(report));

process.stdout.write(
  JSON.stringify(
    {
      allPass: report.allPass,
      passed: report.passed,
      failed: report.failed,
      skipped: report.skipped,
      report: outputJSON,
      markdown: outputMarkdown
    },
    null,
    2
  ) + "\n"
);

if (!report.allPass) {
  process.exitCode = 1;
}

async function summarizeArtifact(artifact) {
  const absolutePath = resolve(rootDir, artifact.path);
  if (!existsSync(absolutePath)) {
    return {
      name: artifact.name,
      status: "skip",
      path: artifact.path,
      reason: "report not found"
    };
  }
  try {
    const data = JSON.parse(await readFile(absolutePath, "utf8"));
    const pass = reportPass(data);
    return {
      name: artifact.name,
      status: pass ? "pass" : "fail",
      path: artifact.path,
      failures: pass ? [] : summarizeFailures(data),
      metrics: reportMetrics(data)
    };
  } catch (error) {
    return {
      name: artifact.name,
      status: "fail",
      path: artifact.path,
      failures: [`invalid report: ${error instanceof Error ? error.message : String(error)}`],
      metrics: {}
    };
  }
}

function reportPass(data) {
  if (typeof data.all_pass === "boolean") return data.all_pass;
  if (typeof data.allPass === "boolean") return data.allPass;
  if (typeof data.pass === "boolean") return data.pass;
  return false;
}

function summarizeFailures(data) {
  const failures = [];
  if (Array.isArray(data.cases)) {
    for (const testCase of data.cases.filter((item) => item.pass !== true)) {
      failures.push(testCase.name ?? testCase.case ?? "case failed");
    }
  }
  if (Array.isArray(data.results)) {
    for (const result of data.results.filter((item) => item.pass !== true)) {
      failures.push(result.page ?? result.name ?? "result failed");
    }
  }
  if (Array.isArray(data.captures)) {
    for (const capture of data.captures.filter((item) => item.contractPass === false || item.pass === false)) {
      failures.push(capture.name ?? "capture failed");
    }
  }
  if (failures.length === 0) {
    failures.push("top-level pass flag is false");
  }
  return failures;
}

function reportMetrics(data) {
  const metrics = {};
  if (Array.isArray(data.results)) {
    metrics.results = data.results.length;
  }
  if (Array.isArray(data.cases)) {
    metrics.cases = data.cases.length;
    metrics.checks = data.cases.reduce((sum, testCase) => sum + (Array.isArray(testCase.checks) ? testCase.checks.length : 0), 0);
  }
  if (Array.isArray(data.captures)) {
    metrics.captures = data.captures.length;
  }
  if (typeof data.diffRatio === "number") {
    metrics.diffRatio = data.diffRatio;
  }
  if (typeof data.changedPixels === "number") {
    metrics.changedPixels = data.changedPixels;
  }
  return metrics;
}

function renderMarkdown(report) {
  const lines = [
    "# Go Migration QA Summary",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Go base URL: ${report.goBaseURL ?? "-"}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    `- Passed: ${report.passed}`,
    `- Failed: ${report.failed}`,
    `- Skipped: ${report.skipped}`,
    "",
    "| Artifact | Status | Metrics | Failures |",
    "| --- | --- | --- | --- |"
  ];
  for (const result of report.results) {
    lines.push(
      `| ${escapeMarkdown(result.name)} | ${result.status.toUpperCase()} | ${escapeMarkdown(JSON.stringify(result.metrics ?? {}))} | ${escapeMarkdown(
        (result.failures ?? [result.reason ?? ""]).join(", ") || "-"
      )} |`
    );
  }
  lines.push("");
  return `${lines.join("\n")}\n`;
}

function uniqueWords(value) {
  return Array.from(new Set(value.split(/\s+/).map((item) => item.trim()).filter(Boolean)));
}

function escapeMarkdown(value) {
  return String(value).replaceAll("|", "\\|");
}
