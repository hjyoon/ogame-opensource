import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { Buffer } from "node:buffer";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type Fixture = {
  user: string;
  password: string;
  player_id: number;
  planet_id: number;
  target_planet_id: number;
  own_fleet_id: number;
  enemy_fleet_id: number;
  session: string;
  private_cookie_name: string;
  private_cookie_value: string;
};

type ViewportSpec = {
  name: string;
  width: number;
  height: number;
};

type Box = {
  x: number;
  y: number;
  width: number;
  height: number;
};

type EventRowContract = {
  className: string;
  text: string;
  timerText: string;
  spans: Array<{ className: string; text: string }>;
  links: Array<{ text: string; title: string; href: string }>;
  box: Box | null;
};

type DiffResult = {
  width: number;
  height: number;
  totalPixels: number;
  changedPixels: number;
  diffRatio: number;
  averageDelta: number;
};

type PageCapture = {
  status: number | null;
  url: string;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  eventRows: EventRowContract[];
  screenshotPath: string;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-overview-fleet-visual", browserName);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const fixturePath = resolve(process.env.OGAME_OVERVIEW_FLEET_FIXTURE_FILE ?? join(rootDir, ".tmp/overview-fleet-fixture.json"));
const chromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : chromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const maxDiffRatio = numberEnv("OGAME_OVERVIEW_FLEET_MAX_DIFF_RATIO", 0);
const colorDeltaThreshold = numberEnv("OGAME_OVERVIEW_FLEET_COLOR_DELTA", 0);
const enforceDiff = process.env.OGAME_OVERVIEW_FLEET_ENFORCE_DIFF !== "0";

const viewport: ViewportSpec = { name: "desktop", width: 1024, height: 768 };
const fixture = JSON.parse(await readFile(fixturePath, "utf8")) as Fixture;

await mkdir(screenshotDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const legacyContext = await newContext(browser, viewport, legacyBaseURL, fixture);
  const legacy = await capturePage(
    legacyContext,
    "legacy",
    legacyOverviewURL(fixture),
    "#content tr.flight, #content tr.return, #content tr.holding",
    viewport
  );
  await legacyContext.close();

  const migratedContext = await newContext(browser, viewport, migratedBaseURL, fixture);
  const migrated = await capturePage(
    migratedContext,
    "migrated",
    migratedOverviewURL(fixture),
    ".legacy-overview-event-timer",
    viewport
  );
  await migratedContext.close();

  const diffPath = join(screenshotDir, `overview-fleet-${viewport.name}-diff.png`);
  const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath);
  const eventContractPass = JSON.stringify(visibleEventContract(legacy.eventRows)) === JSON.stringify(visibleEventContract(migrated.eventRows));
  const pass =
    legacy.status === 200 &&
    migrated.status === 200 &&
    legacy.consoleErrors.length === 0 &&
    migrated.consoleErrors.length === 0 &&
    legacy.failedRequests.length === 0 &&
    migrated.failedRequests.length === 0 &&
    legacy.badResponses.length === 0 &&
    migrated.badResponses.length === 0 &&
    legacy.eventRows.length >= 3 &&
    migrated.eventRows.length >= 3 &&
    eventContractPass &&
    (!enforceDiff || diff.diffRatio <= maxDiffRatio);

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
      targetPlanetID: fixture.target_planet_id,
      ownFleetID: fixture.own_fleet_id,
      enemyFleetID: fixture.enemy_fleet_id
    },
    thresholds: { enforceDiff, maxDiffRatio, colorDeltaThreshold },
    pass,
    eventContractPass,
    legacy,
    migrated,
    diff,
    diffPath
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

async function newContext(browserInstance: Browser, viewportSpec: ViewportSpec, baseURL: string, fixtureData: Fixture): Promise<BrowserContext> {
  const context = await browserInstance.newContext({
    viewport: { width: viewportSpec.width, height: viewportSpec.height },
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

async function capturePage(
  context: BrowserContext,
  side: "legacy" | "migrated",
  url: string,
  readySelector: string,
  viewportSpec: ViewportSpec
): Promise<PageCapture> {
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

  const response = await page.goto(url, { waitUntil: "networkidle", timeout: 15_000 });
  await page.locator(readySelector).first().waitFor({ timeout: 10_000 });
  await waitForImages(page);
  await page.waitForTimeout(250);
  await normalizeDynamicPageParts(page, side);
  await waitForStablePaint(page);

  const eventRows = await eventRowContract(page, side);
  const screenshotPath = join(screenshotDir, `overview-fleet-${viewportSpec.name}-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false });
  const currentURL = page.url();
  await page.close();

  return {
    status: response?.status() ?? null,
    url: currentURL,
    consoleErrors,
    failedRequests,
    badResponses,
    eventRows,
    screenshotPath
  };
}

async function normalizeDynamicPageParts(page: Page, side: "legacy" | "migrated"): Promise<void> {
  await page.evaluate((pageSide) => {
    const hide = (selector: string) => {
      for (const element of document.querySelectorAll(selector)) {
        if (element instanceof HTMLElement) {
          element.style.visibility = "hidden";
        }
      }
    };
    if (pageSide === "legacy") {
      hide("#overDiv");
    }
    hide("#header_top img[width='50'][height='50'], .legacy-header-top img[width='50'][height='50']");
    const resourceValues = Array.from(document.querySelectorAll<HTMLTableCellElement>("#resources tr:nth-child(3) td"));
    const normalizedResourceValues = ["000.000", "000.000", "0.000", "0", "0/0"];
    resourceValues.forEach((cell, index) => {
      cell.textContent = normalizedResourceValues[index] ?? "0";
    });
    for (const headerCell of document.querySelectorAll<HTMLTableCellElement>(".legacy-overview-main-table th, #content table th")) {
      if (headerCell.textContent?.trim() === "Server time") {
        const timeCell = headerCell.nextElementSibling;
        if (timeCell instanceof HTMLElement) {
          timeCell.textContent = "Fri Jun 19 00:00:00";
        }
      }
    }
    for (const timer of document.querySelectorAll<HTMLElement>("#content div[id^='bxx'], .legacy-overview-event-timer")) {
      timer.textContent = "0:00:00";
      timer.setAttribute("title", "0");
      timer.setAttribute("data-time", "0");
      timer.setAttribute("star", "0");
    }
  }, side);
}

async function eventRowContract(page: Page, side: "legacy" | "migrated"): Promise<EventRowContract[]> {
  return await page.evaluate((pageSide) => {
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
      const box = row.getBoundingClientRect();
      const timer = row.querySelector<HTMLElement>("div[id^='bxx'], .legacy-overview-event-timer");
      return {
        className: row.className,
        text: compact(row.textContent),
        timerText: compact(timer?.textContent),
        spans: Array.from(row.querySelectorAll("span")).map((span) => ({
          className: span.className,
          text: compact(span.textContent)
        })),
        links: Array.from(row.querySelectorAll("a")).map((link) => ({
          text: compact(link.textContent),
          title: link.getAttribute("title") ?? "",
          href: normalizeHref(link.getAttribute("href") ?? "")
        })),
        box: { x: box.x, y: box.y, width: box.width, height: box.height }
      };
    });

    function normalizeHref(href: string): string {
      if (!href || href === "#") {
        return href;
      }
      try {
        const url = new URL(href, window.location.href);
        url.searchParams.delete("session");
        return `${url.pathname}?${url.searchParams.toString()}`.replace(/\?$/, "");
      } catch {
        return href;
      }
    }
  }, side);
}

function visibleEventContract(rows: EventRowContract[]) {
  return rows.map((row) => ({
    className: row.className,
    timerText: row.timerText,
    spans: row.spans,
    linkTexts: row.links.map((link) => link.text),
    box: row.box
  }));
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

async function waitForStablePaint(page: Page): Promise<void> {
  await page.evaluate(async () => {
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
  });
}

async function compareScreenshots(browserInstance: Browser, legacyPath: string, migratedPath: string, diffPath: string): Promise<DiffResult> {
  const page = await browserInstance.newPage({ viewport: { width: 16, height: 16 } });
  const legacy = await Bun.file(legacyPath).arrayBuffer();
  const migrated = await Bun.file(migratedPath).arrayBuffer();
  const result = await page.evaluate(
    async ({ left, right, threshold }) => {
      const leftImage = await loadImage(left);
      const rightImage = await loadImage(right);
      const width = Math.min(leftImage.width, rightImage.width);
      const height = Math.min(leftImage.height, rightImage.height);
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext("2d", { willReadFrequently: true });
      if (!ctx) {
        throw new Error("2D canvas is unavailable");
      }
      ctx.drawImage(leftImage, 0, 0);
      const leftPixels = ctx.getImageData(0, 0, width, height).data;
      ctx.clearRect(0, 0, width, height);
      ctx.drawImage(rightImage, 0, 0);
      const rightPixels = ctx.getImageData(0, 0, width, height).data;
      const diffImage = ctx.createImageData(width, height);
      let changedPixels = 0;
      let totalDelta = 0;
      for (let i = 0; i < leftPixels.length; i += 4) {
        const delta =
          Math.abs(leftPixels[i] - rightPixels[i]) +
          Math.abs(leftPixels[i + 1] - rightPixels[i + 1]) +
          Math.abs(leftPixels[i + 2] - rightPixels[i + 2]) +
          Math.abs(leftPixels[i + 3] - rightPixels[i + 3]);
        totalDelta += delta / 4;
        if (delta / 4 > threshold) {
          changedPixels += 1;
          diffImage.data[i] = 255;
          diffImage.data[i + 1] = 0;
          diffImage.data[i + 2] = 0;
          diffImage.data[i + 3] = 255;
        } else {
          const faded =
            230 +
            Math.round(
              (0.2126 * leftPixels[i] + 0.7152 * leftPixels[i + 1] + 0.0722 * leftPixels[i + 2]) * 0.1
            );
          diffImage.data[i] = faded;
          diffImage.data[i + 1] = faded;
          diffImage.data[i + 2] = faded;
          diffImage.data[i + 3] = 255;
        }
      }
      const totalPixels = width * height;
      ctx.putImageData(diffImage, 0, 0);
      return {
        width,
        height,
        totalPixels,
        changedPixels,
        diffRatio: changedPixels / totalPixels,
        averageDelta: totalDelta / totalPixels,
        diffDataURL: canvas.toDataURL("image/png")
      };

      async function loadImage(dataUrl: string): Promise<HTMLImageElement> {
        const image = new Image();
        image.src = dataUrl;
        await image.decode();
        return image;
      }
    },
    {
      left: `data:image/png;base64,${Buffer.from(legacy).toString("base64")}`,
      right: `data:image/png;base64,${Buffer.from(migrated).toString("base64")}`,
      threshold: colorDeltaThreshold
    }
  );
  await page.close();
  const base64 = result.diffDataURL.replace(/^data:image\/png;base64,/, "");
  await Bun.write(diffPath, Uint8Array.from(atob(base64), (char) => char.charCodeAt(0)));
  return {
    width: result.width,
    height: result.height,
    totalPixels: result.totalPixels,
    changedPixels: result.changedPixels,
    diffRatio: result.diffRatio,
    averageDelta: result.averageDelta
  };
}

function renderMarkdown(report: {
  generatedAt: string;
  browserName: string;
  legacyBaseURL: string;
  migratedBaseURL: string;
  pass: boolean;
  eventContractPass: boolean;
  diff: DiffResult;
  diffPath: string;
  legacy: PageCapture;
  migrated: PageCapture;
  thresholds: { enforceDiff: boolean; maxDiffRatio: number; colorDeltaThreshold: number };
}): string {
  const lines: string[] = [];
  lines.push(`# Overview Fleet Visual E2E (${report.browserName})`);
  lines.push("");
  lines.push(`Generated: ${report.generatedAt}`);
  lines.push(`Legacy: ${report.legacyBaseURL}`);
  lines.push(`Migrated: ${report.migratedBaseURL}`);
  lines.push(`Pass: ${report.pass ? "yes" : "no"}`);
  lines.push(`Event contract pass: ${report.eventContractPass ? "yes" : "no"}`);
  lines.push(`Exact diff ratio: ${formatNumber(report.diff.diffRatio)} (${report.diff.changedPixels}/${report.diff.totalPixels})`);
  lines.push(`Diff path: ${report.diffPath}`);
  lines.push(`Threshold: ${report.thresholds.enforceDiff ? formatNumber(report.thresholds.maxDiffRatio) : "not enforced"}`);
  lines.push("");
  lines.push("## Event Rows");
  lines.push("");
  lines.push("### Legacy");
  for (const row of report.legacy.eventRows) {
    lines.push(`- ${row.className || "(none)"}: ${row.text}`);
  }
  lines.push("");
  lines.push("### Migrated");
  for (const row of report.migrated.eventRows) {
    lines.push(`- ${row.className || "(none)"}: ${row.text}`);
  }
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

function formatNumber(value: number): string {
  if (!Number.isFinite(value)) {
    return String(value);
  }
  return value.toFixed(8).replace(/0+$/, "").replace(/\.$/, "");
}
