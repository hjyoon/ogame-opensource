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
  moon_id: number;
  colony_id: number;
  session: string;
  private_cookie_name: string;
  private_cookie_value: string;
};

type ViewportSpec = {
  name: string;
  width: number;
  height: number;
};

type SurfaceContract = {
  news: { href: string; start: string; end: string } | null;
  pageMessages: string[];
  pageErrors: string[];
  title: { text: string; href: string } | null;
  unread: { text: string; href: string } | null;
  moon: { text: string; href: string } | null;
  planets: Array<{ text: string; href: string; title: string }>;
  currentBuild: string;
  infoRows: string[];
  menuLinks: Array<{ text: string; href: string; target: string }>;
  actionLinks: Array<{ key: string; href: string; text: string }>;
};

type EventRowContract = {
  className: string;
  text: string;
  spans: Array<{ className: string; text: string }>;
  links: Array<{ text: string; title: string; href: string }>;
};

type ClickResult = {
  name: string;
  legacy: string;
  migrated: string;
  pass: boolean;
};

type ClickAction = {
  name: string;
  selector: string;
  readHref?: boolean;
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
  surface: SurfaceContract;
  events: EventRowContract[];
  screenshotPath: string;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-overview-all-cases", browserName);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const fixturePath = resolve(process.env.OGAME_OVERVIEW_ALL_FIXTURE_FILE ?? join(rootDir, ".tmp/overview-all-cases-fixture.json"));
const chromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : chromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const maxDiffRatio = numberEnv("OGAME_OVERVIEW_ALL_MAX_DIFF_RATIO", 0);
const colorDeltaThreshold = numberEnv("OGAME_OVERVIEW_ALL_COLOR_DELTA", 0);
const enforceDiff = process.env.OGAME_OVERVIEW_ALL_ENFORCE_DIFF !== "0";

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
  const legacy = await capturePage(legacyContext, "legacy", legacyOverviewURL(fixture), viewport);

  const migratedContext = await newContext(browser, viewport, migratedBaseURL, fixture);
  const migrated = await capturePage(migratedContext, "migrated", migratedOverviewURL(fixture), viewport);
  const legacyClicks = await clickContract(legacyContext, "legacy", fixture);
  const migratedClicks = await clickContract(migratedContext, "migrated", fixture);
  await legacyContext.close();
  await migratedContext.close();

  const diffPath = join(screenshotDir, `overview-all-${viewport.name}-diff.png`);
  const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath);
  const surfacePass = JSON.stringify(legacy.surface) === JSON.stringify(migrated.surface);
  const eventsPass = JSON.stringify(legacy.events) === JSON.stringify(migrated.events);
  const clicks = mergeClickResults(legacyClicks, migratedClicks);
  const clickPass = clicks.every((click) => click.pass);
  const pass =
    legacy.status === 200 &&
    migrated.status === 200 &&
    legacy.consoleErrors.length === 0 &&
    migrated.consoleErrors.length === 0 &&
    legacy.failedRequests.length === 0 &&
    migrated.failedRequests.length === 0 &&
    legacy.badResponses.length === 0 &&
    migrated.badResponses.length === 0 &&
    legacy.events.length >= 20 &&
    migrated.events.length >= 20 &&
    surfacePass &&
    eventsPass &&
    clickPass &&
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
      moonID: fixture.moon_id,
      colonyID: fixture.colony_id
    },
    thresholds: { enforceDiff, maxDiffRatio, colorDeltaThreshold },
    pass,
    surfacePass,
    eventsPass,
    clickPass,
    clicks,
    legacy,
    migrated,
    diff,
    diffPath
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(
    JSON.stringify(
      {
        pass,
        surfacePass,
        eventsPass,
        clickPass,
        diffRatio: diff.diffRatio,
        changedPixels: diff.changedPixels,
        eventRows: { legacy: legacy.events.length, migrated: migrated.events.length },
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
  viewportSpec: ViewportSpec
): Promise<PageCapture> {
  const page = await context.newPage();
  const consoleErrors: string[] = [];
  const failedRequests: string[] = [];
  const badResponses: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error" && !message.text().includes("showGalaxy")) {
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

  const response = await page.goto(url, { waitUntil: "networkidle", timeout: 20_000 });
  await page.locator("#content table").first().waitFor({ timeout: 10_000 });
  await waitForImages(page);
  await page.waitForTimeout(300);
  await normalizeDynamicPageParts(page, side);
  await waitForStablePaint(page);

  const surface = await surfaceContract(page, side);
  const events = await eventContract(page, side);
  const screenshotPath = join(screenshotDir, `overview-all-${viewportSpec.name}-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false });
  const currentURL = page.url();
  await page.close();

  return {
    status: response?.status() ?? null,
    url: currentURL,
    consoleErrors,
    failedRequests,
    badResponses,
    surface,
    events,
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
    if (pageSide === "legacy") {
      const eventRows = Array.from(
        document.querySelectorAll<HTMLTableRowElement>("#content tr.flight, #content tr.return, #content tr.holding")
      ).filter((row) => /Mission:|Rocket Attack|after order/.test((row.textContent ?? "").replace(/\s+/g, " ")));
      for (const row of eventRows) {
        const timerCell = row.querySelector<HTMLElement>("th, td");
        if (timerCell) {
          timerCell.textContent = "0:00:00";
        }
      }
    }
  }, side);
}

async function surfaceContract(page: Page, side: "legacy" | "migrated"): Promise<SurfaceContract> {
  return await page.evaluate((pageSide) => {
    const compact = (value: string | null | undefined): string => (value ?? "").replace(/\s+/g, " ").trim();
    const normalizeHref = (href: string): string => {
      if (!href || href.startsWith("javascript:") || href === "#") {
        return href;
      }
      try {
        const url = new URL(href, window.location.href);
        const page = url.searchParams.get("page");
        const query = new URLSearchParams(url.search);
        query.delete("session");
        query.delete("no_header");
        if (page) {
          query.delete("page");
          const route =
            page === "renameplanet"
              ? "/game/rename-planet"
              : page === "overview"
                ? "/game/overview"
                : page === "messages"
                  ? "/game/messages"
                  : page === "galaxy"
                    ? "/game/galaxy"
                    : page === "statistics"
                      ? "/game/statistics"
                      : `/game/${page}`;
          if (query.has("planet") && !query.has("position")) {
            query.set("position", query.get("planet") ?? "");
            query.delete("planet");
          }
          if (route !== "/game/messages") {
            query.delete("dsp");
          }
          return `${route}?${sortQuery(query)}`.replace(/\?$/, "");
        }
        query.delete("session");
        if (query.has("planet") && !query.has("position")) {
          query.set("position", query.get("planet") ?? "");
          query.delete("planet");
        }
        return `${url.pathname}?${sortQuery(query)}`.replace(/\?$/, "");
      } catch {
        return href;
      }
    };

    const mainTable =
      pageSide === "legacy"
        ? document.querySelector<HTMLTableElement>("#content table[width='519']")
        : document.querySelector<HTMLTableElement>(".legacy-overview-main-table");
    const titleLink = mainTable?.querySelector<HTMLAnchorElement>("tr:first-child td.c a") ?? null;
    const unreadLink =
      mainTable?.querySelector<HTMLAnchorElement>("a[href*='page=messages'], a[href*='/game/messages']") ?? null;
    const moonLink = mainTable?.querySelector<HTMLAnchorElement>("a img[alt='Moon']")?.closest("a") ?? null;
    const planetLinks = Array.from(mainTable?.querySelectorAll<HTMLAnchorElement>("table.s a, .legacy-planet-list a") ?? []).map((link) => ({
      text: compact(link.parentElement?.textContent),
      href: normalizeHref(link.getAttribute("href") ?? ""),
      title: link.getAttribute("title") ?? link.querySelector("img")?.getAttribute("title") ?? ""
    }));
    const actionLinks = Array.from(
      mainTable?.querySelectorAll<HTMLAnchorElement>(
        "a[href*='page=renameplanet'], a[href*='/game/rename-planet'], a[href*='page=messages'], a[href*='/game/messages'], a[href*='page=galaxy'], a[href*='/game/galaxy'], a[href*='page=statistics'], a[href*='/game/statistics']"
      ) ?? []
    ).map((link) => ({
      key: compact(link.textContent) || link.getAttribute("title") || "link",
      text: compact(link.textContent),
      href: normalizeHref(link.getAttribute("href") ?? "")
    }));
    const rows = Array.from(mainTable?.querySelectorAll<HTMLTableRowElement>(":scope > tbody > tr, :scope > tr") ?? []);
    const infoRows = rows
      .map((row) => compact(row.textContent))
      .filter((text) => /Diameter|Temperature|Position|Points/.test(text));
    const buildTexts = rows
      .map((row) => compact(row.textContent).replace(/\s*pp="[^"]*"; ps="[^"]*"; t_building\(\);/g, " "))
      .map((text) => text.replace(/(0:00:00)(?=\S)/g, "$1 "))
      .map((text) => compact(text))
      .filter((text) => text.includes("Metal Mine") || text.includes("Crystal Mine") || text === "free");
    const newsLink = document.querySelector<HTMLAnchorElement>("#combox");
    const menuLinks = Array.from(document.querySelectorAll<HTMLAnchorElement>("#menu a"))
      .map((link) => ({
        text: compact(link.textContent),
        href: normalizeHref(link.getAttribute("href") ?? ""),
        target: link.getAttribute("target") ?? ""
      }))
      .filter((link) => link.text === "Board" || link.text === "Discord");
    return {
      news: newsLink
        ? {
            href: normalizeHref(newsLink.getAttribute("href") ?? ""),
            start: compact(document.querySelector("#anfang")?.textContent),
            end: compact(document.querySelector("#ende")?.textContent)
          }
        : null,
      pageMessages: compact(document.querySelector("#messagebox")?.textContent)
        ? [compact(document.querySelector("#messagebox")?.textContent)]
        : [],
      pageErrors: compact(document.querySelector("#errorbox")?.textContent) ? [compact(document.querySelector("#errorbox")?.textContent)] : [],
      title: titleLink ? { text: compact(titleLink.textContent), href: normalizeHref(titleLink.getAttribute("href") ?? "") } : null,
      unread: unreadLink ? { text: compact(unreadLink.textContent), href: normalizeHref(unreadLink.getAttribute("href") ?? "") } : null,
      moon: moonLink ? { text: compact(moonLink.parentElement?.textContent), href: normalizeHref(moonLink.getAttribute("href") ?? "") } : null,
      planets: planetLinks,
      currentBuild: buildTexts[0] ?? "",
      infoRows,
      menuLinks,
      actionLinks
    };

    function sortQuery(query: URLSearchParams): string {
      const sorted = new URLSearchParams();
      Array.from(query.entries())
        .sort(([left], [right]) => left.localeCompare(right))
        .forEach(([key, value]) => sorted.append(key, value));
      return sorted.toString();
    }
  }, side);
}

async function eventContract(page: Page, side: "legacy" | "migrated"): Promise<EventRowContract[]> {
  return await page.evaluate((pageSide) => {
    const compact = (value: string | null | undefined): string => (value ?? "").replace(/\s+/g, " ").trim();
    const rows =
      pageSide === "legacy"
        ? Array.from(
            document.querySelectorAll<HTMLTableRowElement>(
              "#content tr.flight, #content tr.return, #content tr.holding, #content tr:not([class]), #content tr[class='']"
            )
          ).filter((row) =>
            /Mission:|Rocket Attack|after order/.test(compact(row.textContent))
          )
        : Array.from(document.querySelectorAll<HTMLTableRowElement>(".legacy-overview-main-table tr")).filter((row) =>
            row.querySelector(".legacy-overview-event-timer")
          );
    return rows.map((row) => ({
      className: row.className,
      text: compact(row.textContent),
      spans: Array.from(row.querySelectorAll("span")).map((span) => ({
        className: span.className,
        text: compact(span.textContent)
      })),
      links: Array.from(row.querySelectorAll("a")).map((link) => ({
        text: compact(link.textContent),
        title: link.getAttribute("title") ?? "",
        href: normalizeEventHref(link.getAttribute("href") ?? "")
      }))
    }));

    function normalizeEventHref(href: string): string {
      return href.replace(/session=[^&"']+/g, "session=");
    }
  }, side);
}

async function clickContract(context: BrowserContext, side: "legacy" | "migrated", fixtureData: Fixture): Promise<Record<string, string>> {
  const actions: ClickAction[] = [
    {
      name: "rename",
      selector:
        side === "legacy" ? "#content table[width='519'] tr:first-child td.c a" : ".legacy-overview-main-table tr:first-child td.c a"
    },
    {
      name: "messages",
      readHref: true,
      selector:
        side === "legacy"
          ? "#content table[width='519'] a[href*='page=messages'][href*='dsp=1']"
          : ".legacy-overview-main-table a[href*='/game/messages'][href*='dsp=1']"
    },
    { name: "moon", selector: "a:has(img[alt='Moon'])" },
    {
      name: "colony",
      selector:
        side === "legacy"
          ? `#content table[width='519'] table.s a[href*='cp=${fixtureData.colony_id}']`
          : `.legacy-planet-list a[href*='cp=${fixtureData.colony_id}']`
    },
    { name: "position", selector: "a[href*='page=galaxy'][href*='position=']:visible, .legacy-overview-position-link" },
    {
      name: "rank",
      selector:
        side === "legacy"
          ? "#content table[width='519'] a[href*='page=statistics'][href*='start=']"
          : ".legacy-overview-main-table a[href*='/game/statistics'][href*='start=']"
    },
    { name: "event-galaxy", selector: "#content tr.flight a[href^='javascript:showGalaxy'], .legacy-overview-main-table tr.flight a[href^='javascript:showGalaxy']" },
    { name: "board", readHref: true, selector: "#menu a:has-text('Board')" },
    { name: "discord", readHref: true, selector: "#menu a:has-text('Discord')" }
  ];
  const results: Record<string, string> = {};
  for (const action of actions) {
    const page = await context.newPage();
    await page.goto(side === "legacy" ? legacyOverviewURL(fixtureData) : migratedOverviewURL(fixtureData), {
      waitUntil: "networkidle",
      timeout: 20_000
    });
    await page.locator(action.selector).first().waitFor({ timeout: 10_000 });
    if (action.readHref) {
      const href = await page.locator(action.selector).first().getAttribute("href");
      results[action.name] = normalizeNavigatedURL(new URL(href ?? "", page.url()).toString());
    } else {
      await page.locator(action.selector).first().click({ timeout: 10_000 });
      await page.waitForLoadState("networkidle", { timeout: 10_000 }).catch(() => undefined);
      await page.waitForTimeout(250);
      results[action.name] = normalizeNavigatedURL(page.url());
    }
    await page.close();
  }
  return results;
}

function mergeClickResults(legacy: Record<string, string>, migrated: Record<string, string>): ClickResult[] {
  const names = Array.from(new Set([...Object.keys(legacy), ...Object.keys(migrated)])).sort();
  return names.map((name) => ({
    name,
    legacy: legacy[name] ?? "",
    migrated: migrated[name] ?? "",
    pass: legacy[name] === migrated[name]
  }));
}

function normalizeNavigatedURL(raw: string): string {
  const url = new URL(raw);
  const page = url.searchParams.get("page");
  const query = new URLSearchParams(url.search);
  query.delete("session");
  query.delete("no_header");
  if (query.has("planet") && !query.has("position")) {
    query.set("position", query.get("planet") ?? "");
    query.delete("planet");
  }
  if (page) {
    query.delete("page");
    const path =
      page === "renameplanet"
        ? "/game/rename-planet"
        : page === "overview"
          ? "/game/overview"
          : page === "messages"
            ? "/game/messages"
            : page === "galaxy"
              ? "/game/galaxy"
              : page === "statistics"
                ? "/game/statistics"
                : `/game/${page}`;
    if (path !== "/game/messages") {
      query.delete("dsp");
    }
    return `${path}?${sortQuery(query)}`.replace(/\?$/, "");
  }
  return `${url.pathname}?${sortQuery(query)}`.replace(/\?$/, "");
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
            230 + Math.round((0.2126 * leftPixels[i] + 0.7152 * leftPixels[i + 1] + 0.0722 * leftPixels[i + 2]) * 0.1);
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
  surfacePass: boolean;
  eventsPass: boolean;
  clickPass: boolean;
  clicks: ClickResult[];
  diff: DiffResult;
  diffPath: string;
  legacy: PageCapture;
  migrated: PageCapture;
}): string {
  const lines: string[] = [];
  lines.push(`# Overview All-Cases E2E (${report.browserName})`);
  lines.push("");
  lines.push(`Generated: ${report.generatedAt}`);
  lines.push(`Legacy: ${report.legacyBaseURL}`);
  lines.push(`Migrated: ${report.migratedBaseURL}`);
  lines.push(`Pass: ${report.pass ? "yes" : "no"}`);
  lines.push(`Surface contract: ${report.surfacePass ? "yes" : "no"}`);
  lines.push(`Event contract: ${report.eventsPass ? "yes" : "no"}`);
  lines.push(`Click contract: ${report.clickPass ? "yes" : "no"}`);
  lines.push(`Exact diff ratio: ${formatNumber(report.diff.diffRatio)} (${report.diff.changedPixels}/${report.diff.totalPixels})`);
  lines.push(`Diff path: ${report.diffPath}`);
  lines.push("");
  lines.push("## Clicks");
  for (const click of report.clicks) {
    lines.push(`- ${click.name}: ${click.pass ? "pass" : "fail"} legacy=${click.legacy} migrated=${click.migrated}`);
  }
  lines.push("");
  lines.push("## Event Rows");
  lines.push(`- Legacy: ${report.legacy.events.length}`);
  lines.push(`- Migrated: ${report.migrated.events.length}`);
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

function sortQuery(query: URLSearchParams): string {
  const sorted = new URLSearchParams();
  Array.from(query.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .forEach(([key, value]) => sorted.append(key, value));
  return sorted.toString();
}

function formatNumber(value: number): string {
  if (!Number.isFinite(value)) {
    return String(value);
  }
  return value.toFixed(8).replace(/0+$/, "").replace(/\.$/, "");
}
