import { chromium, firefox, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import {
  compareScreenshots,
  deterministicScreenshotCSS,
  trimTrailingSlash,
  waitForImages,
  waitForStablePaint,
  type DiffResult
} from "./visual/game-visual-utils";

type BrowserName = "chromium" | "firefox";
type SideName = "legacy" | "migrated";

type LoginFailureCapture = {
  url: string;
  status: number | null;
  text: string;
  screenshotPath: string;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
};

type LanguageFlagSpec = {
  label: string;
  cookieValue: string;
};

type LanguageFlagCapture = {
  url: string;
  status: number | null;
  beforePathname: string;
  afterPathname: string;
  afterHash: string;
  cookieValue: string;
  reloaded: boolean;
  probeAfter?: string;
  localizedText: Record<string, string>;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
};

type LanguageFlagComparison = {
  name: string;
  pass: boolean;
  mismatches: string[];
  legacy: LanguageFlagCapture;
  migrated: LanguageFlagCapture;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, `.tmp/playwright-public-login-dynamic/${browserName}`);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const maxDiffRatio = numberEnv("OGAME_PUBLIC_LOGIN_MAX_DIFF_RATIO", 0);
const colorDeltaThreshold = numberEnv("OGAME_PUBLIC_LOGIN_COLOR_DELTA", 0);
const invalidLogin = process.env.OGAME_PUBLIC_LOGIN_BAD_USER ?? "definitelybad";
const invalidPassword = process.env.OGAME_PUBLIC_LOGIN_BAD_PASS ?? "wrong-password";
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const languageFlags: LanguageFlagSpec[] = [
  { label: "Deutschland", cookieValue: "de" },
  { label: "English", cookieValue: "en" },
  { label: "France", cookieValue: "fr" },
  { label: "Italy", cookieValue: "it" },
  { label: "Russia", cookieValue: "ru" }
];

await mkdir(screenshotDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const legacyContext = await newContext(legacyBaseURL);
  const migratedContext = await newContext(migratedBaseURL);
  const legacy = await captureInvalidLogin(legacyContext, "legacy");
  const migrated = await captureInvalidLogin(migratedContext, "migrated");
  await legacyContext.close();
  await migratedContext.close();
  const languageFlagComparisons: LanguageFlagComparison[] = [];
  for (const flag of languageFlags) {
    const legacyFlag = await captureLanguageFlagClick("legacy", flag);
    const migratedFlag = await captureLanguageFlagClick("migrated", flag);
    languageFlagComparisons.push(compareLanguageFlagCaptures(flag, legacyFlag, migratedFlag));
  }

  const diffPath = join(screenshotDir, "invalid-login-diff.png");
  const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath, colorDeltaThreshold);
  const comparisons = compareCaptures(legacy, migrated, diff);
  const pass =
    comparisons.length === 0 &&
    legacy.status === 200 &&
    migrated.status === 200 &&
    legacy.consoleErrors.length === 0 &&
    migrated.consoleErrors.length === 0 &&
    legacy.failedRequests.length === 0 &&
    migrated.failedRequests.length === 0 &&
    legacy.badResponses.length === 0 &&
    migrated.badResponses.length === 0 &&
    diff.diffRatio <= maxDiffRatio &&
    languageFlagComparisons.every((comparison) => comparison.pass);

  const report = {
    generatedAt: new Date().toISOString(),
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    legacyBaseURL,
    migratedBaseURL,
    invalidLogin,
    thresholds: { maxDiffRatio, colorDeltaThreshold },
    pass,
    legacy,
    migrated,
    diff,
    diffPath,
    comparisons,
    languageFlagComparisons
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(JSON.stringify({ pass, diffRatio: diff.diffRatio, changedPixels: diff.changedPixels, report: join(outputDir, "report.json") }, null, 2) + "\n");
  if (!pass) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

async function captureLanguageFlagClick(side: SideName, flag: LanguageFlagSpec): Promise<LanguageFlagCapture> {
  const baseURL = side === "legacy" ? legacyBaseURL : migratedBaseURL;
  const context = await newContext(baseURL);
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
    if (status >= 400 && !ignoredBadResponse(response.url())) {
      badResponses.push(`${status} ${response.url()}`);
    }
  });
  try {
    const response = await page.goto(homeURL(side), { waitUntil: "networkidle", timeout: 20_000 });
    const beforePathname = await page.evaluate(() => window.location.pathname);
    await page.evaluate(() => {
      (window as Window & { __ogamePublicLanguageProbe?: string }).__ogamePublicLanguageProbe = "before";
    });
    const navigation = page
      .waitForNavigation({ waitUntil: "domcontentloaded", timeout: 5_000 })
      .then(() => true)
      .catch(() => false);
    await page.locator(`a:has(img[alt='${flag.label}'])`).click();
    await navigation;
    await page.waitForLoadState("networkidle", { timeout: 20_000 }).catch(() => undefined);
    await page.waitForTimeout(50);
    const state = await page.evaluate(() => ({
      url: window.location.href,
      afterPathname: window.location.pathname,
      afterHash: window.location.hash,
      cookieValue:
        document.cookie
          .split("; ")
          .find((cookie) => cookie.startsWith("ogamelang="))
          ?.split("=")[1] ?? "",
      probeAfter: (window as Window & { __ogamePublicLanguageProbe?: string }).__ogamePublicLanguageProbe,
      localizedText: {
        chooseLanguage: document.querySelector(".products a:last-child")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        mainMenu: document.querySelector("#mainmenu")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        homeTitle: document.querySelector("#title")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        homeText1: document.querySelector("#text1")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        homeText2: document.querySelector("#text2")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        registerButton: document.querySelector("#register")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        loginLabels: document.querySelector("#login_text_1")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        loginLinks: document.querySelector("#login_text_2")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        firstUniverseOption: document.querySelector("select[name='universe'] option")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        copyright: document.querySelector("#copyright")?.textContent?.replace(/\s+/g, " ").trim() ?? "",
        downmenu: document.querySelector("#downmenu")?.textContent?.replace(/\s+/g, " ").trim() ?? ""
      }
    }));
    return {
      url: state.url,
      status: response?.status() ?? null,
      beforePathname,
      afterPathname: state.afterPathname,
      afterHash: state.afterHash,
      cookieValue: state.cookieValue,
      reloaded: state.probeAfter === undefined,
      probeAfter: state.probeAfter,
      localizedText: state.localizedText,
      consoleErrors,
      failedRequests,
      badResponses
    };
  } finally {
    await page.close().catch(() => undefined);
    await context.close().catch(() => undefined);
  }
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

async function captureInvalidLogin(context: BrowserContext, side: SideName): Promise<LoginFailureCapture> {
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
    if (status >= 400 && !ignoredBadResponse(response.url())) {
      badResponses.push(`${status} ${response.url()}`);
    }
  });
  await page.addStyleTag({ content: deterministicScreenshotCSS });

  const response = await page.goto(homeURL(side), { waitUntil: "networkidle", timeout: 20_000 });
  await selectFirstUniverse(page);
  await page.locator("input[name='login']").fill(invalidLogin);
  await page.locator("input[name='pass']").fill(invalidPassword);
  await Promise.all([
    page.waitForURL(/\/game\/reg\/errorpage\.php\?/, { timeout: 20_000 }),
    page.locator("input.loginButton, input.legacy-public-login-button").click()
  ]);
  await page.waitForLoadState("networkidle", { timeout: 20_000 }).catch(() => undefined);
  await page.locator("table").filter({ hasText: "This account does not exist" }).waitFor({ timeout: 10_000 });
  await waitForImages(page);
  await waitForStablePaint(page);
  await page.waitForTimeout(100);

  const screenshotPath = join(screenshotDir, `invalid-login-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false, animations: "disabled" });
  const text = compact(await page.locator("body").innerText({ timeout: 5_000 }));
  const currentURL = page.url();
  await page.close();
  return {
    url: currentURL,
    status: response?.status() ?? null,
    text,
    screenshotPath,
    consoleErrors,
    failedRequests,
    badResponses
  };
}

async function selectFirstUniverse(page: Page): Promise<void> {
  const selector = "select[name='universe']";
  await page.locator(selector).waitFor({ timeout: 10_000 });
  await page.waitForFunction(() => {
    const select = document.querySelector<HTMLSelectElement>("select[name='universe']");
    return Array.from(select?.options ?? []).some((option) => option.value !== "");
  }, null, { timeout: 10_000 });
  const value = await page.locator(`${selector} option[value]:not([value=''])`).first().getAttribute("value");
  if (!value) {
    throw new Error("public login universe option was not available");
  }
  await page.locator(selector).selectOption(value);
}

function compareLanguageFlagCaptures(flag: LanguageFlagSpec, legacy: LanguageFlagCapture, migrated: LanguageFlagCapture): LanguageFlagComparison {
  const mismatches: string[] = [];
  for (const [side, capture] of [
    ["legacy", legacy],
    ["migrated", migrated]
  ] as const) {
    if (capture.status !== 200) {
      mismatches.push(`${side} status ${capture.status}`);
    }
    if (capture.cookieValue !== flag.cookieValue) {
      mismatches.push(`${side} cookie ${capture.cookieValue}`);
    }
    if (!capture.reloaded) {
      mismatches.push(`${side} did not reload`);
    }
    if (capture.probeAfter !== undefined) {
      mismatches.push(`${side} preserved in-memory probe`);
    }
    if (capture.beforePathname !== capture.afterPathname) {
      mismatches.push(`${side} pathname ${capture.beforePathname}->${capture.afterPathname}`);
    }
    if (capture.afterHash !== "") {
      mismatches.push(`${side} hash ${capture.afterHash}`);
    }
    if (capture.consoleErrors.length > 0) {
      mismatches.push(`${side} console errors ${capture.consoleErrors.join("; ")}`);
    }
    if (capture.failedRequests.length > 0) {
      mismatches.push(`${side} failed requests ${capture.failedRequests.join("; ")}`);
    }
    if (capture.badResponses.length > 0) {
      mismatches.push(`${side} bad responses ${capture.badResponses.join("; ")}`);
    }
  }
  for (const key of new Set([...Object.keys(legacy.localizedText), ...Object.keys(migrated.localizedText)])) {
    if (legacy.localizedText[key] !== migrated.localizedText[key]) {
      mismatches.push(`localized ${key} ${JSON.stringify(legacy.localizedText[key])}/${JSON.stringify(migrated.localizedText[key])}`);
    }
  }
  return {
    name: `language flag ${flag.cookieValue}`,
    pass: mismatches.length === 0,
    mismatches,
    legacy,
    migrated
  };
}

function compareCaptures(legacy: LoginFailureCapture, migrated: LoginFailureCapture, diff: DiffResult): string[] {
  const errors: string[] = [];
  const requiredText = [
    `You tried to enter universe 1 under nickname ${invalidLogin}.`,
    "This account does not exist or you have entered your password incorrectly.",
    "Enter the correct password or use password recovery.",
    "You can also create a new account."
  ];
  for (const text of requiredText) {
    if (!legacy.text.includes(text)) {
      errors.push(`legacy missing text: ${text}`);
    }
    if (!migrated.text.includes(text)) {
      errors.push(`migrated missing text: ${text}`);
    }
  }
  if (!new URL(legacy.url).pathname.endsWith("/game/reg/errorpage.php")) {
    errors.push(`legacy did not navigate to errorpage: ${legacy.url}`);
  }
  if (!new URL(migrated.url).pathname.endsWith("/game/reg/errorpage.php")) {
    errors.push(`migrated did not navigate to errorpage: ${migrated.url}`);
  }
  if (diff.diffRatio > maxDiffRatio) {
    errors.push(`exact diff ${diff.diffRatio} (${diff.changedPixels}/${diff.totalPixels})`);
  }
  return errors;
}

function homeURL(side: SideName): string {
  return side === "legacy" ? `${legacyBaseURL}/home.php` : `${migratedBaseURL}/`;
}

function ignoredBadResponse(url: string): boolean {
  return url.endsWith("/favicon.ico") || /\/game\/reg\/css\/(?:default|formate)\.css$/.test(new URL(url).pathname);
}

function ignoredConsoleError(text: string): boolean {
  return /\/game\/reg\/css\/(?:default|formate)\.css/.test(text) || text === "Failed to load resource: the server responded with a status of 404 (Not Found)";
}

function compact(value: string): string {
  return value.replace(/\s+/g, " ").trim();
}

function renderMarkdown(report: {
  generatedAt: string;
  browserName: BrowserName;
  browserExecutable: string;
  pass: boolean;
  diff: DiffResult;
  comparisons: string[];
  languageFlagComparisons: LanguageFlagComparison[];
  legacy: LoginFailureCapture;
  migrated: LoginFailureCapture;
}) {
  const lines = [
    "# Public Login Dynamic Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Result: ${report.pass ? "PASS" : "FAIL"}`,
    `- Diff Ratio: ${report.diff.diffRatio}`,
    `- Changed Pixels: ${report.diff.changedPixels}`,
    "",
    "| Side | URL | Text |",
    "| --- | --- | --- |",
    `| Legacy | ${report.legacy.url} | ${report.legacy.text} |`,
    `| Migrated | ${report.migrated.url} | ${report.migrated.text} |`
  ];
  lines.push("", "## Language Flags", "", "| Flag | Result | Mismatches |", "| --- | --- | --- |");
  for (const comparison of report.languageFlagComparisons) {
    lines.push(`| ${comparison.name} | ${comparison.pass ? "PASS" : "FAIL"} | ${comparison.mismatches.join(", ") || "-"} |`);
  }
  if (report.comparisons.length > 0) {
    lines.push("", "## Differences", "", ...report.comparisons.map((item) => `- ${item}`));
  }
  return `${lines.join("\n")}\n`;
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name] ?? fallback;
  return raw === "firefox" ? "firefox" : "chromium";
}

function numberEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : fallback;
}
