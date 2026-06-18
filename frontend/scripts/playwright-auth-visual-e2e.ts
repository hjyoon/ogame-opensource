import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type ViewportSpec = {
  name: string;
  width: number;
  height: number;
};

type AuthPageSpec = {
  name: string;
  legacyPage: string;
  legacyQuery?: Record<string, string>;
  migratedPath: string;
  migratedQuery?: Record<string, string>;
  legacyReady: string;
  migratedReady: string;
  requiredBoxes?: LayoutBoxName[];
  expectedTexts: string[];
};

type LayoutBoxName = "header" | "menu" | "content";

type Box = {
  x: number;
  y: number;
  width: number;
  height: number;
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
  boxes: Record<string, Box | null>;
  textChecks: Record<string, boolean>;
  screenshotPath: string;
};

type CaseResult = {
  page: string;
  viewport: string;
  pass: boolean;
  parityPass: boolean;
  legacy: PageCapture;
  migrated: PageCapture;
  diff: DiffResult;
  boxMaxDelta: number;
  diffEnforced: boolean;
  layoutEnforced: boolean;
  notes: string[];
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-auth-visual", browserName);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const loginUser = process.env.OGAME_AUTH_VISUAL_USER ?? "legor";
const loginPassword = process.env.OGAME_AUTH_VISUAL_PASS ?? "admin";
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const diffEnforced = process.env.OGAME_AUTH_VISUAL_ENFORCE_DIFF === "1";
const layoutEnforced = process.env.OGAME_AUTH_VISUAL_ENFORCE_LAYOUT === "1";
const maxDiffRatio = numberEnv("OGAME_AUTH_VISUAL_MAX_DIFF_RATIO", 0);
const maxBoxDelta = numberEnv("OGAME_AUTH_VISUAL_MAX_BOX_DELTA", 0);
const colorDeltaThreshold = numberEnv("OGAME_AUTH_VISUAL_COLOR_DELTA", 0);

const viewports: ViewportSpec[] = [{ name: "desktop", width: 1024, height: 768 }];

const pageSpecs: AuthPageSpec[] = [
  {
    name: "game-overview",
    legacyPage: "overview",
    migratedPath: "/game/overview",
    legacyReady: "#content table",
    migratedReady: ".legacy-overview-table",
    expectedTexts: ["Arakis", "Legor", "Diameter", "Temperature", "[1:1:2]", "Points"]
  },
  {
    name: "game-buildings",
    legacyPage: "b_building",
    migratedPath: "/game/buildings",
    legacyReady: "#content img[src*='gebaeude/1.gif']",
    migratedReady: "[data-building-row='1']",
    expectedTexts: ["Metal Mine", "Crystal Mine", "Deuterium Synthesizer", "Cost:", "Duration:"]
  },
  {
    name: "game-resources",
    legacyPage: "resources",
    migratedPath: "/game/resources",
    legacyReady: "#content form#ressourcen",
    migratedReady: ".legacy-resources-table",
    expectedTexts: ["Production factor:", "Resource settings on planet", "Basic Income", "Storage capacity", "Total per hour:"]
  },
  {
    name: "game-research",
    legacyPage: "buildings",
    legacyQuery: { mode: "Forschung" },
    migratedPath: "/game/research",
    legacyReady: "#content table",
    migratedReady: ".legacy-research-table",
    expectedTexts: ["In order to do this, you need to build a research lab!"]
  },
  {
    name: "game-shipyard",
    legacyPage: "buildings",
    legacyQuery: { mode: "Flotte" },
    migratedPath: "/game/shipyard",
    legacyReady: "#content table",
    migratedReady: ".legacy-shipyard-table",
    expectedTexts: ["In order to do that, you need to build a shipyard!"]
  },
  {
    name: "game-fleet",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    expectedTexts: ["Fleets", "Expeditions", "Mission", "Ships (total)", "Please select your ships for this mission:", "Ship Type", "Available"]
  },
  {
    name: "game-galaxy",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Galaxy", "Solar system", "Coord.", "Planet", "Title (activity)", "Moon", "Debris", "Player", "Alliance", "Actions", "Legend"]
  },
  {
    name: "game-technology",
    legacyPage: "techtree",
    migratedPath: "/game/technology",
    legacyReady: "#content table",
    migratedReady: ".legacy-technology-table",
    expectedTexts: ["Buildings", "Requirements", "Metal Mine", "Research", "Ships", "Defense", "Lunar Buildings"]
  },
  {
    name: "game-technology-details",
    legacyPage: "techtreedetails",
    legacyQuery: { tid: "206" },
    migratedPath: "/game/technology",
    migratedQuery: { tid: "206" },
    legacyReady: "#content table",
    migratedReady: ".legacy-technology-details-table",
    expectedTexts: ["Building conditions for", "Cruiser", "Shipyard", "Impulse Drive", "Ion Technology"]
  },
  {
    name: "game-defense",
    legacyPage: "buildings",
    legacyQuery: { mode: "Verteidigung" },
    migratedPath: "/game/defense",
    legacyReady: "#content table",
    migratedReady: ".legacy-defense-table",
    expectedTexts: ["In order to do that, you need to build a shipyard!"]
  },
  {
    name: "game-statistics",
    legacyPage: "statistics",
    legacyQuery: { type: "ressources", start: "1" },
    migratedPath: "/game/statistics",
    migratedQuery: { type: "ressources", start: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-statistics-table",
    expectedTexts: ["Statistics", "What kind of", "Player", "Alliance", "Points"]
  },
  {
    name: "game-statistics-alliance",
    legacyPage: "statistics",
    legacyQuery: { who: "ally", type: "ressources", start: "1" },
    migratedPath: "/game/statistics",
    migratedQuery: { who: "ally", type: "ressources", start: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-statistics-table",
    expectedTexts: ["Statistics", "What kind of", "Alliance", "Num.", "Thousand points", "Per person"]
  },
  {
    name: "game-search",
    legacyPage: "suche",
    migratedPath: "/game/search",
    legacyReady: "#content table",
    migratedReady: ".legacy-search-head-table",
    expectedTexts: ["Search Universe", "Player Name", "Planet Name", "Alliance Tag", "Alliance Name", "search"]
  }
];

await mkdir(screenshotDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const results: CaseResult[] = [];
  for (const viewport of viewports) {
    const legacyContext = await newContext(browser, viewport);
    const legacySession = await loginLegacy(legacyContext);
    const legacyCaptures = new Map<string, PageCapture>();
    for (const spec of pageSpecs) {
      legacyCaptures.set(spec.name, await capturePage(legacyContext, spec, "legacy", legacyURL(spec, legacySession), viewport));
    }
    await legacyContext.close();

    const migratedContext = await newContext(browser, viewport);
    const migratedSession = await loginMigrated(migratedContext);
    for (const spec of pageSpecs) {
      const legacy = legacyCaptures.get(spec.name);
      if (!legacy) {
        throw new Error(`missing legacy capture for ${spec.name}`);
      }
      const migrated = await capturePage(migratedContext, spec, "migrated", migratedURL(spec, migratedSession), viewport);
      const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath);
      const boxMaxDelta = maxPairBoxDelta(legacy.boxes, migrated.boxes, spec.requiredBoxes);
      const notes = caseNotes(legacy, migrated, diff, boxMaxDelta);
      const parityPass = diff.diffRatio <= maxDiffRatio && boxMaxDelta <= maxBoxDelta;
      const contractPass =
        legacy.status === 200 &&
        migrated.status === 200 &&
        legacy.consoleErrors.length === 0 &&
        migrated.consoleErrors.length === 0 &&
        legacy.failedRequests.length === 0 &&
        migrated.failedRequests.length === 0 &&
        legacy.badResponses.length === 0 &&
        migrated.badResponses.length === 0 &&
        boxesPresent(legacy.boxes, spec.requiredBoxes) &&
        boxesPresent(migrated.boxes, spec.requiredBoxes) &&
        Object.values(legacy.textChecks).every(Boolean) &&
        Object.values(migrated.textChecks).every(Boolean);
      const pass = contractPass && (!diffEnforced || diff.diffRatio <= maxDiffRatio) && (!layoutEnforced || boxMaxDelta <= maxBoxDelta);
      results.push({
        page: spec.name,
        viewport: viewport.name,
        pass,
        parityPass,
        legacy,
        migrated,
        diff,
        boxMaxDelta,
        diffEnforced,
        layoutEnforced,
        notes
      });
    }
    await migratedContext.close();
  }

  const report = {
    generatedAt: new Date().toISOString(),
    legacyBaseURL,
    migratedBaseURL,
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    loginUser,
    thresholds: {
      diffEnforced,
      maxDiffRatio,
      layoutEnforced,
      maxBoxDelta,
      colorDeltaThreshold
    },
    allPass: results.every((result) => result.pass),
    allParityPass: results.every((result) => result.parityPass),
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

function legacyURL(spec: AuthPageSpec, session: string): string {
  const query = new URLSearchParams({ page: spec.legacyPage, session });
  for (const [key, value] of Object.entries(spec.legacyQuery ?? {})) {
    query.set(key, value);
  }
  return `${legacyBaseURL}/game/index.php?${query.toString()}`;
}

function migratedURL(spec: AuthPageSpec, session: string): string {
  const query = new URLSearchParams({ lgn: "1", session });
  for (const [key, value] of Object.entries(spec.migratedQuery ?? {})) {
    query.set(key, value);
  }
  return `${migratedBaseURL}${spec.migratedPath}?${query.toString()}`;
}

async function newContext(browser: Browser, viewport: ViewportSpec): Promise<BrowserContext> {
  return await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    deviceScaleFactor: 1,
    locale: "en-US"
  });
}

async function loginLegacy(context: BrowserContext): Promise<string> {
  const page = await context.newPage();
  await page.goto(
    `${legacyBaseURL}/game/reg/login2.php?login=${encodeURIComponent(loginUser)}&pass=${encodeURIComponent(loginPassword)}`,
    { waitUntil: "networkidle", timeout: 15_000 }
  );
  const session = new URL(page.url()).searchParams.get("session") ?? "";
  await page.close();
  if (!session) {
    throw new Error("legacy login did not return a session");
  }
  return session;
}

async function loginMigrated(context: BrowserContext): Promise<string> {
  const page = await context.newPage();
  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  const universe = (await page.locator("select[name='universe'] option").nth(1).getAttribute("value")) ?? "http://localhost:8888";
  await page.locator("select[name='universe']").selectOption(universe);
  await page.locator("input[name='login']").fill(loginUser);
  await page.locator("input[name='pass']").fill(loginPassword);
  await page.locator("input.legacy-public-login-button").click();
  await page.waitForFunction(() => window.location.pathname === "/game/overview" && window.location.search.includes("session="), undefined, {
    timeout: 10_000
  });
  const session = new URL(page.url()).searchParams.get("session") ?? "";
  await page.close();
  if (!session) {
    throw new Error("migrated login did not return a session");
  }
  return session;
}

async function capturePage(
  context: BrowserContext,
  spec: AuthPageSpec,
  side: "legacy" | "migrated",
  url: string,
  viewport: ViewportSpec
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
  await page.locator(side === "legacy" ? spec.legacyReady : spec.migratedReady).first().waitFor({ timeout: 10_000 });
  await page.waitForTimeout(250);
  await normalizeDynamicPageParts(page, side);

  const boxes = {
    header: await boxFor(page, side === "legacy" ? "#header_top" : ".legacy-header-top"),
    menu: await boxFor(page, side === "legacy" ? "#leftmenu" : ".legacy-leftmenu"),
    content: await boxFor(page, side === "legacy" ? "#content" : ".legacy-content")
  };
  const textChecks = await expectedTextChecks(page, spec.expectedTexts);
  const screenshotPath = join(screenshotDir, `${spec.name}-${viewport.name}-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false });
  const currentURL = page.url();
  await page.close();

  return {
    status: response?.status() ?? null,
    url: currentURL,
    consoleErrors,
    failedRequests,
    badResponses,
    boxes,
    textChecks,
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
  }, side);
}

async function expectedTextChecks(page: Page, expectedTexts: string[]): Promise<Record<string, boolean>> {
  return await page.evaluate((texts) => {
    const bodyText = document.body.textContent ?? "";
    return Object.fromEntries(texts.map((text) => [text, bodyText.includes(text)]));
  }, expectedTexts);
}

async function boxFor(page: Page, selector: string): Promise<Box | null> {
  const locator = page.locator(selector).first();
  if ((await locator.count()) === 0) {
    return null;
  }
  const box = await locator.boundingBox();
  return box ? { x: box.x, y: box.y, width: box.width, height: box.height } : null;
}

async function compareScreenshots(browser: Browser, legacyPath: string, migratedPath: string): Promise<DiffResult> {
  const page = await browser.newPage({ viewport: { width: 16, height: 16 } });
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
        }
      }
      const totalPixels = width * height;
      return {
        width,
        height,
        totalPixels,
        changedPixels,
        diffRatio: changedPixels / totalPixels,
        averageDelta: totalDelta / totalPixels
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
  return result;
}

function boxesPresent(boxes: Record<string, Box | null>, requiredBoxes: LayoutBoxName[] = ["header", "menu", "content"]): boolean {
  return requiredBoxes.every((boxName) => boxes[boxName] !== null);
}

function maxPairBoxDelta(
  left: Record<string, Box | null>,
  right: Record<string, Box | null>,
  requiredBoxes: LayoutBoxName[] = ["header", "menu", "content"]
): number {
  let maxDelta = 0;
  for (const key of requiredBoxes) {
    const leftBox = left[key];
    const rightBox = right[key];
    if (!leftBox || !rightBox) {
      return Number.POSITIVE_INFINITY;
    }
    maxDelta = Math.max(
      maxDelta,
      Math.abs(leftBox.x - rightBox.x),
      Math.abs(leftBox.y - rightBox.y),
      Math.abs(leftBox.width - rightBox.width),
      Math.abs(leftBox.height - rightBox.height)
    );
  }
  return maxDelta;
}

function caseNotes(legacy: PageCapture, migrated: PageCapture, diff: DiffResult, boxMaxDelta: number): string[] {
  return [
    ...legacy.consoleErrors.map((value) => `legacy console: ${value}`),
    ...migrated.consoleErrors.map((value) => `migrated console: ${value}`),
    ...legacy.failedRequests.map((value) => `legacy failed: ${value}`),
    ...migrated.failedRequests.map((value) => `migrated failed: ${value}`),
    ...legacy.badResponses.map((value) => `legacy response: ${value}`),
    ...migrated.badResponses.map((value) => `migrated response: ${value}`),
    ...missingTexts("legacy", legacy.textChecks),
    ...missingTexts("migrated", migrated.textChecks),
    `diff ratio ${formatNumber(diff.diffRatio)}`,
    `box max delta ${formatNumber(boxMaxDelta)}`
  ];
}

function missingTexts(side: string, checks: Record<string, boolean>): string[] {
  return Object.entries(checks)
    .filter(([, present]) => !present)
    .map(([text]) => `${side} missing text: ${text}`);
}

function renderMarkdown(report: {
  generatedAt: string;
  legacyBaseURL: string;
  migratedBaseURL: string;
  browserName: string;
  browserExecutable: string;
  loginUser: string;
  thresholds: {
    diffEnforced: boolean;
    maxDiffRatio: number;
    layoutEnforced: boolean;
    maxBoxDelta: number;
    colorDeltaThreshold: number;
  };
  allPass: boolean;
  allParityPass: boolean;
  results: CaseResult[];
}): string {
  const lines = [
    "# Playwright Authenticated Visual E2E Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Legacy: ${report.legacyBaseURL}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Login User: ${report.loginUser}`,
    `- Diff Enforced: ${report.thresholds.diffEnforced}`,
    `- Max Diff Ratio: ${formatNumber(report.thresholds.maxDiffRatio)}`,
    `- Layout Enforced: ${report.thresholds.layoutEnforced}`,
    `- Max Box Delta: ${formatNumber(report.thresholds.maxBoxDelta)}`,
    `- Color Delta Threshold: ${formatNumber(report.thresholds.colorDeltaThreshold)}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    `- Visual Parity: ${report.allParityPass ? "PASS" : "FAIL"}${report.thresholds.diffEnforced || report.thresholds.layoutEnforced ? "" : " (not enforced)"}`,
    "",
    "| Page | Viewport | Contract | Parity | Diff Ratio | Box Max Delta | Notes |",
    "| --- | --- | --- | --- | ---: | ---: | --- |"
  ];
  for (const result of report.results) {
    lines.push(
      `| ${result.page} | ${result.viewport} | ${result.pass ? "PASS" : "FAIL"} | ${result.parityPass ? "PASS" : "FAIL"} | ${formatNumber(
        result.diff.diffRatio
      )} | ${formatNumber(result.boxMaxDelta)} | ${result.notes.join("<br>") || "-"} |`
    );
  }
  lines.push("");
  return lines.join("\n");
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
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
  return raw === "chromium" || raw === "firefox" ? raw : fallback;
}

function formatNumber(value: number): string {
  if (value === 0 || Number.isInteger(value)) {
    return String(value);
  }
  return value.toPrecision(12).replace(/\.?0+$/, "");
}
