import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { Buffer } from "node:buffer";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";
type Side = "legacy" | "migrated";

type Coordinates = {
  g: number;
  s: number;
  p: number;
};

type Fixture = {
  user: string;
  player_id: number;
  planet_id: number;
  colony_id: number;
  enemy_planet_id: number;
  enemy_moon_id: number;
  debris_id: number;
  phantom_id: number;
  farspace_id: number;
  union_id: number;
  coordinates: {
    home: Coordinates;
    enemy: Coordinates;
    colony: Coordinates;
    moon: Coordinates;
    debris: Coordinates;
    farspace: Coordinates;
    empty: Coordinates;
  };
  session: string;
  private_cookie_name: string;
  private_cookie_value: string;
};

type ViewportSpec = {
  name: string;
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

type TableContract = {
  rows: string[];
  visibleRows: string[];
  links: Array<{ text: string; href: string; title: string }>;
  submits: string[];
  inputs: Array<{ name: string; value: string; type: string }>;
  images: Array<{ src: string; alt: string; width: string; height: string }>;
};

type FleetCapture = {
  name: string;
  status: number | null;
  url: string;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  contract: TableContract;
  screenshotPath: string;
};

type ClickResult = {
  name: string;
  legacy: unknown;
  migrated: unknown;
  pass: boolean;
};

type PreviewCase = {
  name: string;
  ships: Record<string, string>;
  target: Coordinates;
  targetType: 1 | 2 | 3;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-fleet-all-cases", browserName);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const fixturePath = resolve(process.env.OGAME_FLEET_ALL_FIXTURE_FILE ?? join(rootDir, ".tmp/fleet-all-cases-fixture.json"));
const chromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : chromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const maxDiffRatio = numberEnv("OGAME_FLEET_ALL_MAX_DIFF_RATIO", 0);
const colorDeltaThreshold = numberEnv("OGAME_FLEET_ALL_COLOR_DELTA", 0);
const enforceDiff = process.env.OGAME_FLEET_ALL_ENFORCE_DIFF !== "0";

const viewport: ViewportSpec = { name: "desktop", width: 1024, height: 768 };
const fixture = JSON.parse(await readFile(fixturePath, "utf8")) as Fixture;

const previewCases: PreviewCase[] = [
  { name: "preview-enemy-planet", ships: { ship204: "1" }, target: fixture.coordinates.enemy, targetType: 1 },
  { name: "preview-own-colony", ships: { ship202: "1" }, target: fixture.coordinates.colony, targetType: 1 },
  { name: "preview-debris", ships: { ship209: "1" }, target: fixture.coordinates.debris, targetType: 2 },
  { name: "preview-enemy-moon-destroy", ships: { ship214: "1" }, target: fixture.coordinates.moon, targetType: 3 },
  { name: "preview-expedition", ships: { ship202: "1" }, target: fixture.coordinates.farspace, targetType: 1 },
  { name: "preview-probe-only", ships: { ship210: "1" }, target: fixture.coordinates.enemy, targetType: 1 },
  { name: "preview-colonize-empty", ships: { ship208: "1" }, target: fixture.coordinates.empty, targetType: 1 }
];

await mkdir(screenshotDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const legacyContext = await newContext(browser, viewport, legacyBaseURL, fixture);
  const migratedContext = await newContext(browser, viewport, migratedBaseURL, fixture);

  const captureSpecs: Array<{
    name: string;
    capture: (context: BrowserContext, side: Side, viewportSpec: ViewportSpec) => Promise<FleetCapture>;
  }> = [
    { name: "initial", capture: captureInitialFleet },
    { name: "union", capture: captureUnionFleet },
    { name: "target", capture: captureTargetFleet },
    ...previewCases.map((preview) => ({
      name: preview.name,
      capture: (context: BrowserContext, side: Side, viewportSpec: ViewportSpec) =>
        capturePreviewFleet(context, side, viewportSpec, preview)
    }))
  ];

  const captures: Array<{ name: string; legacy: FleetCapture; migrated: FleetCapture; contractPass: boolean; diff: DiffResult; diffPath: string }> = [];
  for (const spec of captureSpecs) {
    const legacy = await spec.capture(legacyContext, "legacy", viewport);
    const migrated = await spec.capture(migratedContext, "migrated", viewport);
    const diffPath = join(screenshotDir, `${spec.name}-${viewport.name}-diff.png`);
    const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath);
    captures.push({
      name: spec.name,
      legacy,
      migrated,
      contractPass: JSON.stringify(legacy.contract) === JSON.stringify(migrated.contract),
      diff,
      diffPath
    });
  }

  const legacyClicks = await clickContract(legacyContext, "legacy");
  const migratedClicks = await clickContract(migratedContext, "migrated");
  await legacyContext.close();
  await migratedContext.close();

  const clicks = mergeClickResults(legacyClicks, migratedClicks);
  const statusPass = captures.every((capture) => capture.legacy.status === 200 && capture.migrated.status === 200);
  const requestPass = captures.every(
    (capture) =>
      capture.legacy.consoleErrors.length === 0 &&
      capture.migrated.consoleErrors.length === 0 &&
      capture.legacy.failedRequests.length === 0 &&
      capture.migrated.failedRequests.length === 0 &&
      capture.legacy.badResponses.length === 0 &&
      capture.migrated.badResponses.length === 0
  );
  const contractPass = captures.every((capture) => capture.contractPass);
  const diffPass = captures.every((capture) => !enforceDiff || capture.diff.diffRatio <= maxDiffRatio);
  const clickPass = clicks.every((click) => click.pass);
  const pass = statusPass && requestPass && contractPass && diffPass && clickPass;

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
      colonyID: fixture.colony_id,
      enemyPlanetID: fixture.enemy_planet_id,
      enemyMoonID: fixture.enemy_moon_id,
      debrisID: fixture.debris_id,
      phantomID: fixture.phantom_id,
      farspaceID: fixture.farspace_id,
      unionID: fixture.union_id
    },
    thresholds: { enforceDiff, maxDiffRatio, colorDeltaThreshold },
    pass,
    statusPass,
    requestPass,
    contractPass,
    diffPass,
    clickPass,
    clicks,
    captures
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(
    JSON.stringify(
      {
        pass,
        contractPass,
        clickPass,
        captures: captures.map((capture) => ({
          name: capture.name,
          contractPass: capture.contractPass,
          diffRatio: capture.diff.diffRatio,
          changedPixels: capture.diff.changedPixels
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

async function captureInitialFleet(context: BrowserContext, side: Side, viewportSpec: ViewportSpec): Promise<FleetCapture> {
  const page = await monitoredPage(context);
  const response = await page.goto(fleetURL(side), { waitUntil: "networkidle", timeout: 20_000 });
  await fleetShipInput(page, side, "ship202").waitFor({ timeout: 10_000 });
  await normalizeAndPaint(page, side);
  return finishCapture(page, side, "initial", response?.status() ?? null, viewportSpec, fleetInitialContract);
}

async function captureUnionFleet(context: BrowserContext, side: Side, viewportSpec: ViewportSpec): Promise<FleetCapture> {
  const page = await monitoredPage(context);
  const response = await page.goto(fleetURL(side), { waitUntil: "networkidle", timeout: 20_000 });
  await fleetShipInput(page, side, "ship202").waitFor({ timeout: 10_000 });
  const button = page.locator(side === "legacy" ? "#content input[type='submit'][value='Union']" : ".legacy-fleet-table input[type='submit'][value='Union']").first();
  if (side === "legacy") {
    await Promise.all([page.waitForNavigation({ waitUntil: "networkidle", timeout: 10_000 }), button.click()]);
  } else {
    await button.click();
    await page.locator(".legacy-fleet-union-table").waitFor({ timeout: 10_000 });
  }
  await normalizeAndPaint(page, side);
  return finishCapture(page, side, "union", response?.status() ?? null, viewportSpec, fleetUnionContract);
}

async function captureTargetFleet(context: BrowserContext, side: Side, viewportSpec: ViewportSpec): Promise<FleetCapture> {
  const page = await monitoredPage(context);
  const response = await page.goto(fleetURL(side), { waitUntil: "networkidle", timeout: 20_000 });
  await openTargetStep(page, side, { ship202: "1" });
  await normalizeAndPaint(page, side);
  return finishCapture(page, side, "target", response?.status() ?? null, viewportSpec, fleetTargetContract);
}

async function capturePreviewFleet(
  context: BrowserContext,
  side: Side,
  viewportSpec: ViewportSpec,
  preview: PreviewCase
): Promise<FleetCapture> {
  const page = await monitoredPage(context);
  const response = await page.goto(fleetURL(side), { waitUntil: "networkidle", timeout: 20_000 });
  await openTargetStep(page, side, preview.ships);
  await fillTarget(page, side, preview.target, preview.targetType);
  await clickTargetNext(page, side);
  await dispatchTable(page, side).waitFor({ timeout: 10_000 });
  await normalizeAndPaint(page, side);
  return finishCapture(page, side, preview.name, response?.status() ?? null, viewportSpec, fleetDispatchContract);
}

async function openTargetStep(page: Page, side: Side, ships: Record<string, string>): Promise<void> {
  await fleetShipInput(page, side, "ship202").waitFor({ timeout: 10_000 });
  for (const [name, value] of Object.entries(ships)) {
    await fleetShipInput(page, side, name).fill(value);
  }
  const submit = page.locator(side === "legacy" ? "#content input[type='submit'][value='continue']" : ".legacy-fleet-select-table input[type='submit'][value='continue']").first();
  if (side === "legacy") {
    await Promise.all([page.waitForNavigation({ waitUntil: "networkidle", timeout: 10_000 }), submit.click()]);
  } else {
    await submit.click();
  }
  await targetTable(page, side).waitFor({ timeout: 10_000 });
}

async function fillTarget(page: Page, side: Side, target: Coordinates, targetType: 1 | 2 | 3): Promise<void> {
  await targetInput(page, side, "galaxy").fill(String(target.g));
  await targetInput(page, side, "system").fill(String(target.s));
  await targetInput(page, side, "planet").fill(String(target.p));
  await targetSelect(page, side, "planettype").selectOption(String(targetType));
}

async function clickTargetNext(page: Page, side: Side): Promise<void> {
  const next = page.locator(side === "legacy" ? "#content input[type='submit'][value='Next']" : ".legacy-fleet-target-table input[type='submit'][value='Next']").first();
  if (side === "legacy") {
    await Promise.all([page.waitForNavigation({ waitUntil: "networkidle", timeout: 10_000 }), next.click()]);
  } else {
    await next.click();
  }
}

async function clickContract(context: BrowserContext, side: Side): Promise<Array<{ name: string; value: unknown }>> {
  const results: Array<{ name: string; value: unknown }> = [];

  let page = await monitoredPage(context);
  await page.goto(fleetURL(side), { waitUntil: "networkidle", timeout: 20_000 });
  await fleetShipInput(page, side, "ship202").waitFor({ timeout: 10_000 });
  results.push({ name: "standard-fleets-href", value: await normalizedHref(page, side, side === "legacy" ? "a[href*='page=fleet_templates']" : "a[href*='/game/fleet-templates']") });
  await activateLink(page.locator(side === "legacy" ? "#content a" : ".legacy-fleet-select-table a").filter({ hasText: /Probe Sweep|Raid Pair/ }).first());
  results.push({ name: "template-first-values", value: await selectedShipValues(page, side, ["ship202", "ship204", "ship209", "ship210"]) });
  await activateLink(
    page
      .locator(side === "legacy" ? "#content a[href*=\"maxShip('ship202')\"]" : ".legacy-fleet-select-table tr[data-fleet-ship-row='202'] a")
      .filter({ hasText: "FLEET1_ALL" })
      .first()
  );
  results.push({ name: "max-small-cargo", value: await fleetShipInput(page, side, "ship202").inputValue() });
  await activateLink(page.locator(side === "legacy" ? "#content a[href^='javascript:maxShips']" : ".legacy-fleet-select-table a[href='#all-ships']").first());
  results.push({ name: "all-ships", value: await selectedShipValues(page, side, ["ship202", "ship203", "ship204", "ship208", "ship209", "ship210", "ship214"]) });
  await activateLink(page.locator(side === "legacy" ? "#content a[href^='javascript:noShips']" : ".legacy-fleet-select-table a[href='#clear-ships']").first());
  results.push({ name: "no-ships", value: await selectedShipValues(page, side, ["ship202", "ship203", "ship204", "ship208", "ship209", "ship210", "ship214"]) });
  await page.close();

  page = await monitoredPage(context);
  await page.goto(fleetURL(side), { waitUntil: "networkidle", timeout: 20_000 });
  await openTargetStep(page, side, { ship202: "1" });
  await activateLink(page.locator(side === "legacy" ? "#content a[href*='setUnion']" : ".legacy-fleet-target-table a[href='#set-union-target']").first());
  results.push({ name: "target-union-link", value: await targetInputValues(page, side) });
  await page.close();

  page = await monitoredPage(context);
  await page.goto(fleetURL(side), { waitUntil: "networkidle", timeout: 20_000 });
  await openTargetStep(page, side, { ship202: "1" });
  await fillTarget(page, side, fixture.coordinates.enemy, 1);
  await clickTargetNext(page, side);
  await dispatchTable(page, side).waitFor({ timeout: 10_000 });
  await activateLink(page.locator(side === "legacy" ? "#content a[href^='javascript:maxResource']" : ".legacy-fleet-dispatch-table a[href='#max-resource']").first());
  results.push({ name: "resource-first-max", value: await resourceInputValues(page, side) });
  await activateLink(page.locator(side === "legacy" ? "#content a[href^='javascript:maxResources']" : ".legacy-fleet-dispatch-table a[href='#max-resources']").first());
  results.push({ name: "resource-all-max", value: await resourceInputValues(page, side) });
  await page.close();

  return results;
}

async function activateLink(locator: ReturnType<Page["locator"]>): Promise<void> {
  await locator.scrollIntoViewIfNeeded();
  await locator.evaluate((element) => {
    if (element instanceof HTMLElement) {
      element.click();
    }
  });
}

async function finishCapture(
  page: Page,
  side: Side,
  name: string,
  status: number | null,
  viewportSpec: ViewportSpec,
  contractFn: (page: Page, side: Side) => Promise<TableContract>
): Promise<FleetCapture> {
  const contract = await contractFn(page, side);
  const screenshotPath = join(screenshotDir, `${name}-${viewportSpec.name}-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false });
  const currentURL = page.url();
  const monitor = pageMonitor(page);
  await page.close();
  return {
    name,
    status,
    url: currentURL,
    consoleErrors: monitor.consoleErrors,
    failedRequests: monitor.failedRequests,
    badResponses: monitor.badResponses,
    contract,
    screenshotPath
  };
}

async function monitoredPage(context: BrowserContext): Promise<Page> {
  const page = await context.newPage();
  const monitor = {
    consoleErrors: [] as string[],
    failedRequests: [] as string[],
    badResponses: [] as string[]
  };
  page.on("console", (message) => {
    if (message.type() === "error") {
      const text = message.text();
      if (!text.includes("showGalaxy")) {
        monitor.consoleErrors.push(text);
      }
    }
  });
  page.on("requestfailed", (request) => {
    monitor.failedRequests.push(`${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`.trim());
  });
  page.on("response", (response) => {
    const status = response.status();
    if (status >= 400 && !response.url().endsWith("/favicon.ico")) {
      monitor.badResponses.push(`${status} ${response.url()}`);
    }
  });
  Reflect.set(page, "__ogameMonitor", monitor);
  return page;
}

function pageMonitor(page: Page): { consoleErrors: string[]; failedRequests: string[]; badResponses: string[] } {
  return Reflect.get(page, "__ogameMonitor") as { consoleErrors: string[]; failedRequests: string[]; badResponses: string[] };
}

async function normalizeAndPaint(page: Page, side: Side): Promise<void> {
  await waitForImages(page);
  await normalizeDynamicPageParts(page, side);
  await waitForStablePaint(page);
}

async function normalizeDynamicPageParts(page: Page, side: Side): Promise<void> {
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
    if (document.querySelector(".legacy-fleet-union-view, input[name='union_name']")) {
      hide("#header_top, .legacy-header-top");
      for (const table of document.querySelectorAll<HTMLElement>("#content table[width='519'], .legacy-fleet-table")) {
        table.style.backgroundColor = "#344566";
      }
    }
    hide("#header_top img, .legacy-header-top img");
    hide("#content table[width='519'] img[src*='admiral_ikon'], .legacy-fleet-table img[src*='admiral_ikon']");
    const resourceValues = Array.from(document.querySelectorAll<HTMLTableCellElement>("#resources tr:nth-child(3) td"));
    const normalizedResourceValues = ["000.000", "000.000", "0.000", "0", "0/0"];
    resourceValues.forEach((cell, index) => {
      cell.textContent = normalizedResourceValues[index] ?? "0";
    });
    for (const timer of document.querySelectorAll<HTMLElement>("#content div[id^='bxx'], .legacy-fleet-target-table div[id^='bxx']")) {
      timer.textContent = "0:00:00";
      timer.setAttribute("title", "0");
      timer.setAttribute("star", "0");
    }
  }, side);
}

async function fleetInitialContract(page: Page, side: Side): Promise<TableContract> {
  return tableContract(page, side, side === "legacy" ? "#content table[width='519']" : ".legacy-fleet-table, .legacy-fleet-select-table");
}

async function fleetUnionContract(page: Page, side: Side): Promise<TableContract> {
  return tableContract(
    page,
    side,
    side === "legacy"
      ? "#content table[width='519']"
      : ".legacy-fleet-table, .legacy-fleet-union-table, .legacy-fleet-select-table"
  );
}

async function fleetTargetContract(page: Page, side: Side): Promise<TableContract> {
  return tableContract(page, side, side === "legacy" ? "#content table[width='519']" : ".legacy-fleet-target-table");
}

async function fleetDispatchContract(page: Page, side: Side): Promise<TableContract> {
  return tableContract(page, side, side === "legacy" ? "#content table[width='519']" : ".legacy-fleet-dispatch-table");
}

async function tableContract(page: Page, side: Side, selector: string): Promise<TableContract> {
  return await page.evaluate(
    ({ pageSide, tableSelector }) => {
      const compact = (value: string | null | undefined): string => (value ?? "").replace(/\s+/g, " ").trim();
      const normalizeHref = (link: HTMLAnchorElement): string => {
        const href = link.getAttribute("href") ?? "";
        const text = compact(link.textContent);
        if (text === "FLEET1_ALL") {
          return "action:max-ship";
        }
        if (text === "no ships") {
          return "action:no-ships";
        }
        if (text === "all ships") {
          return "action:all-ships";
        }
        if (text === "max") {
          return "action:max-resource";
        }
        if (text === "All resources") {
          return "action:max-resources";
        }
        if (href.startsWith("javascript:setShips") || href.startsWith("javascript:throw new Error")) {
          return `action:fleet-template:${text}`;
        }
        if (href.includes("setUnion") || href === "#set-union-target") {
          return `action:set-union-target:${text}`;
        }
        if (href.includes("setTarget") || href === "#set-target") {
          return `action:set-target:${text}`;
        }
        if (!href || href.startsWith("javascript:") || href === "#") {
          return href;
        }
        try {
          const url = new URL(href, window.location.href);
          const page = url.searchParams.get("page");
          const query = new URLSearchParams(url.search);
          query.delete("session");
          query.delete("cp");
          query.delete("no_header");
          if (page) {
            query.delete("page");
            const route =
              page === "flotten1"
                ? "/game/fleet"
                : page === "fleet_templates"
                  ? "/game/fleet-templates"
                  : page === "galaxy"
                    ? "/game/galaxy"
                    : `/game/${page}`;
            if (query.has("planet") && !query.has("position")) {
              query.set("position", query.get("planet") ?? "");
              query.delete("planet");
            }
            return `${route}?${sortQuery(query)}`.replace(/\?$/, "");
          }
          query.delete("session");
          query.delete("cp");
          return `${url.pathname}?${sortQuery(query)}`.replace(/\?$/, "");
        } catch {
          return href;
        }
      };
      const visibleText = (node: Node): string => {
        if (node instanceof HTMLTableRowElement) {
          return compact(Array.from(node.cells).map((cell) => visibleText(cell)).join(" "));
        }
        let value = "";
        const walk = (current: Node) => {
          if (current.nodeType === Node.TEXT_NODE) {
            value += current.textContent ?? "";
            return;
          }
          if (!(current instanceof Element)) {
            return;
          }
          if (current instanceof HTMLBRElement) {
            value += " ";
            return;
          }
          if (current instanceof HTMLInputElement) {
            if (current.type !== "hidden") {
              value += ` ${current.value} `;
            }
            return;
          }
          if (current instanceof HTMLSelectElement) {
            const selected = current.selectedOptions[0] ?? current.options[0];
            value += ` ${selected?.textContent ?? ""} `;
            return;
          }
          if (current instanceof HTMLTextAreaElement) {
            value += ` ${current.value} `;
            return;
          }
          for (const child of Array.from(current.childNodes)) {
            walk(child);
          }
        };
        walk(node);
        return compact(value);
      };
      const tables = Array.from(document.querySelectorAll<HTMLTableElement>(tableSelector));
      const rows = tables
        .flatMap((table) => Array.from(table.querySelectorAll<HTMLTableRowElement>("tr")))
        .filter((row) => !Array.from(row.cells).some((cell) => cell.querySelector("table")));
      const links = tables.flatMap((table) =>
        Array.from(table.querySelectorAll<HTMLAnchorElement>("a")).map((link) => ({
          text: compact(link.textContent),
          href: normalizeHref(link),
          title: compact(link.getAttribute("title") ?? "")
        }))
      );
      const submits = tables.flatMap((table) =>
        Array.from(table.querySelectorAll<HTMLInputElement>("input[type='submit']")).map((input) => input.value)
      );
      const inputs = tables.flatMap((table) =>
        Array.from(table.querySelectorAll<HTMLInputElement>("input"))
          .filter((input) => input.type !== "hidden")
          .map((input) => ({ name: input.name, value: input.value, type: input.type }))
      );
      const images = tables.flatMap((table) =>
        Array.from(table.querySelectorAll<HTMLImageElement>("img")).map((image) => {
          const rect = image.getBoundingClientRect();
          return {
            src: normalizeImageSource(image.getAttribute("src") || image.currentSrc),
            alt: compact(image.getAttribute("alt") ?? ""),
            width: image.getAttribute("width") ?? String(Math.round(rect.width)),
            height: image.getAttribute("height") ?? String(Math.round(rect.height))
          };
        })
      );
      return {
        rows: rows.map((row) => visibleText(row)),
        visibleRows: rows.map((row) => visibleText(row)),
        links,
        submits,
        inputs,
        images
      };

      function normalizeImageSource(src: string): string {
        if (!src) {
          return "";
        }
        try {
          const url = new URL(src, window.location.href);
          return url.pathname.split("/").filter(Boolean).pop() ?? url.pathname;
        } catch {
          return src.split("/").filter(Boolean).pop() ?? src;
        }
      }

      function sortQuery(query: URLSearchParams): string {
        const sorted = new URLSearchParams();
        Array.from(query.entries())
          .sort(([left], [right]) => left.localeCompare(right))
          .forEach(([key, value]) => sorted.append(key, value));
        return sorted.toString();
      }
    },
    { pageSide: side, tableSelector: selector }
  );
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

function fleetURL(side: Side): string {
  const query = new URLSearchParams({ session: fixture.session, cp: String(fixture.planet_id) });
  if (side === "legacy") {
    query.set("page", "flotten1");
    return `${legacyBaseURL}/game/index.php?${query.toString()}`;
  }
  return `${migratedBaseURL}/game/fleet?${query.toString()}`;
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

function fleetShipInput(page: Page, side: Side, name: string) {
  return page.locator(side === "legacy" ? `#content input[name='${name}']` : `.legacy-fleet-select-table input[name='${name}']`).first();
}

function targetTable(page: Page, side: Side) {
  return page.locator(side === "legacy" ? "#content input[name='galaxy']" : ".legacy-fleet-target-table input[name='galaxy']").first();
}

function dispatchTable(page: Page, side: Side) {
  return page.locator(side === "legacy" ? "#content input[name='order']" : ".legacy-fleet-dispatch-table input[name='order']").first();
}

function targetInput(page: Page, side: Side, name: string) {
  return page.locator(side === "legacy" ? `#content input[name='${name}']` : `.legacy-fleet-target-table input[name='${name}']`).first();
}

function targetSelect(page: Page, side: Side, name: string) {
  return page.locator(side === "legacy" ? `#content select[name='${name}']` : `.legacy-fleet-target-table select[name='${name}']`).first();
}

async function normalizedHref(page: Page, _side: Side, selector: string): Promise<string> {
  return await page.locator(selector).first().evaluate((link) => {
    const raw = link instanceof HTMLAnchorElement ? link.getAttribute("href") ?? "" : "";
    if (!raw || raw.startsWith("javascript:")) {
      return raw;
    }
    const url = new URL(raw, window.location.href);
    const query = new URLSearchParams(url.search);
    const pageParam = query.get("page");
    query.delete("session");
    query.delete("cp");
    if (pageParam) {
      query.delete("page");
    }
    const route = pageParam === "fleet_templates" || url.pathname.endsWith("/game/fleet-templates") ? "/game/fleet-templates" : url.pathname;
    return `${route}?${query.toString()}`.replace(/\?$/, "");
  });
}

async function selectedShipValues(page: Page, side: Side, names: string[]): Promise<Record<string, string>> {
  const values: Record<string, string> = {};
  for (const name of names) {
    const input = fleetShipInput(page, side, name);
    if ((await input.count()) > 0) {
      values[name] = await input.inputValue();
    }
  }
  return values;
}

async function targetInputValues(page: Page, side: Side): Promise<Record<string, string>> {
  const values: Record<string, string> = {};
  for (const name of ["galaxy", "system", "planet"]) {
    values[name] = await targetInput(page, side, name).inputValue();
  }
  values.planettype = await targetSelect(page, side, "planettype").inputValue();
  const union = page.locator(side === "legacy" ? "#content input[name='union2']" : ".legacy-fleet-target-form input[name='union2']").first();
  values.union2 = (await union.count()) > 0 ? await union.inputValue() : "";
  return values;
}

async function resourceInputValues(page: Page, side: Side): Promise<Record<string, string>> {
  const values: Record<string, string> = {};
  for (const name of ["resource1", "resource2", "resource3"]) {
    const input = page.locator(side === "legacy" ? `#content input[name='${name}']` : `.legacy-fleet-dispatch-table input[name='${name}']`).first();
    values[name] = await input.inputValue();
  }
  return values;
}

function mergeClickResults(
  legacy: Array<{ name: string; value: unknown }>,
  migrated: Array<{ name: string; value: unknown }>
): ClickResult[] {
  const byName = new Map(migrated.map((entry) => [entry.name, entry.value]));
  return legacy.map((entry) => {
    const migratedValue = byName.get(entry.name);
    return {
      name: entry.name,
      legacy: entry.value,
      migrated: migratedValue,
      pass: JSON.stringify(entry.value) === JSON.stringify(migratedValue)
    };
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
            Math.round((0.2126 * leftPixels[i] + 0.7152 * leftPixels[i + 1] + 0.0722 * leftPixels[i + 2]) * 0.1);
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
  pass: boolean;
  contractPass: boolean;
  clickPass: boolean;
  captures: Array<{ name: string; contractPass: boolean; diff: DiffResult; diffPath: string }>;
  clicks: ClickResult[];
}): string {
  const lines: string[] = [];
  lines.push(`# Fleet All-Cases E2E (${report.browserName})`);
  lines.push("");
  lines.push(`Generated: ${report.generatedAt}`);
  lines.push(`Pass: ${report.pass ? "yes" : "no"}`);
  lines.push(`Contract pass: ${report.contractPass ? "yes" : "no"}`);
  lines.push(`Click pass: ${report.clickPass ? "yes" : "no"}`);
  lines.push("");
  lines.push("## Captures");
  for (const capture of report.captures) {
    lines.push(
      `- ${capture.name}: contract=${capture.contractPass ? "pass" : "fail"}, diff=${formatNumber(capture.diff.diffRatio)} (${capture.diff.changedPixels}/${capture.diff.totalPixels})`
    );
    lines.push(`  diff: ${capture.diffPath}`);
  }
  lines.push("");
  lines.push("## Clicks");
  for (const click of report.clicks) {
    lines.push(`- ${click.name}: ${click.pass ? "pass" : "fail"}`);
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
