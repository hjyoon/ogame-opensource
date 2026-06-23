import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type Fixture = {
  user: string;
  player_id: number;
  planet_id: number;
  target_planet_id: number;
  session: string;
  private_cookie_name: string;
  private_cookie_value: string;
};

type TimerRow = {
  className: string;
  timerText: string;
  seconds: number | null;
  bodyText: string;
};

type TimerSnapshot = {
  capturedAt: string;
  rows: TimerRow[];
};

type PageCapture = {
  status: number | null;
  url: string;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  before: TimerSnapshot;
  after: TimerSnapshot;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-overview-fleet-countdown", browserName);
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const fixturePath = resolve(process.env.OGAME_OVERVIEW_FLEET_FIXTURE_FILE ?? join(rootDir, ".tmp/overview-fleet-fixture.json"));
const chromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : chromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const sampleDelayMs = numberEnv("OGAME_OVERVIEW_FLEET_COUNTDOWN_DELAY_MS", 2200);
const maxCrossSideSkewSeconds = numberEnv("OGAME_OVERVIEW_FLEET_COUNTDOWN_MAX_SKEW", 3);

const fixture = JSON.parse(await readFile(fixturePath, "utf8")) as Fixture;

await mkdir(outputDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const legacyContext = await newContext(browser, legacyBaseURL, fixture);
  const legacy = await captureCountdown(legacyContext, "legacy", legacyOverviewURL(fixture));
  await legacyContext.close();

  const migratedContext = await newContext(browser, migratedBaseURL, fixture);
  const migrated = await captureCountdown(migratedContext, "migrated", migratedOverviewURL(fixture));
  await migratedContext.close();

  const checks = evaluateCountdown(legacy, migrated);
  const pass =
    checks.statusPass &&
    checks.noBrowserErrors &&
    checks.rowCountPass &&
    checks.visibleContractPass &&
    checks.timerFormatPass &&
    checks.decreasePass &&
    checks.crossSideSkewPass;

  const report = {
    generatedAt: new Date().toISOString(),
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    legacyBaseURL,
    migratedBaseURL,
    fixture: {
      user: fixture.user,
      playerID: fixture.player_id,
      planetID: fixture.planet_id,
      targetPlanetID: fixture.target_planet_id
    },
    sampleDelayMs,
    maxCrossSideSkewSeconds,
    pass,
    checks,
    legacy,
    migrated
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(
    JSON.stringify(
      {
        pass,
        legacyDeltas: checks.legacyDeltas,
        migratedDeltas: checks.migratedDeltas,
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

function legacyOverviewURL(fixtureData: Fixture): string {
  const query = new URLSearchParams({
    page: "overview",
    session: fixtureData.session,
    cp: String(fixtureData.planet_id)
  });
  return `${legacyBaseURL}/game/index.php?${query.toString()}`;
}

function migratedOverviewURL(fixtureData: Fixture): string {
  const query = new URLSearchParams({
    session: fixtureData.session,
    cp: String(fixtureData.planet_id)
  });
  return `${migratedBaseURL}/game/overview?${query.toString()}`;
}

async function newContext(browserInstance: Browser, baseURL: string, fixtureData: Fixture): Promise<BrowserContext> {
  const context = await browserInstance.newContext({
    viewport: { width: 1024, height: 768 },
    deviceScaleFactor: 1,
    locale: "en-US"
  });
  await context.addCookies([
    {
      name: fixtureData.private_cookie_name,
      value: fixtureData.private_cookie_value,
      url: baseURL
    }
  ]);
  return context;
}

async function captureCountdown(context: BrowserContext, side: "legacy" | "migrated", url: string): Promise<PageCapture> {
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

  const readySelector = side === "legacy" ? "#content div[id^='bxx']" : ".legacy-overview-event-timer";
  const response = await page.goto(url, { waitUntil: "networkidle", timeout: 15_000 });
  await page.locator(readySelector).first().waitFor({ timeout: 10_000 });
  await waitForImages(page);
  await page.waitForTimeout(250);
  const before = await timerSnapshot(page, side);
  await page.waitForTimeout(sampleDelayMs);
  const after = await timerSnapshot(page, side);
  const currentURL = page.url();
  await page.close();

  return {
    status: response?.status() ?? null,
    url: currentURL,
    consoleErrors,
    failedRequests,
    badResponses,
    before,
    after
  };
}

async function timerSnapshot(page: Page, side: "legacy" | "migrated"): Promise<TimerSnapshot> {
  const rows = await page.evaluate((pageSide) => {
    const compact = (value: string | null | undefined): string => (value ?? "").replace(/\s+/g, " ").trim();
    const eventRows =
      pageSide === "legacy"
        ? Array.from(document.querySelectorAll<HTMLTableRowElement>("#content tr.flight, #content tr.return, #content tr.holding, #content tr:not([class])")).filter((row) =>
            compact(row.textContent).includes("Mission:")
          )
        : Array.from(document.querySelectorAll<HTMLTableRowElement>(".legacy-overview-main-table tr")).filter((row) =>
            row.querySelector(".legacy-overview-event-timer")
          );
    return eventRows.map((row) => {
      const timer = row.querySelector<HTMLElement>("div[id^='bxx'], .legacy-overview-event-timer");
      const bodyCell = row.querySelector<HTMLElement>("th[colspan='3']") ?? row.children.item(1);
      const timerText = compact(timer?.textContent);
      return {
        className: row.className,
        timerText,
        seconds: parseTimerSeconds(timerText),
        bodyText: compact(bodyCell?.textContent)
      };
    });

    function parseTimerSeconds(value: string): number | null {
      const colon = /^(\d+):(\d{2}):(\d{2})$/.exec(value);
      if (colon) {
        return Number(colon[1]) * 3600 + Number(colon[2]) * 60 + Number(colon[3]);
      }
      const parts = /^(?:(\d+)h\s*)?(?:(\d+)m\s*)?(?:(\d+)s)?$/.exec(value);
      if (parts && (parts[1] || parts[2] || parts[3])) {
        return Number(parts[1] ?? 0) * 3600 + Number(parts[2] ?? 0) * 60 + Number(parts[3] ?? 0);
      }
      return null;
    }
  }, side);
  return {
    capturedAt: new Date().toISOString(),
    rows
  };
}

function evaluateCountdown(legacy: PageCapture, migrated: PageCapture) {
  const legacyDeltas = deltas(legacy);
  const migratedDeltas = deltas(migrated);
  const rowCountPass =
    legacy.before.rows.length >= 3 &&
    legacy.before.rows.length === legacy.after.rows.length &&
    legacy.before.rows.length === migrated.before.rows.length &&
    migrated.before.rows.length === migrated.after.rows.length;
  const visibleContractPass =
    JSON.stringify(visibleRows(legacy.before.rows)) === JSON.stringify(visibleRows(migrated.before.rows)) &&
    JSON.stringify(visibleRows(legacy.after.rows)) === JSON.stringify(visibleRows(migrated.after.rows));
  const timerFormatPass = [...legacy.before.rows, ...legacy.after.rows, ...migrated.before.rows, ...migrated.after.rows].every((row) =>
    /^\d+:\d{2}:\d{2}$/.test(row.timerText)
  );
  const decreasePass = [...legacyDeltas, ...migratedDeltas].every((delta) => delta !== null && delta >= 1 && delta <= 5);
  const crossSideSkewPass = legacy.before.rows.every((legacyRow, index) => {
    const migratedRow = migrated.before.rows[index];
    if (legacyRow.seconds === null || migratedRow?.seconds === null || migratedRow?.seconds === undefined) {
      return false;
    }
    return Math.abs(legacyRow.seconds - migratedRow.seconds) <= maxCrossSideSkewSeconds;
  });
  return {
    statusPass: legacy.status === 200 && migrated.status === 200,
    noBrowserErrors:
      legacy.consoleErrors.length === 0 &&
      migrated.consoleErrors.length === 0 &&
      legacy.failedRequests.length === 0 &&
      migrated.failedRequests.length === 0 &&
      legacy.badResponses.length === 0 &&
      migrated.badResponses.length === 0,
    rowCountPass,
    visibleContractPass,
    timerFormatPass,
    decreasePass,
    crossSideSkewPass,
    legacyDeltas,
    migratedDeltas
  };
}

function visibleRows(rows: TimerRow[]) {
  return rows.map((row) => ({ className: row.className, bodyText: row.bodyText }));
}

function deltas(capture: PageCapture): Array<number | null> {
  return capture.before.rows.map((row, index) => {
    const after = capture.after.rows[index];
    if (row.seconds === null || after?.seconds === null || after?.seconds === undefined) {
      return null;
    }
    return row.seconds - after.seconds;
  });
}

async function waitForImages(page: Page): Promise<void> {
  await page.waitForFunction(async () => {
    const images = Array.from(document.images);
    await Promise.all(
      images.map(async (image) => {
        if (image.complete) {
          return;
        }
        await new Promise<void>((resolve) => {
          image.addEventListener("load", () => resolve(), { once: true });
          image.addEventListener("error", () => resolve(), { once: true });
        });
      })
    );
    return true;
  });
}

function renderMarkdown(report: {
  generatedAt: string;
  browserName: string;
  pass: boolean;
  checks: ReturnType<typeof evaluateCountdown>;
  legacy: PageCapture;
  migrated: PageCapture;
}): string {
  const lines: string[] = [];
  lines.push(`# Overview Fleet Countdown E2E (${report.browserName})`);
  lines.push("");
  lines.push(`Generated: ${report.generatedAt}`);
  lines.push(`Pass: ${report.pass ? "yes" : "no"}`);
  lines.push(`Legacy deltas: ${report.checks.legacyDeltas.join(", ")}`);
  lines.push(`Migrated deltas: ${report.checks.migratedDeltas.join(", ")}`);
  lines.push("");
  lines.push("## Legacy");
  report.legacy.before.rows.forEach((row, index) => {
    lines.push(`- ${row.timerText} -> ${report.legacy.after.rows[index]?.timerText ?? "(missing)"} ${row.bodyText}`);
  });
  lines.push("");
  lines.push("## Migrated");
  report.migrated.before.rows.forEach((row, index) => {
    lines.push(`- ${row.timerText} -> ${report.migrated.after.rows[index]?.timerText ?? "(missing)"} ${row.bodyText}`);
  });
  return `${lines.join("\n")}\n`;
}

function numberEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name];
  return raw === "firefox" || raw === "chromium" ? raw : fallback;
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
