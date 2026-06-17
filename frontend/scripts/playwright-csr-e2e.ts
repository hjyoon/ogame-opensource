import { chromium, firefox } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type StepResult = {
  name: string;
  pass: boolean;
  details: Record<string, unknown>;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-csr", browserName);
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

const results: StepResult[] = [];

try {
  const page = await browser.newPage({ viewport: { width: 1024, height: 768 }, deviceScaleFactor: 1, locale: "en-US" });
  page.on("console", (message) => {
    if (message.type() === "error") {
      results.push({ name: `console error: ${message.text()}`, pass: false, details: {} });
    }
  });
  page.on("requestfailed", (request) => {
    results.push({ name: `request failed: ${request.url()}`, pass: false, details: { method: request.method(), error: request.failure()?.errorText } });
  });

  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "domcontentloaded", timeout: 15_000 });
  await record("initial legacy CSS before app idle", async () => publicChromeState(page));
  await page.waitForLoadState("networkidle");
  await record("legacy CSS remains after app idle", async () => publicChromeState(page));

  await assertClientNavigation(page, "mainmenu about alias", "#mainmenu a[href='about.php']", "/about.php");
  await assertClientNavigation(page, "mainmenu screenshots alias", "#mainmenu a[href='screenshots.php']", "/screenshots.php");
  await assertClientNavigation(page, "downmenu rules alias", "#downmenu a[href='regeln.php']", "/regeln.php");

  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  await assertClientNavigation(page, "home register CTA", "#register.bigbutton", "/register.php");

  const report = {
    generatedAt: new Date().toISOString(),
    migratedBaseURL,
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    allPass: results.every((result) => result.pass),
    results
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(JSON.stringify({ allPass: report.allPass, report: join(outputDir, "report.json") }, null, 2) + "\n");
  if (!report.allPass) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

async function assertClientNavigation(page: Awaited<ReturnType<typeof browser.newPage>>, name: string, selector: string, expectedPathname: string) {
  const marker = `probe-${name}`;
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator(selector).click();
  await page.waitForFunction((pathname) => window.location.pathname === pathname, expectedPathname, { timeout: 5_000 });
  await record(name, async () => {
    const state = await page.evaluate(() => ({
      pathname: window.location.pathname,
      probe: window.__ogameCsrProbe,
      legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
      legacyBody: document.body.classList.contains("legacy-public-body")
    }));
    return {
      pass: state.pathname === expectedPathname && state.probe === marker && state.legacyCssLinks === 2 && state.legacyBody,
      details: state
    };
  });
}

async function publicChromeState(page: Awaited<ReturnType<typeof browser.newPage>>) {
  const state = await page.evaluate(() => ({
    legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
    legacyBody: document.body.classList.contains("legacy-public-body"),
    bodyBackground: getComputedStyle(document.body).backgroundImage
  }));
  return {
    pass: state.legacyCssLinks === 2 && state.legacyBody && state.bodyBackground.includes("sterne_bg2.jpg"),
    details: state
  };
}

async function record(name: string, run: () => Promise<{ pass: boolean; details: Record<string, unknown> }>) {
  try {
    const result = await run();
    results.push({ name, ...result });
  } catch (error) {
    results.push({ name, pass: false, details: { error: error instanceof Error ? error.message : String(error) } });
  }
}

function renderMarkdown(report: {
  generatedAt: string;
  migratedBaseURL: string;
  browserName: BrowserName;
  browserExecutable: string;
  allPass: boolean;
  results: StepResult[];
}) {
  const lines = [
    "# Playwright CSR E2E Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    "",
    "| Step | Pass | Details |",
    "| --- | --- | --- |"
  ];
  for (const result of report.results) {
    lines.push(`| ${result.name} | ${result.pass ? "PASS" : "FAIL"} | ${JSON.stringify(result.details).replaceAll("|", "\\|")} |`);
  }
  lines.push("");
  return lines.join("\n");
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name];
  if (raw === "chromium" || raw === "firefox") {
    return raw;
  }
  return fallback;
}

declare global {
  interface Window {
    __ogameCsrProbe?: string;
  }
}
