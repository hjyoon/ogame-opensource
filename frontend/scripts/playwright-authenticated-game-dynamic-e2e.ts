import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { execFile } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { promisify } from "node:util";
import {
  gameDynamicBehaviorSpecs,
  selectGameDynamicBehaviorSpecs,
  type GameDynamicAction,
  type GameDynamicAssertion,
  type GameDynamicBehaviorSpec,
  type GameFixtureFeature,
  type SideName
} from "./visual/game-dynamic-behavior-registry";
import { deterministicScreenshotCSS, numberEnv, trimTrailingSlash } from "./visual/game-visual-utils";

type BrowserName = "chromium" | "firefox";

type AuthFixture = {
  session: string;
  home_planet_id?: number;
  login_user?: string;
  cookies?: Record<string, string>;
  admin?: AuthProfile;
  max_fleet?: AuthProfile;
  no_ships?: AuthProfile;
  low_fuel?: AuthProfile;
  no_cargo?: AuthProfile;
  queue_short?: AuthProfile;
  research_short?: AuthProfile;
  shipyard_short?: AuthProfile;
  features?: Partial<Record<"acs" | "alliance" | "commander" | "phalanx" | "report", boolean>>;
};

type AuthProfile = {
  session: string;
  home_planet_id?: number;
  login_user?: string;
  cookies?: Record<string, string>;
};

type SideResult = {
  status: number | null;
  url: string;
  skipped: boolean;
  skipReason?: string;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  actionErrors: string[];
  assertions: Record<string, string | number | boolean | null>;
};

type CaseResult = {
  name: string;
  pass: boolean;
  skipped: boolean;
  notes: string[];
  legacy: SideResult;
  migrated: SideResult;
  comparisons: string[];
};

const rootDir = resolve(import.meta.dir, "../..");
const execFileAsync = promisify(execFile);
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, process.env.OGAME_GAME_DYNAMIC_OUTPUT_DIR ?? `.tmp/playwright-authenticated-game-dynamic/${browserName}`);
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const fixtureFile = process.env.OGAME_GAME_VISUAL_FIXTURE_FILE;
let fixture = await loadAuthFixture(fixtureFile);
const selectedSpecs = selectGameDynamicBehaviorSpecs(process.env.OGAME_GAME_DYNAMIC_CASES ?? "");
const fixedNowMs = numberEnv("OGAME_GAME_DYNAMIC_FIXED_NOW_MS", 1_765_584_000_000);
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
  const results: CaseResult[] = [];
  for (const spec of selectedSpecs) {
    const missingFeature = missingFixtureFeature(spec);
    if (missingFeature) {
      const skipReason = `missing fixture feature: ${missingFeature}`;
      results.push({
        name: spec.name,
        pass: true,
        skipped: true,
        notes: spec.notes ?? [],
        legacy: skippedSideResult(skipReason),
        migrated: skippedSideResult(skipReason),
        comparisons: []
      });
      continue;
    }

    if (spec.isolateSides) {
      fixture = await refreshAuthFixture(spec);
    }
    const legacyContext = await newContext(browser, legacyBaseURL, spec);
    const legacy = await runSide(legacyContext, "legacy", spec, legacyURL(spec));
    await legacyContext.close();

    if (spec.isolateSides) {
      fixture = await refreshAuthFixture(spec);
    }
    const migratedContext = await newContext(browser, migratedBaseURL, spec);
    const migrated = await runSide(migratedContext, "migrated", spec, migratedURL(spec));
    await migratedContext.close();

    const comparisons = compareAssertions(spec, legacy, migrated);
    const bothSkipped = legacy.skipped && migrated.skipped;
    const oneSkipped = legacy.skipped !== migrated.skipped;
    const pass =
      bothSkipped ||
      (!oneSkipped &&
        legacy.status === 200 &&
        migrated.status === 200 &&
        legacy.consoleErrors.length === 0 &&
        migrated.consoleErrors.length === 0 &&
        legacy.failedRequests.length === 0 &&
        migrated.failedRequests.length === 0 &&
        legacy.badResponses.length === 0 &&
        migrated.badResponses.length === 0 &&
        legacy.actionErrors.length === 0 &&
        migrated.actionErrors.length === 0 &&
        comparisons.length === 0);
    results.push({
      name: spec.name,
      pass,
      skipped: bothSkipped,
      notes: spec.notes ?? [],
      legacy,
      migrated,
      comparisons
    });
  }

  const report = {
    generatedAt: new Date().toISOString(),
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    legacyBaseURL,
    migratedBaseURL,
    loginUser: fixture.login_user ?? "fixture",
    fixedNowMs,
    selectedCases: selectedSpecs.length,
    totalCases: gameDynamicBehaviorSpecs.length,
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

function skippedSideResult(skipReason: string): SideResult {
  return {
    status: null,
    url: "",
    skipped: true,
    skipReason,
    consoleErrors: [],
    failedRequests: [],
    badResponses: [],
    actionErrors: [],
    assertions: {}
  };
}

async function loadAuthFixture(path: string | undefined): Promise<AuthFixture> {
  if (!path) {
    throw new Error("OGAME_GAME_VISUAL_FIXTURE_FILE is required for authenticated dynamic behavior E2E");
  }
  const resolved = resolve(rootDir, path);
  if (!existsSync(resolved)) {
    throw new Error(`authenticated game fixture file does not exist: ${resolved}`);
  }
  const parsed = parseAuthFixture(await readFile(resolved, "utf8"), resolved);
  return parsed;
}

function parseAuthFixture(raw: string, label: string): AuthFixture {
  const parsed = JSON.parse(raw) as AuthFixture;
  if (!parsed.session) {
    throw new Error(`authenticated game fixture is missing session: ${label}`);
  }
  return parsed;
}

async function refreshAuthFixture(spec?: GameDynamicBehaviorSpec): Promise<AuthFixture> {
  if (process.env.OGAME_GAME_DYNAMIC_PREPARE_FIXTURE === "0") {
    return loadAuthFixture(fixtureFile);
  }
  const containerDir = process.env.OGAME_E2E_CONTAINER_DIR ?? "/tmp/ogame-e2e";
  const fixtureScript = join(rootDir, "testing/e2e/prepare-authenticated-game-visual-fixture.php");
  const containerScript = `${containerDir}/prepare-authenticated-game-visual-fixture.php`;
  await execFileAsync("docker", ["compose", "cp", fixtureScript, `server:${containerScript}`], { cwd: rootDir });
  const { stdout } = await execFileAsync(
    "docker",
    [
      "compose",
      "exec",
      "-T",
      "-e",
      `OGAME_GAME_VISUAL_COMMANDER_FIXTURE=${fixtureFeatureFlag(spec, "commander", "1")}`,
      "-e",
      `OGAME_GAME_VISUAL_ALLIANCE_FIXTURE=${fixtureFeatureFlag(spec, "alliance", "1")}`,
      "-e",
      `OGAME_GAME_VISUAL_REPORT_FIXTURE=${fixtureFeatureFlag(spec, "report", "1")}`,
      "-e",
      `OGAME_GAME_VISUAL_PHALANX_FIXTURE=${fixtureFeatureFlag(spec, "phalanx", "1")}`,
      "-e",
      `OGAME_GAME_VISUAL_ACS_FIXTURE=${fixtureFeatureFlag(spec, "acs", "1")}`,
      "-e",
      `OGAME_GAME_VISUAL_USER=${process.env.OGAME_GAME_VISUAL_USER ?? ""}`,
      "-e",
      `OGAME_GAME_VISUAL_PASS=${process.env.OGAME_GAME_VISUAL_PASS ?? ""}`,
      "-e",
      `OGAME_GAME_VISUAL_ADMIN=${process.env.OGAME_GAME_VISUAL_ADMIN ?? ""}`,
      "server",
      "php",
      containerScript
    ],
    { cwd: rootDir, maxBuffer: 1024 * 1024 }
  );
  if (fixtureFile) {
    await writeFile(resolve(rootDir, fixtureFile), stdout);
  }
  return parseAuthFixture(stdout, "refreshed authenticated game fixture");
}

function fixtureFeatureFlag(spec: GameDynamicBehaviorSpec | undefined, feature: GameFixtureFeature, defaultValue: "0" | "1"): string {
  const override = spec?.fixtureFeatures?.[feature];
  if (override !== undefined) {
    return override ? "1" : "0";
  }
  const envName = `OGAME_GAME_VISUAL_${feature.toUpperCase()}_FIXTURE`;
  return process.env[envName] ?? defaultValue;
}

async function newContext(browserInstance: Browser, baseURL: string, spec: GameDynamicBehaviorSpec): Promise<BrowserContext> {
  const context = await browserInstance.newContext({
    viewport: { width: 1024, height: 768 },
    deviceScaleFactor: 1,
    locale: "en-US",
    reducedMotion: "reduce",
    timezoneId: "UTC"
  });
  if (spec.fixedClock !== false) {
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
  } else {
    await context.addInitScript(() => {
      Math.random = () => 0.42;
    });
  }
  await context.addCookies([{ name: "ogamelang", value: "en", url: baseURL }]);
  const profile = fixtureProfile(spec);
  if (profile.cookies) {
    await context.addCookies(Object.entries(profile.cookies).map(([name, value]) => ({ name, value, url: baseURL })));
  }
  return context;
}

async function runSide(context: BrowserContext, side: SideName, spec: GameDynamicBehaviorSpec, url: string): Promise<SideResult> {
  const page = await context.newPage();
  const consoleErrors: string[] = [];
  const failedRequests: string[] = [];
  const badResponses: string[] = [];
  const actionErrors: string[] = [];
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

  const missingFeature = missingFixtureFeature(spec);
  if (missingFeature) {
    await page.close();
    return {
      status: null,
      url,
      skipped: true,
      skipReason: `missing fixture feature: ${missingFeature}`,
      consoleErrors,
      failedRequests,
      badResponses,
      actionErrors,
      assertions: {}
    };
  }

  const response = await page.goto(url, { waitUntil: "networkidle", timeout: 15_000 });
  const readySelector = side === "legacy" ? spec.legacyReady : spec.migratedReady;
  await page.locator(readySelector).first().waitFor({ timeout: 10_000 });
  await page.addStyleTag({ content: deterministicScreenshotCSS });

  const applicable = await isApplicable(page, side, spec);
  if (!applicable.ok) {
    const currentURL = page.url();
    await page.close();
    return {
      status: response?.status() ?? null,
      url: currentURL,
      skipped: true,
      skipReason: applicable.reason,
      consoleErrors,
      failedRequests,
      badResponses,
      actionErrors,
      assertions: {}
    };
  }

  for (const action of spec.actions) {
    try {
      await performAction(page, side, action);
    } catch (error) {
      actionErrors.push(`${action.type} ${actionSelector(action, side) ?? ""}: ${errorMessage(error)}`);
    }
  }
  await page.waitForTimeout(100);
  const assertions: Record<string, string | number | boolean | null> = {};
  for (const assertion of spec.assertions) {
    assertions[assertion.name] = await readAssertion(page, side, assertion);
  }
  const currentURL = page.url();
  await page.close();
  return {
    status: response?.status() ?? null,
    url: currentURL,
    skipped: false,
    consoleErrors,
    failedRequests,
    badResponses,
    actionErrors,
    assertions
  };
}

function missingFixtureFeature(spec: GameDynamicBehaviorSpec): string | null {
  for (const feature of spec.requiredFixtureFeatures ?? []) {
    if (fixture.features?.[feature] !== true) {
      return feature;
    }
  }
  return null;
}

async function isApplicable(page: Page, side: SideName, spec: GameDynamicBehaviorSpec): Promise<{ ok: boolean; reason?: string }> {
  const selector = sideSelector(spec, side, "ApplicabilitySelector") ?? spec.applicabilitySelector;
  if (!selector) {
    return { ok: true };
  }
  const count = await page.locator(selector).count();
  return count > 0 ? { ok: true } : { ok: false, reason: `missing applicability selector: ${selector}` };
}

async function performAction(page: Page, side: SideName, action: GameDynamicAction): Promise<void> {
  if (action.type === "wait") {
    await page.waitForTimeout(action.waitMs ?? 100);
    return;
  }
  const selector = actionSelector(action, side);
  if (!selector) {
    throw new Error("missing selector");
  }
  if (action.type === "press" && /^(body|html)$/i.test(selector.trim())) {
    await page.keyboard.press(resolveFixtureValue(action.value ?? "Tab"));
    const waitForSelector = actionWaitForSelector(action, side);
    if (waitForSelector) {
      await page.locator(waitForSelector).first().waitFor({ timeout: 10_000 });
    }
    if (action.waitMs && action.waitMs > 0) {
      await page.waitForTimeout(action.waitMs);
    }
    return;
  }
  const locator = page.locator(selector).first();
  await locator.waitFor({ timeout: 5_000 });
  if (action.type === "popup") {
    await page.evaluate(() => {
      (window as Window & { __ogameDynamicPopup?: unknown }).__ogameDynamicPopup = null;
    });
    const popupPromise = page.waitForEvent("popup", { timeout: 5_000 });
    if (actionDispatchClick(action, side)) {
      await locator.dispatchEvent("click");
    } else {
      await locator.click({ timeout: 5_000 });
    }
    const popup = await popupPromise;
    await popup.waitForLoadState("domcontentloaded", { timeout: 10_000 }).catch(() => undefined);
    const popupWaitForSelector = actionPopupWaitForSelector(action, side);
    if (popupWaitForSelector) {
      await popup.locator(popupWaitForSelector).first().waitFor({ timeout: 10_000 });
    }
    if (action.waitMs && action.waitMs > 0) {
      await popup.waitForTimeout(action.waitMs);
    }
    const popupData = await popup.evaluate(() => ({
      bodyText: document.body?.innerText ?? "",
      innerHeight: window.innerHeight,
      innerWidth: window.innerWidth,
      title: document.title,
      url: window.location.href
    }));
    await page.evaluate((data) => {
      (window as Window & { __ogameDynamicPopup?: unknown }).__ogameDynamicPopup = data;
    }, popupData);
    await popup.close().catch(() => undefined);
  } else if (action.type === "click") {
    await locator.click({ timeout: 5_000 });
  } else if (action.type === "fill") {
    await locator.fill(resolveFixtureValue(action.value ?? ""), { timeout: 5_000 });
  } else if (action.type === "type") {
    await locator.fill("", { timeout: 5_000 });
    await locator.pressSequentially(resolveFixtureValue(action.value ?? ""), { timeout: 5_000 });
  } else if (action.type === "select") {
    await locator.selectOption(resolveFixtureValue(action.value ?? ""), { timeout: 5_000 });
  } else if (action.type === "hover") {
    await locator.hover({ timeout: 5_000 });
  } else {
    await locator.press(resolveFixtureValue(action.value ?? "Tab"), { timeout: 5_000 });
  }
  const waitForSelector = actionWaitForSelector(action, side);
  if (waitForSelector) {
    await page.locator(waitForSelector).first().waitFor({ timeout: 10_000 });
  }
  if (action.waitMs && action.waitMs > 0) {
    await page.waitForTimeout(action.waitMs);
  }
}

async function readAssertion(page: Page, side: SideName, assertion: GameDynamicAssertion): Promise<string | number | boolean | null> {
  if (assertion.type === "evaluate") {
    return normalizeEvaluation(await page.evaluate(assertion.expression ?? "undefined"));
  }
  const selector = assertionSelector(assertion, side);
  if (!selector) {
    return null;
  }
  if (assertion.type === "count") {
    return await page.locator(selector).count();
  }
  const locator = page.locator(selector).first();
  if ((await locator.count()) === 0) {
    return null;
  }
  if (assertion.type === "value") {
    return await locator.inputValue({ timeout: 5_000 });
  }
  if (assertion.type === "visible") {
    return await locator.isVisible({ timeout: 5_000 });
  }
  if (assertion.type === "checked") {
    return await locator.isChecked({ timeout: 5_000 });
  }
  if (assertion.type === "html") {
    return compactHTML(await locator.evaluate((element) => element.innerHTML));
  }
  return compact(await locator.textContent({ timeout: 5_000 }));
}

function normalizeEvaluation(value: unknown): string | number | boolean | null {
  if (value === null || value === undefined) {
    return null;
  }
  if (typeof value === "string") {
    return compact(value);
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return value;
  }
  return JSON.stringify(value);
}

function compareAssertions(spec: GameDynamicBehaviorSpec, legacy: SideResult, migrated: SideResult): string[] {
  const errors: string[] = [];
  if (legacy.skipped || migrated.skipped) {
    if (legacy.skipped !== migrated.skipped) {
      errors.push(`skip mismatch: legacy=${legacy.skipReason ?? "no"} migrated=${migrated.skipReason ?? "no"}`);
    }
    return errors;
  }
  for (const assertion of spec.assertions) {
    const legacyValue = legacy.assertions[assertion.name];
    const migratedValue = migrated.assertions[assertion.name];
    for (const [side, value] of [
      ["legacy", legacyValue],
      ["migrated", migratedValue]
    ] as const) {
      if (value === null) {
        errors.push(`${assertion.name} missing on ${side}`);
      }
      if (assertion.expected !== undefined && String(value) !== assertion.expected) {
        errors.push(`${assertion.name} expected ${assertion.expected} on ${side}, got ${String(value)}`);
      }
      if (assertion.contains !== undefined && !String(value).includes(assertion.contains)) {
        errors.push(`${assertion.name} expected ${side} to contain ${assertion.contains}, got ${String(value)}`);
      }
    }
    if (assertion.compareSides && !valuesEquivalent(legacyValue, migratedValue, assertion.tolerance)) {
      const suffix = assertion.tolerance === undefined ? "" : ` tolerance=${assertion.tolerance}`;
      errors.push(`${assertion.name} differs: legacy=${String(legacyValue)} migrated=${String(migratedValue)}${suffix}`);
    }
  }
  return errors;
}

function legacyURL(spec: GameDynamicBehaviorSpec): string {
  const profile = fixtureProfile(spec);
  const query = new URLSearchParams({ page: spec.legacyPage, session: profile.session });
  if (profile.home_planet_id && !spec.legacyQuery?.cp) {
    query.set("cp", String(profile.home_planet_id));
  }
  for (const [key, value] of Object.entries(spec.legacyQuery ?? {})) {
    query.set(key, resolveFixtureValue(value));
  }
  return `${legacyBaseURL}/game/index.php?${query.toString()}`;
}

function migratedURL(spec: GameDynamicBehaviorSpec): string {
  const profile = fixtureProfile(spec);
  const query = new URLSearchParams({ session: profile.session });
  if (profile.home_planet_id && !spec.migratedQuery?.cp) {
    query.set("cp", String(profile.home_planet_id));
  }
  for (const [key, value] of Object.entries(spec.migratedQuery ?? {})) {
    query.set(key, resolveFixtureValue(value));
  }
  return `${migratedBaseURL}${spec.migratedPath}?${query.toString()}`;
}

function fixtureProfile(spec: GameDynamicBehaviorSpec): AuthProfile {
  if (spec.fixtureProfile) {
    const profile = fixture[spec.fixtureProfile];
    if (!profile?.session) {
      throw new Error(`authenticated game fixture is missing ${spec.fixtureProfile} profile`);
    }
    return profile;
  }
  return fixture;
}

function resolveFixtureValue(value: string): string {
  const prefix = "$fixture.";
  if (!value.startsWith(prefix)) {
    return value;
  }
  const path = value.slice(prefix.length).split(".").filter(Boolean);
  let current: unknown = fixture;
  for (const key of path) {
    if (typeof current !== "object" || current === null || !(key in current)) {
      throw new Error(`dynamic query value ${value} is missing from authenticated fixture`);
    }
    current = (current as Record<string, unknown>)[key];
  }
  if (current === null || current === undefined || typeof current === "object") {
    throw new Error(`dynamic query value ${value} resolved to a non-scalar fixture value`);
  }
  return String(current);
}

function actionSelector(action: GameDynamicAction, side: SideName): string | undefined {
  return side === "legacy" ? action.legacySelector ?? action.selector : action.migratedSelector ?? action.selector;
}

function actionWaitForSelector(action: GameDynamicAction, side: SideName): string | undefined {
  return side === "legacy"
    ? action.legacyWaitForSelector ?? action.waitForSelector
    : action.migratedWaitForSelector ?? action.waitForSelector;
}

function actionPopupWaitForSelector(action: GameDynamicAction, side: SideName): string | undefined {
  return side === "legacy"
    ? action.legacyPopupWaitForSelector ?? action.popupWaitForSelector
    : action.migratedPopupWaitForSelector ?? action.popupWaitForSelector;
}

function actionDispatchClick(action: GameDynamicAction, side: SideName): boolean {
  return side === "legacy"
    ? action.legacyDispatchClick ?? action.dispatchClick ?? false
    : action.migratedDispatchClick ?? action.dispatchClick ?? false;
}

function assertionSelector(assertion: GameDynamicAssertion, side: SideName): string | undefined {
  return side === "legacy" ? assertion.legacySelector ?? assertion.selector : assertion.migratedSelector ?? assertion.selector;
}

function sideSelector(spec: GameDynamicBehaviorSpec, side: SideName, suffix: "ApplicabilitySelector"): string | undefined {
  const key = `${side}${suffix}` as keyof GameDynamicBehaviorSpec;
  const value = spec[key];
  return typeof value === "string" ? value : undefined;
}

function renderMarkdown(report: {
  generatedAt: string;
  browserName: string;
  browserExecutable: string;
  legacyBaseURL: string;
  migratedBaseURL: string;
  loginUser: string;
  fixedNowMs: number;
  selectedCases: number;
  totalCases: number;
  allPass: boolean;
  results: CaseResult[];
}): string {
  const lines = [
    "# Authenticated Game Dynamic Behavior Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Legacy: ${report.legacyBaseURL}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- User: ${report.loginUser}`,
    `- Fixed Clock: ${new Date(report.fixedNowMs).toISOString()}`,
    `- Cases: ${report.selectedCases}/${report.totalCases}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    "",
    "| Case | Result | Legacy | Migrated | Notes |",
    "| --- | --- | --- | --- | --- |"
  ];
  for (const result of report.results) {
    const resultText = result.skipped ? "SKIP" : result.pass ? "PASS" : "FAIL";
    lines.push(
      `| ${result.name} | ${resultText} | ${sideSummary(result.legacy)} | ${sideSummary(result.migrated)} | ${[...result.notes, ...result.comparisons].join("<br>") || "-"} |`
    );
  }
  lines.push("");
  return lines.join("\n");
}

function sideSummary(result: SideResult): string {
  if (result.skipped) {
    return `SKIP: ${result.skipReason}`;
  }
  const values = Object.entries(result.assertions)
    .map(([name, value]) => `${name}=${String(value)}`)
    .join(", ");
  return values || "no assertions";
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name];
  return raw === "chromium" || raw === "firefox" ? raw : fallback;
}

function compact(value: string | null | undefined): string {
  return (value ?? "").replace(/\s+/g, " ").trim();
}

function compactHTML(value: string | null | undefined): string {
  return compact(value).replaceAll(/; ?/g, ";");
}

function valuesEquivalent(
  left: string | number | boolean | null | undefined,
  right: string | number | boolean | null | undefined,
  tolerance: number | undefined
): boolean {
  if (tolerance === undefined) {
    return left === right;
  }
  const leftNumber = legacyNumber(left);
  const rightNumber = legacyNumber(right);
  if (leftNumber === null || rightNumber === null) {
    return left === right;
  }
  return Math.abs(leftNumber - rightNumber) <= tolerance;
}

function legacyNumber(value: string | number | boolean | null | undefined): number | null {
  if (typeof value === "number") {
    return value;
  }
  if (typeof value !== "string") {
    return null;
  }
  const parsed = Number.parseInt(value.replaceAll(".", "").replace(/[^0-9-]/g, ""), 10);
  return Number.isFinite(parsed) ? parsed : null;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message.split("\n")[0] : String(error);
}
