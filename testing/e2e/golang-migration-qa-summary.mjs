import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";

const rootDir = resolve(import.meta.dirname, "../..");
const outputJSON = resolve(rootDir, ".tmp/golang-migration-qa-summary.json");
const outputMarkdown = resolve(rootDir, ".tmp/golang-migration-qa-summary.md");
const browserNames = uniqueWords(process.env.OGAME_QA_SUMMARY_BROWSERS ?? "chromium firefox");

const artifacts = [
  { name: "legacy behavior surface", path: ".tmp/legacy-behavior-surface.json" },
  { name: "legacy source inventory", path: ".tmp/legacy-migration-inventory.json" },
  { name: "legacy checksum baselines", path: ".tmp/golang-checksum-baseline-audit.json" },
  { name: "state mutation coverage", path: ".tmp/golang-state-mutation-coverage.json" },
  { name: "queue runtime coverage", path: ".tmp/golang-queue-runtime-coverage.json" },
  { name: "bot runtime coverage", path: ".tmp/golang-bot-runtime-coverage.json" },
  { name: "go DB recovery", path: ".tmp/golang-db-recovery.json" },
  { name: "go DB pool", path: ".tmp/golang-db-pool.json" },
  { name: "go Mod policy", path: ".tmp/golang-mod-policy.json" },
  { name: "go compatibility smoke", path: ".tmp/golang-compat-smoke.json" },
  { name: "go/PHP resource differential", path: ".tmp/golang-resource-differential.json" },
  { name: "go/PHP overview differential", path: ".tmp/golang-overview-differential.json" },
  { name: "go/PHP officers differential", path: ".tmp/golang-officers-differential.json" },
  { name: "go/PHP merchant differential", path: ".tmp/golang-merchant-differential.json" },
  { name: "go/PHP payment differential", path: ".tmp/golang-payment-differential.json" },
  { name: "go/PHP empire differential", path: ".tmp/golang-empire-differential.json" },
  { name: "go/PHP galaxy actions differential", path: ".tmp/golang-galaxy-actions-differential.json" },
  { name: "go/PHP public account differential", path: ".tmp/golang-public-account-differential.json" },
  { name: "go/PHP building differential", path: ".tmp/golang-building-differential.json" },
  { name: "go/PHP research differential", path: ".tmp/golang-research-differential.json" },
  { name: "go/PHP shipyard-defense differential", path: ".tmp/golang-shipyard-defense-differential.json" },
  { name: "go/PHP advanced building differential", path: ".tmp/golang-building-advanced-differential.json" },
  { name: "go/PHP defense limits differential", path: ".tmp/golang-defense-limits-differential.json" },
  { name: "go/PHP fleet differential", path: ".tmp/golang-fleet-differential.json" },
  { name: "go/PHP accelerated fleet Recall differential", path: ".tmp/golang-fleet-speed-recall-differential.json" },
  { name: "go/PHP fleet-template differential", path: ".tmp/golang-fleet-template-differential.json" },
  { name: "go/PHP combat engine differential", path: ".tmp/golang-combat-engine-differential.json" },
  { name: "go/PHP combat report differential", path: ".tmp/golang-combat-report-differential.json" },
  { name: "go/PHP expedition differential", path: ".tmp/golang-expedition-differential.json" },
  { name: "go/PHP colonization differential", path: ".tmp/golang-colonization-differential.json" },
  { name: "go/PHP missile differential", path: ".tmp/golang-missile-differential.json" },
  { name: "go/PHP phalanx differential", path: ".tmp/golang-phalanx-differential.json" },
  { name: "go/PHP jump gate differential", path: ".tmp/golang-jump-gate-differential.json" },
  { name: "go/PHP buddy differential", path: ".tmp/golang-buddy-differential.json" },
  { name: "go/PHP messages differential", path: ".tmp/golang-messages-differential.json" },
  { name: "go/PHP notes differential", path: ".tmp/golang-notes-differential.json" },
  { name: "go/PHP alliance differential", path: ".tmp/golang-alliance-differential.json" },
  { name: "go/PHP options differential", path: ".tmp/golang-options-differential.json" },
  { name: "go/PHP ACS attack differential", path: ".tmp/golang-acs-attack-differential.json" },
  { name: "go/PHP holding defense differential", path: ".tmp/golang-holding-defense-differential.json" },
  { name: "go/PHP battle moon differential", path: ".tmp/golang-battle-moon-differential.json" },
  { name: "go/PHP moon destruction differential", path: ".tmp/golang-moon-destruction-differential.json" },
  { name: "go/PHP admin bans differential", path: ".tmp/golang-admin-bans-differential.json" },
  { name: "go/PHP admin queue differential", path: ".tmp/golang-admin-queue-differential.json" },
  { name: "go/PHP admin cron differential", path: ".tmp/golang-admin-cron-differential.json" },
  { name: "go/PHP runtime queue differential", path: ".tmp/golang-runtime-queue-differential.json" },
  { name: "go/PHP admin cleanup differential", path: ".tmp/golang-admin-cleanup-differential.json" },
  { name: "go/PHP admin coupon cron differential", path: ".tmp/golang-admin-coupon-cron-differential.json" },
  { name: "go/PHP admin users differential", path: ".tmp/golang-admin-users-differential.json" },
  { name: "go/PHP admin planets differential", path: ".tmp/golang-admin-planets-differential.json" },
  { name: "go/PHP admin operations differential", path: ".tmp/golang-admin-operations-differential.json" },
  { name: "go/PHP admin universe differential", path: ".tmp/golang-admin-universe-differential.json" },
  { name: "go/PHP admin audit differential", path: ".tmp/golang-admin-audit-differential.json" },
  { name: "go/PHP admin coupons differential", path: ".tmp/golang-admin-coupons-differential.json" },
  { name: "go/PHP admin database differential", path: ".tmp/golang-admin-database-differential.json" },
  { name: "go/PHP admin bots differential", path: ".tmp/golang-admin-bots-differential.json" },
  { name: "go/PHP admin colony settings differential", path: ".tmp/golang-admin-colony-settings-differential.json" },
  { name: "go/PHP admin checksum differential", path: ".tmp/golang-admin-checksum-differential.json" },
  { name: "go/PHP admin localization differential", path: ".tmp/golang-admin-loca-differential.json" },
  { name: "go/PHP admin simulators differential", path: ".tmp/golang-admin-simulators-differential.json" },
  { name: "go user type API QA", path: ".tmp/golang-user-type-qa.json" },
  ...browserNames.map((browser) => ({
    name: `go user type Playwright ${browser}`,
    path: `.tmp/playwright-user-types/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `public visual ${browser}`,
    path: `.tmp/playwright-visual/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `public login dynamic ${browser}`,
    path: `.tmp/playwright-public-login-dynamic/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `auth visual ${browser}`,
    path: `.tmp/playwright-auth-visual/auth/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `auth game visual ${browser}`,
    path: `.tmp/playwright-authenticated-game-visual/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `auth game commander visual ${browser}`,
    path: `.tmp/playwright-authenticated-game-commander-visual/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `auth game dynamic ${browser}`,
    path: `.tmp/playwright-authenticated-game-dynamic/${browser}/report.json`
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
  })),
  ...browserNames.map((browser) => ({
    name: `fleet Recall timing ${browser}`,
    path: `.tmp/playwright-fleet-recall-timing/${browser}/report.json`
  })),
  ...browserNames.map((browser) => ({
    name: `navigation visual ${browser}`,
    path: `.tmp/playwright-navigation-visual/${browser}/report.json`
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
