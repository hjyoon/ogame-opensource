import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { copyFile, mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import {
  gameVisualScreens,
  globalGameVisualMaskSelectors,
  selectGameVisualScreens,
  selectGameVisualViewports,
  type BrowserName,
  type GameVisualScreenSpec,
  type LayoutBoxName,
  type SideName,
  type ViewportSpec
} from "./visual/game-screen-registry";
import {
  boxFor,
  boxesPresent,
  caseNotes,
  compareScreenshots,
  deterministicScreenshotCSS,
  expectedTextChecks,
  formatNumber,
  maxPairBoxDelta,
  normalizeDynamicPageParts,
  numberEnv,
  performVisualActions,
  textChecksEquivalent,
  trimTrailingSlash,
  waitForImages,
  waitForStablePaint,
  type Box,
  type DiffResult
} from "./visual/game-visual-utils";

type AuthFixture = {
  session: string;
  player_id?: number;
  login_user?: string;
  home_planet_id?: number;
  private_session?: string;
  cookies?: Record<string, string>;
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
  area: string;
  viewport: string;
  pass: boolean;
  parityPass: boolean;
  legacy: PageCapture;
  migrated: PageCapture;
  diff: DiffResult;
  diffPath: string;
  baselinePath?: string;
  boxMaxDelta: number;
  diffEnforced: boolean;
  layoutEnforced: boolean;
  notes: string[];
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = visualOutputDir(process.env.OGAME_GAME_VISUAL_OUTPUT_DIR, browserName);
const screenshotDir = join(outputDir, "screenshots");
const baselineDir = resolve(rootDir, process.env.OGAME_GAME_VISUAL_BASELINE_DIR ?? "testing/e2e/visual-baselines/authenticated-game", browserName);
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const loginUser = process.env.OGAME_GAME_VISUAL_USER ?? process.env.OGAME_AUTH_VISUAL_USER ?? "legor";
const loginPassword = process.env.OGAME_GAME_VISUAL_PASS ?? process.env.OGAME_AUTH_VISUAL_PASS ?? "admin";
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const fixedNowMs = numberEnv("OGAME_GAME_VISUAL_FIXED_NOW_MS", 1_765_584_000_000);
const diffEnforced = process.env.OGAME_GAME_VISUAL_ENFORCE_DIFF !== "0";
const layoutEnforced = process.env.OGAME_GAME_VISUAL_ENFORCE_LAYOUT !== "0";
const maxDiffRatio = numberEnv("OGAME_GAME_VISUAL_MAX_DIFF_RATIO", 0);
const maxBoxDelta = numberEnv("OGAME_GAME_VISUAL_MAX_BOX_DELTA", 0);
const colorDeltaThreshold = numberEnv("OGAME_GAME_VISUAL_COLOR_DELTA", 0);
const updateBaselines = process.env.OGAME_GAME_VISUAL_UPDATE_BASELINES === "1";
const screenFilter =
  process.env.OGAME_GAME_VISUAL_SCREEN ?? process.env.OGAME_GAME_VISUAL_SCREENS ?? process.env.OGAME_GAME_VISUAL_AREA ?? "";
const viewportFilter = process.env.OGAME_GAME_VISUAL_VIEWPORTS ?? process.env.OGAME_GAME_VISUAL_VIEWPORT ?? "";
const fixture = await loadAuthFixture(process.env.OGAME_GAME_VISUAL_FIXTURE_FILE);
const selectedScreens = selectGameVisualScreens(screenFilter);
const selectedViewports = selectGameVisualViewports(viewportFilter);
const viewportFilterActive = viewportFilter.trim().length > 0;

await mkdir(screenshotDir, { recursive: true });
if (updateBaselines) {
  await mkdir(baselineDir, { recursive: true });
}

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const results: CaseResult[] = [];
  for (const spec of selectedScreens) {
    const viewportsForSpec =
      !viewportFilterActive && spec.viewports ? selectGameVisualViewports(spec.viewports.join(",")) : selectedViewports;
    for (const viewport of viewportsForSpec) {
      if (viewportFilterActive && spec.viewports && !spec.viewports.includes(viewport.name)) {
        continue;
      }
      const legacyContext = await newContext(browser, viewport, legacyBaseURL, fixture);
      const legacySession = fixture?.session ?? (await loginLegacy(legacyContext));
      const legacy = await capturePage(legacyContext, spec, "legacy", legacyURL(spec, legacySession, fixture), viewport);
      await legacyContext.close();

      const migratedContext = await newContext(browser, viewport, migratedBaseURL, fixture);
      const migratedSession = fixture?.session ?? (await loginMigrated(migratedContext));
      const migrated = await capturePage(migratedContext, spec, "migrated", migratedURL(spec, migratedSession, fixture), viewport);
      await migratedContext.close();

      const diffPath = join(screenshotDir, `${spec.name}-${viewport.name}-diff.png`);
      const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath, colorDeltaThreshold);
      const boxMaxDelta = maxPairBoxDelta(legacy.boxes, migrated.boxes, spec.requiredBoxes);
      const notes = [...(spec.notes ?? []), ...caseNotes(legacy, migrated, diff, boxMaxDelta)];
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
        textChecksEquivalent(legacy.textChecks, migrated.textChecks);
      const pass = contractPass && (!diffEnforced || diff.diffRatio <= maxDiffRatio) && (!layoutEnforced || boxMaxDelta <= maxBoxDelta);
      const baselinePath = updateBaselines ? join(baselineDir, `${spec.name}-${viewport.name}.png`) : undefined;
      if (baselinePath) {
        await copyFile(legacy.screenshotPath, baselinePath);
      }
      results.push({
        page: spec.name,
        area: spec.area,
        viewport: viewport.name,
        pass,
        parityPass,
        legacy,
        migrated,
        diff,
        diffPath,
        baselinePath,
        boxMaxDelta,
        diffEnforced,
        layoutEnforced,
        notes
      });
    }
  }

  const report = {
    generatedAt: new Date().toISOString(),
    legacyBaseURL,
    migratedBaseURL,
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    loginUser: fixture?.login_user ?? loginUser,
    authMode: fixture ? "fixture" : "login",
    screenFilter: screenFilter || "default-enabled",
    viewportFilter: viewportFilter || "desktop",
    inventory: {
      totalScreens: gameVisualScreens.length,
      selectedScreens: selectedScreens.length,
      selectedViewports: selectedViewports.map((viewport) => viewport.name),
      defaultEnabledScreens: gameVisualScreens.filter((spec) => spec.defaultEnabled !== false).length
    },
    thresholds: {
      diffEnforced,
      maxDiffRatio,
      layoutEnforced,
      maxBoxDelta,
      colorDeltaThreshold
    },
    deterministic: {
      fixedNowMs,
      fixedNowISO: new Date(fixedNowMs).toISOString(),
      masks: globalGameVisualMaskSelectors
    },
    baselineDir: updateBaselines ? baselineDir : undefined,
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

async function loadAuthFixture(path: string | undefined): Promise<AuthFixture | undefined> {
  if (!path) {
    return undefined;
  }
  const resolved = resolve(rootDir, path);
  if (!existsSync(resolved)) {
    throw new Error(`authenticated game visual fixture file does not exist: ${resolved}`);
  }
  const parsed = JSON.parse(await readFile(resolved, "utf8")) as AuthFixture;
  if (!parsed.session) {
    throw new Error(`authenticated game visual fixture is missing session: ${resolved}`);
  }
  return parsed;
}

async function newContext(
  browser: Browser,
  viewport: ViewportSpec,
  baseURL: string,
  authFixture: AuthFixture | undefined
): Promise<BrowserContext> {
  const context = await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    deviceScaleFactor: 1,
    locale: "en-US",
    reducedMotion: "reduce",
    timezoneId: "UTC"
  });
  await context.addInitScript((now) => {
    const RealDate = Date;
    class FixedDate extends RealDate {
      constructor(...args: unknown[]) {
        if (args.length === 0) {
          super(now);
        } else if (args.length === 1) {
          super(args[0] as string | number | Date);
        } else {
          const dateArgs = args as [number, number, number?, number?, number?, number?, number?];
          super(dateArgs[0], dateArgs[1], dateArgs[2] ?? 1, dateArgs[3] ?? 0, dateArgs[4] ?? 0, dateArgs[5] ?? 0, dateArgs[6] ?? 0);
        }
      }
      static now() {
        return now;
      }
    }
    Object.setPrototypeOf(FixedDate, RealDate);
    Date = FixedDate as DateConstructor;
    Math.random = () => 0.42;
  }, fixedNowMs);
  await context.addCookies([{ name: "ogamelang", value: "en", url: baseURL }]);
  if (authFixture?.cookies) {
    await context.addCookies(
      Object.entries(authFixture.cookies).map(([name, value]) => ({
        name,
        value,
        url: baseURL
      }))
    );
  }
  return context;
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
  const universe = (await page.locator("select[name='universe'] option").nth(1).getAttribute("value")) ?? legacyBaseURL;
  await page.locator("select[name='universe']").selectOption(universe);
  await page.locator("input[name='login']").fill(loginUser);
  await page.locator("input[name='pass']").fill(loginPassword);
  await page.locator("input.legacy-public-login-button").click();
  await page.waitForFunction(() => window.location.pathname === "/game/overview" && window.location.search.includes("session="), undefined, {
    timeout: 10_000
  });
  await page.locator(".legacy-overview-table").first().waitFor({ timeout: 10_000 });
  const session = new URL(page.url()).searchParams.get("session") ?? "";
  await page.close();
  if (!session) {
    throw new Error("migrated login did not return a session");
  }
  return session;
}

async function capturePage(
  context: BrowserContext,
  spec: GameVisualScreenSpec,
  side: SideName,
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
  const readySelector = side === "legacy" ? spec.legacyReady : spec.migratedReady;
  try {
    await page.locator(readySelector).first().waitFor({ timeout: 10_000 });
  } catch (error) {
    consoleErrors.push(`${side} ready selector timeout: ${readySelector}; ${errorMessage(error)}`);
  }
  await page.addStyleTag({ content: deterministicScreenshotCSS });
  await page.mouse.move(0, 0);
  await performVisualActions(page, side, spec.actions);
  await page.waitForTimeout(100);
  await normalizeDynamicPageParts(page, side, spec, mergedMaskSelectors(spec));
  await waitForImages(page);
  await waitForStablePaint(page);

  const boxes = {
    header: await boxFor(page, side === "legacy" ? "#header_top" : ".legacy-header-top"),
    menu: await boxFor(page, side === "legacy" ? "#leftmenu" : ".legacy-leftmenu"),
    content: await boxFor(page, side === "legacy" ? "#content" : ".legacy-content")
  };
  const textChecks = await expectedTextChecks(page, spec.expectedTexts);
  const screenshotPath = join(screenshotDir, `${spec.name}-${viewport.name}-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false, animations: "disabled" });
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

function mergedMaskSelectors(spec: GameVisualScreenSpec): string[] {
  return [...globalGameVisualMaskSelectors, ...(spec.maskSelectors ?? [])];
}

function legacyURL(spec: GameVisualScreenSpec, session: string, authFixture: AuthFixture | undefined): string {
  const query = new URLSearchParams({ page: spec.legacyPage, session });
  const homePlanetId = authFixture?.home_planet_id;
  if (homePlanetId && !spec.legacyQuery?.cp) {
    query.set("cp", String(homePlanetId));
  }
  for (const [key, value] of Object.entries(spec.legacyQuery ?? {})) {
    query.set(key, value);
  }
  return `${legacyBaseURL}/game/index.php?${query.toString()}`;
}

function migratedURL(spec: GameVisualScreenSpec, session: string, authFixture: AuthFixture | undefined): string {
  const query = new URLSearchParams({ session });
  const homePlanetId = authFixture?.home_planet_id;
  if (homePlanetId && !spec.migratedQuery?.cp) {
    query.set("cp", String(homePlanetId));
  }
  for (const [key, value] of Object.entries(spec.migratedQuery ?? {})) {
    query.set(key, value);
  }
  return `${migratedBaseURL}${spec.migratedPath}?${query.toString()}`;
}

function renderMarkdown(report: {
  generatedAt: string;
  legacyBaseURL: string;
  migratedBaseURL: string;
  browserName: string;
  browserExecutable: string;
  loginUser: string;
  authMode: string;
  screenFilter: string;
  viewportFilter: string;
  inventory: {
    totalScreens: number;
    selectedScreens: number;
    selectedViewports: string[];
    defaultEnabledScreens: number;
  };
  thresholds: {
    diffEnforced: boolean;
    maxDiffRatio: number;
    layoutEnforced: boolean;
    maxBoxDelta: number;
    colorDeltaThreshold: number;
  };
  deterministic: {
    fixedNowMs: number;
    fixedNowISO: string;
    masks: string[];
  };
  baselineDir?: string;
  allPass: boolean;
  allParityPass: boolean;
  results: CaseResult[];
}): string {
  const lines = [
    "# Authenticated Game Visual Regression Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Legacy: ${report.legacyBaseURL}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- User: ${report.loginUser}`,
    `- Auth Mode: ${report.authMode}`,
    `- Screen Filter: ${report.screenFilter}`,
    `- Viewports: ${report.inventory.selectedViewports.join(", ")}`,
    `- Inventory: ${report.inventory.selectedScreens}/${report.inventory.totalScreens} selected, ${report.inventory.defaultEnabledScreens} default-enabled`,
    `- Fixed Clock: ${report.deterministic.fixedNowISO} (${report.deterministic.fixedNowMs})`,
    `- Diff Enforced: ${report.thresholds.diffEnforced}`,
    `- Max Diff Ratio: ${formatNumber(report.thresholds.maxDiffRatio)}`,
    `- Layout Enforced: ${report.thresholds.layoutEnforced}`,
    `- Max Box Delta: ${formatNumber(report.thresholds.maxBoxDelta)}`,
    `- Color Delta Threshold: ${formatNumber(report.thresholds.colorDeltaThreshold)}`,
    `- Baseline Dir: ${report.baselineDir ?? "not updated"}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    `- Visual Parity: ${report.allParityPass ? "PASS" : "FAIL"}`,
    "",
    "| Page | Area | Viewport | Contract | Parity | Diff Ratio | Box Max Delta | Diff Image | Notes |",
    "| --- | --- | --- | --- | --- | ---: | ---: | --- | --- |"
  ];
  for (const result of report.results) {
    lines.push(
      `| ${result.page} | ${result.area} | ${result.viewport} | ${result.pass ? "PASS" : "FAIL"} | ${result.parityPass ? "PASS" : "FAIL"} | ${formatNumber(
        result.diff.diffRatio
      )} | ${formatNumber(result.boxMaxDelta)} | ${result.diffPath} | ${result.notes.join("<br>") || "-"} |`
    );
  }
  lines.push("");
  return lines.join("\n");
}

function visualOutputDir(value: string | undefined, browser: string): string {
  if (!value) {
    return resolve(rootDir, ".tmp/playwright-authenticated-game-visual", browser);
  }
  return resolve(rootDir, value);
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name];
  return raw === "chromium" || raw === "firefox" ? raw : fallback;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message.split("\n")[0] : String(error);
}
