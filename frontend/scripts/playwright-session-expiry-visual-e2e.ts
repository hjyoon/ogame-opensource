import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import {
  compareScreenshots,
  deterministicScreenshotCSS,
  formatNumber,
  numberEnv,
  trimTrailingSlash,
  waitForImages,
  waitForStablePaint,
  type DiffResult
} from "./visual/game-visual-utils";

type BrowserName = "chromium" | "firefox";
type SideName = "legacy" | "migrated";
type ExpiryExpectation = "public-home" | "invalid-session";

type SessionExpiryCase = {
  name: string;
  expectation: ExpiryExpectation;
  legacyURL: string;
  migratedURL: string;
  normalizeErrorID: boolean;
  waitMs: number;
};

type CaptureResult = {
  status: number | null;
  url: string;
  pathname: string;
  text: string;
  screenshotPath: string;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
};

type CaseResult = {
  name: string;
  expectation: ExpiryExpectation;
  pass: boolean;
  comparisons: string[];
  diff: DiffResult;
  diffPath: string;
  legacy: CaptureResult;
  migrated: CaptureResult;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, `.tmp/playwright-session-expiry-visual/${browserName}`);
const screenshotDir = join(outputDir, "screenshots");
const diffDir = join(outputDir, "diffs");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const loginUser = process.env.OGAME_SESSION_EXPIRY_LOGIN_USER ?? "Legor";
const loginPassword = process.env.OGAME_SESSION_EXPIRY_LOGIN_PASS ?? "admin";
const maxDiffRatio = numberEnv("OGAME_SESSION_EXPIRY_MAX_DIFF_RATIO", 0);
const colorDeltaThreshold = numberEnv("OGAME_SESSION_EXPIRY_COLOR_DELTA", 0);
const maxVisualAttempts = Math.max(1, Math.floor(numberEnv("OGAME_SESSION_EXPIRY_VISUAL_ATTEMPTS", 3)));
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);

await mkdir(screenshotDir, { recursive: true });
await mkdir(diffDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const validPublicSession = await createLegacyPublicSession(browser);
  const cases: SessionExpiryCase[] = [
    {
      name: "missing-public-session-redirects-home",
      expectation: "public-home",
      legacyURL: `${legacyBaseURL}/game/index.php?page=overview&session=deadbeefcafe`,
      migratedURL: `${migratedBaseURL}/game/overview?session=deadbeefcafe`,
      normalizeErrorID: false,
      waitMs: 3_000
    },
    {
      name: "missing-private-session-renders-invalid-session",
      expectation: "invalid-session",
      legacyURL: `${legacyBaseURL}/game/index.php?page=overview&session=${validPublicSession}`,
      migratedURL: `${migratedBaseURL}/game/overview?session=${validPublicSession}`,
      normalizeErrorID: true,
      waitMs: 250
    }
  ];

  const results: CaseResult[] = [];
  for (const item of cases) {
    let legacy = await captureCase(item, "legacy");
    let migrated = await captureCase(item, "migrated");
    const diffPath = join(diffDir, `${item.name}.png`);
    let diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath, colorDeltaThreshold);
    let comparisons = compareCaptures(item, legacy, migrated, diff);
    for (let attempt = 2; attempt <= maxVisualAttempts && shouldRetryExactVisual(comparisons); attempt += 1) {
      legacy = await captureCase(item, "legacy");
      migrated = await captureCase(item, "migrated");
      diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath, colorDeltaThreshold);
      comparisons = compareCaptures(item, legacy, migrated, diff);
    }
    results.push({
      name: item.name,
      expectation: item.expectation,
      pass: comparisons.length === 0,
      comparisons,
      diff,
      diffPath,
      legacy,
      migrated
    });
  }

  const pass = results.every((item) => item.pass);
  const report = {
    generatedAt: new Date().toISOString(),
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    legacyBaseURL,
    migratedBaseURL,
    thresholds: { maxDiffRatio, colorDeltaThreshold },
    loginUser,
    pass,
    results
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(
    JSON.stringify(
      {
        pass,
        cases: results.map((item) => ({
          name: item.name,
          diffRatio: item.diff.diffRatio,
          changedPixels: item.diff.changedPixels,
          pass: item.pass
        })),
        report: join(outputDir, "report.json")
      },
      null,
      2
    ) + "\n"
  );
  if (!pass) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

async function createLegacyPublicSession(browser: Browser): Promise<string> {
  const context = await newContext(browser);
  const page = await context.newPage();
  try {
    const url = `${legacyBaseURL}/game/reg/login2.php?login=${encodeURIComponent(loginUser)}&pass=${encodeURIComponent(loginPassword)}`;
    await page.goto(url, { waitUntil: "networkidle", timeout: 20_000 });
    await page.waitForFunction(() => window.location.search.includes("session="), undefined, { timeout: 20_000 });
    const session = new URL(page.url()).searchParams.get("session") ?? "";
    if (session === "") {
      throw new Error("legacy login did not return a public session");
    }
    return session;
  } finally {
    await context.close();
  }
}

async function captureCase(spec: SessionExpiryCase, side: SideName): Promise<CaptureResult> {
  const context = await newContext(browser);
  const page = await context.newPage();
  const consoleErrors: string[] = [];
  const failedRequests: string[] = [];
  const badResponses: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error" && !ignoredConsoleError(message.text())) {
      consoleErrors.push(message.text());
    }
  });
  page.on("requestfailed", (request) => {
    if (!ignoredBadResponse(request.url())) {
      failedRequests.push(`${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`.trim());
    }
  });
  page.on("response", (response) => {
    const status = response.status();
    if (status >= 400 && !ignoredBadResponse(response.url(), status)) {
      badResponses.push(`${status} ${response.url()}`);
    }
  });

  try {
    const response = await page.goto(side === "legacy" ? spec.legacyURL : spec.migratedURL, {
      waitUntil: "domcontentloaded",
      timeout: 20_000
    });
    if (spec.expectation === "public-home") {
      await page.waitForFunction(() => window.location.pathname === "/" || window.location.pathname.endsWith("/home.php"), undefined, {
        timeout: 20_000
      });
    } else {
      await page.locator("body").filter({ hasText: "The session is invalid." }).waitFor({ timeout: 20_000 });
    }
    await page.waitForLoadState("networkidle", { timeout: 20_000 }).catch(() => undefined);
    await page.waitForTimeout(spec.waitMs);
    await page.addStyleTag({ content: deterministicScreenshotCSS });
    if (spec.normalizeErrorID) {
      await normalizeInvalidSessionErrorID(page);
    }
    await waitForImages(page).catch(() => undefined);
    await waitForStablePaint(page).catch(() => undefined);

    const screenshotPath = join(screenshotDir, `${spec.name}-${side}.png`);
    await page.screenshot({ path: screenshotPath, fullPage: false, animations: "disabled", caret: "hide" });
    const state = await page.evaluate(() => ({
      url: window.location.href,
      pathname: window.location.pathname,
      text: (document.body.innerText || document.body.textContent || "").replace(/\s+/g, " ").trim()
    }));
    return {
      status: response?.status() ?? null,
      url: state.url,
      pathname: state.pathname,
      text: state.text.slice(0, 500),
      screenshotPath,
      consoleErrors,
      failedRequests,
      badResponses
    };
  } finally {
    await context.close();
  }
}

async function normalizeInvalidSessionErrorID(page: Page): Promise<void> {
  await page.evaluate(() => {
    for (const element of document.querySelectorAll(".legacy-invalid-session-error-id")) {
      element.textContent = "00000";
    }
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      node.textContent = (node.textContent ?? "").replace(/Error-ID:\s*\d+/, "Error-ID: 00000");
    }
  });
}

function compareCaptures(spec: SessionExpiryCase, legacy: CaptureResult, migrated: CaptureResult, diff: DiffResult): string[] {
  const errors = [
    ...legacy.consoleErrors.map((item) => `legacy console: ${item}`),
    ...migrated.consoleErrors.map((item) => `migrated console: ${item}`),
    ...legacy.failedRequests.map((item) => `legacy failed request: ${item}`),
    ...migrated.failedRequests.map((item) => `migrated failed request: ${item}`),
    ...legacy.badResponses.map((item) => `legacy bad response: ${item}`),
    ...migrated.badResponses.map((item) => `migrated bad response: ${item}`)
  ];
  if (diff.diffRatio > maxDiffRatio) {
    errors.push(`diff ratio ${formatNumber(diff.diffRatio)} exceeded ${formatNumber(maxDiffRatio)}`);
  }
  if (spec.expectation === "public-home") {
    if (!isPublicHomePath(legacy.pathname) || !isPublicHomePath(migrated.pathname)) {
      errors.push(`expected public home redirect, got legacy=${legacy.pathname} migrated=${migrated.pathname}`);
    }
    if (!legacy.text.includes("Choose your language") || !migrated.text.includes("Choose your language")) {
      errors.push("public home redirect did not render the legacy language selector");
    }
  } else {
    const requiredTexts = ["An error occurred", "The session is invalid.", "Error-ID:"];
    for (const text of requiredTexts) {
      if (!legacy.text.includes(text) || !migrated.text.includes(text)) {
        errors.push(`invalid-session text mismatch for ${text}`);
      }
    }
    if (/Overview|Resources|Merchant|Messages/.test(migrated.text)) {
      errors.push("migrated invalid-session page leaked authenticated game chrome");
    }
  }
  return errors;
}

function shouldRetryExactVisual(comparisons: string[]): boolean {
  return comparisons.length === 1 && comparisons[0].startsWith("diff ratio ");
}

async function newContext(browser: Browser): Promise<BrowserContext> {
  return await browser.newContext({
    viewport: { width: 1024, height: 768 },
    deviceScaleFactor: 1
  });
}

function isPublicHomePath(pathname: string): boolean {
  return pathname === "/" || pathname.endsWith("/home.php");
}

function ignoredBadResponse(url: string, status?: number): boolean {
  try {
    const pathname = new URL(url).pathname;
    return pathname.endsWith("/favicon.ico") || (status === 401 && pathname === "/api/game/overview");
  } catch {
    return false;
  }
}

function ignoredConsoleError(text: string): boolean {
  return (
    text === "Failed to load resource: the server responded with a status of 404 (Not Found)" ||
    text === "Failed to load resource: the server responded with a status of 401 (Unauthorized)"
  );
}

function renderMarkdown(report: {
  generatedAt: string;
  browserName: BrowserName;
  browserExecutable: string;
  pass: boolean;
  thresholds: { maxDiffRatio: number; colorDeltaThreshold: number };
  results: CaseResult[];
}): string {
  const lines = [
    "# Session Expiry Visual Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Result: ${report.pass ? "PASS" : "FAIL"}`,
    `- Max diff ratio: ${formatNumber(report.thresholds.maxDiffRatio)}`,
    "",
    "| Case | Result | Diff Ratio | Changed Pixels | Legacy URL | Migrated URL |",
    "| --- | --- | --- | --- | --- | --- |"
  ];
  for (const result of report.results) {
    lines.push(
      `| ${result.name} | ${result.pass ? "PASS" : "FAIL"} | ${formatNumber(result.diff.diffRatio)} | ${result.diff.changedPixels} | ${escapeCell(result.legacy.url)} | ${escapeCell(result.migrated.url)} |`
    );
  }
  const failures = report.results.filter((item) => item.comparisons.length > 0);
  if (failures.length > 0) {
    lines.push("", "## Differences");
    for (const result of failures) {
      lines.push("", `### ${result.name}`, ...result.comparisons.map((item) => `- ${item}`));
    }
  }
  return `${lines.join("\n")}\n`;
}

function escapeCell(value: string): string {
  return value.replace(/\|/g, "\\|");
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name] ?? fallback;
  return raw === "firefox" ? "firefox" : "chromium";
}
