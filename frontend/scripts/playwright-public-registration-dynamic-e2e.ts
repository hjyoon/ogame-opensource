import { chromium, firefox, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { trimTrailingSlash } from "./visual/game-visual-utils";

type BrowserName = "chromium" | "firefox";
type SideName = "legacy" | "migrated";

type RegisterState = {
  url: string;
  status: number | null;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  states: Record<string, { info: string; status: string; statusClass: string }>;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, `.tmp/playwright-public-registration-dynamic/${browserName}`);
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);

await mkdir(outputDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const legacyContext = await newContext(legacyBaseURL);
  const migratedContext = await newContext(migratedBaseURL);
  const legacy = await runSide(legacyContext, "legacy");
  const migrated = await runSide(migratedContext, "migrated");
  await legacyContext.close();
  await migratedContext.close();

  const comparisons = compareStates(legacy, migrated);
  const pass =
    legacy.status === 200 &&
    migrated.status === 200 &&
    legacy.consoleErrors.length === 0 &&
    migrated.consoleErrors.length === 0 &&
    legacy.failedRequests.length === 0 &&
    migrated.failedRequests.length === 0 &&
    legacy.badResponses.length === 0 &&
    migrated.badResponses.length === 0 &&
    comparisons.length === 0;

  const report = {
    generatedAt: new Date().toISOString(),
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    legacyBaseURL,
    migratedBaseURL,
    pass,
    legacy,
    migrated,
    comparisons
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(JSON.stringify({ pass, report: join(outputDir, "report.json") }, null, 2) + "\n");
  if (!pass) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

async function newContext(baseURL: string): Promise<BrowserContext> {
  const context = await browser.newContext({
    viewport: { width: 1024, height: 768 },
    deviceScaleFactor: 1,
    locale: "en-US",
    reducedMotion: "reduce"
  });
  await context.addCookies([{ name: "ogamelang", value: "en", url: baseURL }]);
  return context;
}

async function runSide(context: BrowserContext, side: SideName): Promise<RegisterState> {
  const page = await context.newPage();
  const consoleErrors: string[] = [];
  const failedRequests: string[] = [];
  const badResponses: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });
  page.on("requestfailed", (request) => {
    failedRequests.push(`${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`.trim());
  });
  page.on("response", (response) => {
    const status = response.status();
    if (status >= 400 && !response.url().endsWith("/favicon.ico")) {
      badResponses.push(`${status} ${response.url()}`);
    }
  });

  const response = await page.goto(registerURL(side), { waitUntil: "domcontentloaded", timeout: 20_000 });
  await page.locator("form[name='registerForm']").waitFor({ timeout: 10_000 });

  const username = `Dyn${browserName}${Date.now().toString(36).slice(-5)}`.slice(0, 18);
  const states: RegisterState["states"] = {};
  await page.locator("input[name='email']").focus();
  await page.locator("input[name='character']").focus();
  await page.waitForTimeout(100);
  states.usernameHelp = await readRegisterState(page);

  await page.locator("input[name='character']").fill(username);
  await page.waitForTimeout(2200);
  states.usernamePollOK = await readRegisterState(page);

  await page.locator("input[name='email']").focus();
  await page.waitForTimeout(100);
  states.emailHelp = await readRegisterState(page);

  await page.locator("input[name='password']").focus();
  await page.waitForTimeout(100);
  states.passwordHelp = await readRegisterState(page);

  await page.locator("input[name='agb']").focus();
  await page.waitForTimeout(100);
  states.termsHelp = await readRegisterState(page);

  const currentURL = page.url();
  await page.close();
  return {
    url: currentURL,
    status: response?.status() ?? null,
    consoleErrors,
    failedRequests,
    badResponses,
    states
  };
}

function registerURL(side: SideName): string {
  return side === "legacy" ? `${legacyBaseURL}/register.php` : `${migratedBaseURL}/register`;
}

async function readRegisterState(page: Page) {
  const statusSpan = page.locator("#statustext span").first();
  const statusClass = (await statusSpan.count()) > 0 ? (await statusSpan.getAttribute("class")) ?? "" : "";
  return {
    info: compact(await page.locator("#infotext").innerText({ timeout: 5_000 }).catch(() => "")),
    status: compact(await page.locator("#statustext").innerText({ timeout: 5_000 }).catch(() => "")),
    statusClass
  };
}

function compareStates(legacy: RegisterState, migrated: RegisterState): string[] {
  const errors: string[] = [];
  for (const key of Object.keys(legacy.states)) {
    const legacyState = legacy.states[key];
    const migratedState = migrated.states[key];
    if (!migratedState) {
      errors.push(`${key} missing on migrated`);
      continue;
    }
    for (const field of ["info", "status", "statusClass"] as const) {
      if (legacyState[field] !== migratedState[field]) {
        errors.push(`${key}.${field} differs: legacy=${legacyState[field]} migrated=${migratedState[field]}`);
      }
    }
  }
  return errors;
}

function compact(value: string): string {
  return value.replace(/\s+/g, " ").trim();
}

function renderMarkdown(report: {
  generatedAt: string;
  browserName: BrowserName;
  browserExecutable: string;
  pass: boolean;
  comparisons: string[];
  legacy: RegisterState;
  migrated: RegisterState;
}) {
  const lines = [
    "# Public Registration Dynamic Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Result: ${report.pass ? "PASS" : "FAIL"}`,
    "",
    "| State | Legacy | Migrated |",
    "| --- | --- | --- |"
  ];
  for (const key of Object.keys(report.legacy.states)) {
    lines.push(`| ${key} | ${stateSummary(report.legacy.states[key])} | ${stateSummary(report.migrated.states[key])} |`);
  }
  if (report.comparisons.length > 0) {
    lines.push("", "## Differences", "", ...report.comparisons.map((item) => `- ${item}`));
  }
  return `${lines.join("\n")}\n`;
}

function stateSummary(state: { info: string; status: string; statusClass: string } | undefined): string {
  if (!state) {
    return "missing";
  }
  return `${state.info} / ${state.status} / ${state.statusClass}`;
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name] ?? fallback;
  return raw === "firefox" ? "firefox" : "chromium";
}
