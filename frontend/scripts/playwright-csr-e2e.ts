import { chromium, firefox, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type StepResult = {
  name: string;
  pass: boolean;
  details: Record<string, unknown>;
};

type LoginFixture = {
  login: string;
  password: string;
  universe: string;
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
  await assertPublicLanguageFlagReload(page);

  await assertClientNavigation(page, "mainmenu about alias", "#mainmenu a[href='about.php']", "/about.php");
  await assertClientNavigation(page, "mainmenu screenshots alias", "#mainmenu a[href='screenshots.php']", "/screenshots.php");
  await assertClientNavigation(page, "downmenu rules alias", "#downmenu a[href='regeln.php']", "/regeln.php");
  await assertHistoryNavigation(page, "browser back preserves CSR", "back", "/screenshots.php");
  await assertHistoryNavigation(page, "browser forward preserves CSR", "forward", "/regeln.php");

  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  await assertClientNavigation(page, "home register CTA", "#register.bigbutton", "/register.php");

  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  const loginFixture = await createLoginFixture();
  await assertLoginFormRedirectsToGame(page, loginFixture);
  await record("game overview shell loads with session", async () => gameShellState(page, "login-form-submit", "Overview"));
  await assertGameClientNavigation(page, "game buildings menu preserves CSR", "a[href^='/game/buildings']", "/game/buildings", "Buildings");
  await assertGameClientNavigation(page, "game resources menu preserves CSR", "a[href^='/game/resources']", "/game/resources", "Resources");
  await assertGameClientNavigation(page, "game research menu preserves CSR", "a[href^='/game/research']", "/game/research", "Research");
  await assertGameClientNavigation(page, "game shipyard menu preserves CSR", "a[href^='/game/shipyard']", "/game/shipyard", "Shipyard");
  await assertGameClientNavigation(page, "game fleet menu preserves CSR", "a[href^='/game/fleet']", "/game/fleet", "Fleet");
  await assertGameClientNavigation(page, "game overview menu preserves CSR", "a[href^='/game/overview']", "/game/overview", "Overview");

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

async function assertClientNavigation(page: Page, name: string, selector: string, expectedPathname: string) {
  const marker = `probe-${name}`;
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator(selector).click();
  await page.waitForFunction((pathname) => window.location.pathname === pathname, expectedPathname, { timeout: 5_000 });
  await record(name, async () => {
    const state = await csrState(page);
    return {
      pass: state.pathname === expectedPathname && state.probe === marker && state.legacyCssLinks === 2 && state.legacyBody,
      details: state
    };
  });
}

async function assertHistoryNavigation(page: Page, name: string, direction: "back" | "forward", expectedPathname: string) {
  const marker = `probe-${name}`;
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  if (direction === "back") {
    await page.goBack();
  } else {
    await page.goForward();
  }
  await page.waitForFunction((pathname) => window.location.pathname === pathname, expectedPathname, { timeout: 5_000 });
  await record(name, async () => {
    const state = await csrState(page);
    return {
      pass: state.pathname === expectedPathname && state.probe === marker && state.legacyCssLinks === 2 && state.legacyBody,
      details: state
    };
  });
}

async function assertLoginFormRedirectsToGame(page: Page, fixture: LoginFixture) {
  await page.evaluate(() => {
    window.__ogameCsrProbe = "login-form-submit";
  });
  await page.locator("select[name='universe']").selectOption(fixture.universe);
  await page.locator("input[name='login']").fill(fixture.login);
  await page.locator("input[name='pass']").fill(fixture.password);
  await page.locator("input.legacy-public-login-button").click();
  await page.waitForFunction(() => window.location.pathname === "/game/overview" && window.location.search.includes("session="), undefined, {
    timeout: 10_000
  });
  await record("login form redirects directly to game", async () => {
    const state = await gameShellState(page, "login-form-submit", "Overview");
    return {
      pass: state.pass && state.details.openOverviewLinks === 0,
      details: state.details
    };
  });
}

async function assertPublicLanguageFlagReload(page: Page) {
  await page.context().clearCookies();
  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  await page.evaluate(() => {
    window.__ogameCsrProbe = "language-flag-before";
  });
  const navigation = page
    .waitForNavigation({ waitUntil: "domcontentloaded", timeout: 5_000 })
    .then(() => true)
    .catch(() => false);
  await page.locator("a:has(img[alt='Deutschland'])").click();
  const reloaded = await navigation;
  await page.waitForLoadState("networkidle").catch(() => undefined);
  await record("public language flag reloads like legacy", async () => {
    const state = await page.evaluate(() => ({
      pathname: window.location.pathname,
      hash: window.location.hash,
      cookie: document.cookie,
      probe: window.__ogameCsrProbe,
      legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
      legacyBody: document.body.classList.contains("legacy-public-body")
    }));
    return {
      pass:
        reloaded &&
        state.pathname === "/home" &&
        state.hash === "" &&
        state.cookie.split("; ").includes("ogamelang=de") &&
        state.probe === undefined &&
        state.legacyCssLinks === 2 &&
        state.legacyBody,
      details: { ...state, reloaded }
    };
  });
}

async function createLoginFixture(): Promise<LoginFixture> {
  const universesResponse = await fetch(`${migratedBaseURL}/api/public/universes`);
  const universesPayload = await universesResponse.json();
  const universe = universesPayload.universes?.[0]?.baseUrl ?? "http://localhost:8888";
  const suffix = `${browserName.slice(0, 2)}${Date.now().toString(36).slice(-8)}${Math.random().toString(36).slice(2, 4)}`;
  const login = `Csr${suffix}`.slice(0, 20);
  const password = "E2E_http123";
  const registrationResponse = await fetch(`${migratedBaseURL}/api/public/registration`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: login,
      password,
      email: `${login.toLowerCase()}@example.local`,
      universe,
      agb: true
    })
  });
  const registrationPayload = await registrationResponse.json();
  if (registrationResponse.status !== 200 || registrationPayload.valid !== true || registrationPayload.created !== true) {
    throw new Error(`Unable to create CSR login fixture: ${JSON.stringify(registrationPayload)}`);
  }
  return { login, password, universe };
}

async function assertGameClientNavigation(
  page: Page,
  name: string,
  selector: string,
  expectedPathname: string,
  expectedMenuLabel: string
) {
  const marker = `probe-${name}`;
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator(selector).click();
  await page.waitForFunction((pathname) => window.location.pathname === pathname, expectedPathname, { timeout: 5_000 });
  if (expectedMenuLabel === "Buildings") {
    await page.locator("[data-building-row]").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Resources") {
    await page.locator(".legacy-resources-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Research") {
    await page.locator(".legacy-research-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Shipyard") {
    await page.locator(".legacy-shipyard-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel !== "Overview") {
    await page.waitForFunction(() => document.body.textContent?.includes("queued for React and Go migration"), undefined, {
      timeout: 5_000
    });
  }
  await record(name, async () => {
    const state = await gameShellState(page, marker, expectedMenuLabel);
    const contentReady =
      expectedMenuLabel === "Buildings"
        ? state.details.buildingRows > 0 && state.details.buildingNames.includes("Metal Mine") && state.details.pendingText === false
        : expectedMenuLabel === "Resources"
          ? state.details.resourcesTable === true && state.details.resourceSettingsText === true && state.details.pendingText === false
          : expectedMenuLabel === "Research"
            ? state.details.researchTable === true && state.details.researchRows >= 0 && state.details.pendingText === false
            : expectedMenuLabel === "Shipyard"
              ? state.details.shipyardTable === true && state.details.shipyardRows >= 0 && state.details.pendingText === false
              : expectedMenuLabel === "Overview"
                ? state.details.pendingText === false
                : state.details.pendingText === true;
    return {
      pass:
        state.details.pathname === expectedPathname &&
        state.details.probe === marker &&
        state.details.search.includes("session=") &&
        state.details.gameShell === true &&
        state.details.activeMenuLabel === expectedMenuLabel &&
        contentReady &&
        state.details.legacyCssLinks === 0 &&
        state.details.legacyBody === false,
      details: state.details
    };
  });
}

async function publicChromeState(page: Page) {
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

async function gameShellState(page: Page, expectedProbe: string, expectedMenuLabel: string) {
  const state = await page.evaluate(() => ({
    pathname: window.location.pathname,
    search: window.location.search,
    probe: window.__ogameCsrProbe,
    gameShell: document.querySelector(".legacy-game-shell") !== null,
    activeMenuLabel: document.querySelector(".legacy-menu a[aria-current='page']")?.textContent?.trim() ?? "",
    legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
    legacyBody: document.body.classList.contains("legacy-public-body"),
    openOverviewLinks: Array.from(document.querySelectorAll("a")).filter((link) => link.textContent?.trim() === "Open overview").length,
    buildingRows: document.querySelectorAll("[data-building-row]").length,
    buildingNames: Array.from(document.querySelectorAll("[data-building-row] .legacy-building-description a")).map(
      (link) => link.textContent?.trim() ?? ""
    ),
    resourceRows: document.querySelectorAll("[data-resource-row]").length,
    resourcesTable: document.querySelector(".legacy-resources-table") !== null,
    resourceSettingsText: document.body.textContent?.includes("Resource settings on planet") ?? false,
    resourceNames: Array.from(document.querySelectorAll("[data-resource-row] th:first-child")).map(
      (cell) => cell.textContent?.trim().replace(/\s+/g, " ") ?? ""
    ),
    researchRows: document.querySelectorAll("[data-research-row]").length,
    researchTable: document.querySelector(".legacy-research-table") !== null,
    researchNames: Array.from(document.querySelectorAll("[data-research-row] .legacy-building-description a")).map(
      (link) => link.textContent?.trim() ?? ""
    ),
    shipyardRows: document.querySelectorAll("[data-shipyard-row]").length,
    shipyardTable: document.querySelector(".legacy-shipyard-table") !== null,
    shipyardNames: Array.from(document.querySelectorAll("[data-shipyard-row] .legacy-building-description a")).map(
      (link) => link.textContent?.trim() ?? ""
    ),
    pendingText: document.body.textContent?.includes("queued for React and Go migration") ?? false
  }));
  return {
    pass:
      state.gameShell &&
      state.search.includes("session=") &&
      state.activeMenuLabel === expectedMenuLabel &&
      state.probe === expectedProbe &&
      state.legacyCssLinks === 0 &&
      !state.legacyBody,
    details: state
  };
}

async function csrState(page: Page) {
  return await page.evaluate(() => ({
    pathname: window.location.pathname,
    probe: window.__ogameCsrProbe,
    legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
    legacyBody: document.body.classList.contains("legacy-public-body")
  }));
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
