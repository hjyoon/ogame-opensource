import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync, readFileSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type FixtureUser = {
  login: string;
  display: string;
  player_id: number;
  home_planet_id: number;
  admin: number;
};

type Fixture = {
  password: string;
  users: Record<string, FixtureUser>;
};

type LoginAuth = {
  role: string;
  user: FixtureUser;
  cookiePair: string;
  redirectTo: string;
  sessionSearch: string;
};

type BrowserSignals = {
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
};

type StepResult = {
  name: string;
  pass: boolean;
  details: Record<string, unknown>;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-user-types", browserName);
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const fixtureFile = resolve(process.cwd(), process.env.OGAME_USER_TYPE_FIXTURE_FILE ?? join(rootDir, ".tmp/golang-user-type-fixture.json"));
const fixture = JSON.parse(readFileSync(fixtureFile, "utf8")) as Fixture;
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
  const universe = await loadUniverse();

  await withAuthenticatedPage(browser, "player", universe, async (page, auth, signals) => {
    await gotoGame(page, auth, "/game/admin");
    const body = await page.locator("body").innerText();
    const finalPath = new URL(page.url()).pathname;
    const adminLinks = await page.locator(".legacy-menu a[href^='/game/admin']").count();
    const adminShells = await page.locator(".legacy-admin-home-table, .legacy-admin-mode-shell").count();
    record("regular player admin screen is denied", {
      pass: finalPath === "/game/overview" && adminLinks === 0 && adminShells === 0 && !body.includes("*Admin Area*") && signalsClean(signals),
      details: { finalPath, adminLinks, adminShells, signals }
    });
  });

  await withAuthenticatedPage(browser, "operator", universe, async (page, auth, signals) => {
    await gotoGame(page, auth, "/game/admin");
    await page.locator(".legacy-admin-home-table").waitFor({ timeout: 10_000 });
    const homeText = await page.locator("body").innerText();
    const homePass = homeText.includes("Fleet Logs") && homeText.includes("Botstrat Editor");

    await gotoGame(page, auth, "/game/admin", { mode: "Users" });
    await page.locator(".legacy-admin-users-table").waitFor({ timeout: 10_000 });
    const usersText = await page.locator("body").innerText();

    await gotoGame(page, auth, "/game/admin", { mode: "BotEdit" });
    await page.getByText("Access denied.").waitFor({ timeout: 10_000 });
    const botEditText = await page.locator("body").innerText();
    const botEditTables = await page.locator(".legacy-admin-botedit-table").count();
    const botEditPath = new URL(page.url()).pathname;
    record("operator admin screens match role boundary", {
      pass:
        homePass &&
        usersText.includes("New users:") &&
        botEditPath === "/game/admin" &&
        botEditText.includes("Access denied.") &&
        botEditTables === 0 &&
        signalsClean(signals),
      details: { homePass, botEditPath, botEditTables, signals }
    });
  });

  await withAuthenticatedPage(browser, "admin", universe, async (page, auth, signals) => {
    await gotoGame(page, auth, "/game/admin", { mode: "Uni" });
    await page.locator(".legacy-admin-universe-table").waitFor({ timeout: 10_000 });
    const uniText = await page.locator("body").innerText();

    await gotoGame(page, auth, "/game/admin", { mode: "BotEdit" });
    await page.locator(".legacy-admin-botedit-table").waitFor({ timeout: 10_000 });
    const botEditText = await page.locator("body").innerText();
    const botEditTables = await page.locator(".legacy-admin-botedit-table").count();
    record("administrator admin-only screens render", {
      pass:
        uniText.includes("Universe 1 Settings") &&
        botEditTables === 1 &&
        botEditText.includes("Name of the edited strategy:") &&
        !botEditText.includes("Access denied.") &&
        signalsClean(signals),
      details: { botEditTables, signals }
    });
  });

  await withAuthenticatedPage(browser, "unvalidated", universe, async (page, auth, signals) => {
    await gotoGame(page, auth, "/game/options");
    await page.locator(".legacy-options-table").waitFor({ timeout: 10_000 });
    const body = await page.locator("body").innerText();
    record("unvalidated account renders options screen", {
      pass: body.includes("User Data") && body.includes("General Options") && !body.includes("Session is invalid.") && signalsClean(signals),
      details: { signals }
    });
  });

  await withAuthenticatedPage(browser, "vacation", universe, async (page, auth, signals) => {
    await gotoGame(page, auth, "/game/options");
    await page.locator(".legacy-options-table").waitFor({ timeout: 10_000 });
    const vacationChecked = await page.locator("input[name='urlaubs_modus']").isChecked();

    await gotoGame(page, auth, "/game/buildings");
    await page.locator("[data-building-row='1']").waitFor({ timeout: 10_000 });
    const buildLinks = await page.locator("[data-building-row='1'] .legacy-building-action a").count();
    let vacationIssueVisible = false;
    if (buildLinks > 0) {
      await page.locator("[data-building-row='1'] .legacy-building-action a").first().click();
      const vacationIssue = page.getByText("Vacation mode is active.").first();
      await vacationIssue.waitFor({ timeout: 10_000 });
      vacationIssueVisible = await vacationIssue.isVisible();
    }
    record("vacation account renders state and blocks build action", {
      pass: vacationChecked && buildLinks > 0 && vacationIssueVisible && signalsClean(signals),
      details: { vacationChecked, buildLinks, vacationIssueVisible, signals }
    });
  });

  await withAuthenticatedPage(browser, "deletion_queued", universe, async (page, auth, signals) => {
    await gotoGame(page, auth, "/game/options");
    await page.locator(".legacy-options-table").waitFor({ timeout: 10_000 });
    const deletionChecked = await page.locator("input[name='db_deaktjava']").isChecked();
    const body = await page.locator("body").innerText();
    record("deletion-queued account renders delete-account state", {
      pass: deletionChecked && body.includes("Delete account") && body.includes("am:") && signalsClean(signals),
      details: { deletionChecked, signals }
    });
  });

  await assertBannedLogin(universe);

  const report = {
    generatedAt: new Date().toISOString(),
    migratedBaseURL,
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    fixture: fixtureFile,
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

async function assertBannedLogin(universe: string) {
  const context = await browser.newContext({ viewport: { width: 1024, height: 768 }, deviceScaleFactor: 1, locale: "en-US" });
  const signals = newSignals();
  const page = await newPage(context, signals);
  try {
    const user = fixture.users.banned;
    await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
    await page.locator("select[name='universe']").selectOption(universe);
    await page.locator("input[name='login']").fill(user.login);
    await page.locator("input[name='pass']").fill(fixture.password);
    await page.locator(".legacy-public-login-button").click();
    await page.locator(".legacy-public-login-feedback").waitFor({ timeout: 10_000 });
    const body = await page.locator("body").innerText();
    const cookies = await context.cookies();
    record("banned account stays on login screen with error", {
      pass:
        page.url().startsWith(`${migratedBaseURL}/home`) &&
        body.includes("Commander account is banned.") &&
        cookies.every((cookie) => !cookie.name.startsWith("prsess_")) &&
        signalsClean(signals),
      details: { url: page.url(), cookies: cookies.map((cookie) => cookie.name), signals }
    });
  } finally {
    await context.close();
  }
}

async function withAuthenticatedPage(
  activeBrowser: Browser,
  role: string,
  universe: string,
  run: (page: Page, auth: LoginAuth, signals: BrowserSignals) => Promise<void>
) {
  const auth = await login(role, universe);
  const context = await activeBrowser.newContext({ viewport: { width: 1024, height: 768 }, deviceScaleFactor: 1, locale: "en-US" });
  const [cookieName, cookieValue] = auth.cookiePair.split("=");
  await context.addCookies([{ name: cookieName, value: cookieValue, url: migratedBaseURL, httpOnly: true, sameSite: "Lax" }]);
  const signals = newSignals();
  const page = await newPage(context, signals);
  try {
    await run(page, auth, signals);
  } finally {
    await context.close();
  }
}

async function newPage(context: BrowserContext, signals: BrowserSignals): Promise<Page> {
  const page = await context.newPage();
  page.on("console", (message) => {
    if (message.type() === "error") {
      signals.consoleErrors.push(message.text());
    }
  });
  page.on("requestfailed", (request) => {
    signals.failedRequests.push(`${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`.trim());
  });
  page.on("response", (response) => {
    if (response.status() >= 400) {
      signals.badResponses.push(`${response.status()} ${response.url()}`);
    }
  });
  return page;
}

async function loadUniverse(): Promise<string> {
  const response = await fetch(`${migratedBaseURL}/api/public/universes`);
  const payload = await response.json();
  return payload.universes?.[0]?.baseUrl ?? "http://localhost:8888";
}

async function login(role: string, universe: string): Promise<LoginAuth> {
  const user = fixture.users[role];
  if (!user) {
    throw new Error(`Missing fixture user for role ${role}`);
  }
  const response = await fetch(`${migratedBaseURL}/api/public/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ login: user.login, pass: fixture.password, universe })
  });
  const payload = await response.json();
  const cookie = response.headers.get("set-cookie") ?? "";
  const cookiePair = cookie.split(";")[0] ?? "";
  if (response.status !== 200 || payload.valid !== true || !cookiePair.startsWith(`prsess_${user.player_id}_`)) {
    throw new Error(`Unable to login ${role}: ${JSON.stringify({ status: response.status, payload, cookiePair })}`);
  }
  const redirectTo = String(payload.session?.redirectTo ?? "");
  const sessionSearch = new URL(redirectTo, migratedBaseURL).search;
  return { role, user, cookiePair, redirectTo, sessionSearch };
}

async function gotoGame(page: Page, auth: LoginAuth, path: string, query: Record<string, string> = {}) {
  const search = new URLSearchParams(auth.sessionSearch);
  for (const [key, value] of Object.entries(query)) {
    search.set(key, value);
  }
  await page.goto(`${migratedBaseURL}${path}?${search.toString()}`, { waitUntil: "networkidle", timeout: 15_000 });
  await page.locator("#content").waitFor({ timeout: 10_000 });
}

function record(name: string, result: { pass: boolean; details: Record<string, unknown> }) {
  results.push({ name, pass: result.pass, details: result.details });
}

function newSignals(): BrowserSignals {
  return { consoleErrors: [], failedRequests: [], badResponses: [] };
}

function signalsClean(signals: BrowserSignals): boolean {
  return signals.consoleErrors.length === 0 && signals.failedRequests.length === 0 && signals.badResponses.length === 0;
}

function renderMarkdown(report: { generatedAt: string; migratedBaseURL: string; browserName: string; allPass: boolean; results: StepResult[] }) {
  const lines = [
    "# Playwright User Type E2E",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Base URL: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    "",
    "| Step | Result |",
    "| --- | --- |"
  ];
  for (const result of report.results) {
    lines.push(`| ${result.name} | ${result.pass ? "PASS" : "FAIL"} |`);
  }
  return `${lines.join("\n")}\n`;
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const value = process.env[name];
  return value === "firefox" || value === "chromium" ? value : fallback;
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
