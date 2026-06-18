import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { publicRouteManifest } from "../src/publicRouteManifest";
import type { PublicRouteKey } from "../src/routes";

type ViewportSpec = {
  name: string;
  width: number;
  height: number;
};

type BoxPair = {
  name: string;
  legacy: string;
  migrated: string;
};

type PageSpec = {
  name: string;
  legacyPath: string;
  migratedPath: string;
  boxes: BoxPair[];
  contracts?: DomContractSpec[];
};

type DomContractSpec = {
  name: string;
  selector: string;
  includeInnerHTML?: boolean;
};

type Box = {
  x: number;
  y: number;
  width: number;
  height: number;
};

type PageCapture = {
  status: number | null;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  boxes: Record<string, Box | null>;
  contracts: Record<string, DomContractSnapshot | null>;
  screenshotPath: string;
};

type DiffResult = {
  width: number;
  height: number;
  totalPixels: number;
  changedPixels: number;
  diffRatio: number;
  averageDelta: number;
};

type BoxCheck = {
  name: string;
  pass: boolean;
  maxDelta: number;
  legacy: Box | null;
  migrated: Box | null;
};

type DomContractImageSnapshot = {
  alt: string | null;
  title: string | null;
  style: Record<string, string>;
  rect: Box;
};

type DomContractLinkSnapshot = {
  text: string;
  href: string | null;
  target: string | null;
  style: Record<string, string>;
  rect: Box;
  images: DomContractImageSnapshot[];
};

type DomContractSnapshot = {
  tagName: string;
  innerHTML?: string;
  style: Record<string, string>;
  rect: Box;
  links: DomContractLinkSnapshot[];
};

type DomContractCheck = {
  name: string;
  pass: boolean;
  mismatches: string[];
  legacy: DomContractSnapshot | null;
  migrated: DomContractSnapshot | null;
};

type CaseResult = {
  page: string;
  viewport: string;
  pass: boolean;
  legacy: PageCapture;
  migrated: PageCapture;
  diff: DiffResult;
  maxDiffRatio: number;
  boxChecks: BoxCheck[];
  contractChecks: DomContractCheck[];
};

type PublicBehaviorObservation = {
  status: number | null;
  beforeHref: string;
  afterHref: string;
  beforePathname: string;
  afterPathname: string;
  afterHash: string;
  cookieValue: string;
  reloaded: boolean;
  probeAfter?: string;
};

type BehaviorResult = {
  name: string;
  pass: boolean;
  mismatches: string[];
  legacy: PublicBehaviorObservation;
  migrated: PublicBehaviorObservation;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-visual", browserName);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const defaultMaxDiffRatio = numberEnv("OGAME_VISUAL_MAX_DIFF_RATIO", 0);
const defaultMaxBoxDelta = numberEnv("OGAME_VISUAL_MAX_BOX_DELTA", 0);
const colorDeltaThreshold = numberEnv("OGAME_VISUAL_COLOR_DELTA", 0);
const domContractStyleProperties = [
  "display",
  "position",
  "left",
  "top",
  "width",
  "height",
  "fontFamily",
  "fontSize",
  "fontWeight",
  "fontStyle",
  "lineHeight",
  "color",
  "textDecorationLine",
  "textDecorationColor",
  "textAlign",
  "backgroundColor",
  "paddingTop",
  "paddingRight",
  "paddingBottom",
  "paddingLeft",
  "marginTop",
  "marginRight",
  "marginBottom",
  "marginLeft"
] as const;

const publicShellContracts: DomContractSpec[] = [
  { name: "products", selector: ".products" },
  { name: "mainmenu", selector: "#mainmenu" },
  { name: "login_text_2", selector: "#login_text_2" },
  { name: "copyright", selector: "#copyright" },
  { name: "downmenu", selector: "#downmenu", includeInnerHTML: true }
];

const viewports: ViewportSpec[] = [
  { name: "desktop", width: 1024, height: 768 },
  { name: "mobile", width: 390, height: 844 }
];

const publicMainBoxes: BoxPair[] = [
  { name: "main", legacy: "#main", migrated: ".legacy-public-main" },
  { name: "mainmenu", legacy: "#mainmenu", migrated: ".legacy-public-mainmenu" },
  { name: "login", legacy: "#login", migrated: ".legacy-public-login" }
];

const visualBoxesByRouteKey: Partial<Record<PublicRouteKey, BoxPair[]>> = {
  home: [...publicMainBoxes, { name: "panel", legacy: ".rightmenu", migrated: ".legacy-public-rightmenu" }],
  register: [...publicMainBoxes, { name: "panel", legacy: ".rightmenu_register", migrated: ".legacy-public-register-panel" }],
  about: [...publicMainBoxes, { name: "panel", legacy: ".rightmenu_big", migrated: ".legacy-public-about-panel" }],
  story: [...publicMainBoxes, { name: "panel", legacy: ".rightmenu_big", migrated: ".legacy-public-story-panel" }],
  screenshots: [...publicMainBoxes, { name: "panel", legacy: ".rightmenu_big", migrated: ".legacy-public-screenshots-panel" }],
  rules: [...publicMainBoxes, { name: "panel", legacy: ".rightmenu_big", migrated: ".legacy-public-rules-panel" }],
  universes: [...publicMainBoxes, { name: "panel", legacy: ".rightmenu_big", migrated: ".legacy-public-universes-panel" }],
  legal: [{ name: "document", legacy: "table", migrated: ".legacy-legal-document" }]
};

const visualContractsByRouteKey: Partial<Record<PublicRouteKey, DomContractSpec[]>> = {
  legal: []
};

type VisualPublicRouteEntry = (typeof publicRouteManifest)[number] & { legacyVisualPath: string };

const visualRouteEntries = publicRouteManifest.filter((route): route is VisualPublicRouteEntry => route.legacyVisualPath !== undefined);

const pageSpecs: PageSpec[] = visualRouteEntries.map((route) => {
  const boxes = visualBoxesByRouteKey[route.key];
  if (!boxes) {
    throw new Error(`Missing visual box mapping for public route ${route.key}`);
  }
  return {
    name: route.key,
    legacyPath: route.legacyVisualPath,
    migratedPath: route.path,
    boxes,
    contracts: visualContractsByRouteKey[route.key]
  };
});

await mkdir(screenshotDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const results: CaseResult[] = [];
  for (const viewport of viewports) {
    const context = await browser.newContext({
      viewport: { width: viewport.width, height: viewport.height },
      deviceScaleFactor: 1,
      locale: "en-US"
    });
    for (const spec of pageSpecs) {
      const legacy = await capturePage(context, spec, "legacy", legacyBaseURL + spec.legacyPath, viewport);
      const migrated = await capturePage(context, spec, "migrated", migratedBaseURL + spec.migratedPath, viewport);
      const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath);
      const maxBoxDelta = defaultMaxBoxDelta;
      const boxChecks = spec.boxes.map((pair) => compareBoxes(pair.name, legacy.boxes[pair.name], migrated.boxes[pair.name], maxBoxDelta));
      const maxDiffRatio = defaultMaxDiffRatio;
      const contractSpecs = spec.contracts ?? publicShellContracts;
      const contractChecks = contractSpecs.map((contract) => compareDomContracts(contract.name, legacy.contracts[contract.name], migrated.contracts[contract.name]));
      const pass =
        legacy.status === 200 &&
        migrated.status === 200 &&
        legacy.consoleErrors.length === 0 &&
        migrated.consoleErrors.length === 0 &&
        legacy.failedRequests.length === 0 &&
        migrated.failedRequests.length === 0 &&
        legacy.badResponses.length === 0 &&
        migrated.badResponses.length === 0 &&
        diff.diffRatio <= maxDiffRatio &&
        boxChecks.every((check) => check.pass) &&
        contractChecks.every((check) => check.pass);
      results.push({ page: spec.name, viewport: viewport.name, pass, legacy, migrated, diff, maxDiffRatio, boxChecks, contractChecks });
    }
    await context.close();
  }
  const behaviorResults = [
    await comparePublicBehavior(browser, "public language flag", "a:has(img[alt='Deutschland'])", {
      cookieValue: "de",
      reloaded: true,
      afterHrefEndsWithHash: false
    }),
    await comparePublicBehavior(browser, "public choose language link", ".products a:last-child", {
      cookieValue: "",
      reloaded: false,
      afterHrefEndsWithHash: true
    })
  ];

  const report = {
    generatedAt: new Date().toISOString(),
    legacyBaseURL,
    migratedBaseURL,
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    thresholds: {
      defaultMaxDiffRatio,
      defaultMaxBoxDelta,
      colorDeltaThreshold
    },
    allPass: results.every((result) => result.pass) && behaviorResults.every((result) => result.pass),
    behaviorResults,
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

async function capturePage(
  context: BrowserContext,
  spec: PageSpec,
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
  await page.waitForTimeout(250);
  await page.keyboard.press("Escape").catch(() => undefined);
  await page.waitForTimeout(50);
  const boxes: Record<string, Box | null> = {};
  for (const pair of spec.boxes) {
    const selector = side === "legacy" ? pair.legacy : pair.migrated;
    boxes[pair.name] = await boxFor(page, selector);
  }
  const contractEntries = await Promise.all((spec.contracts ?? publicShellContracts).map(async (contract) => [contract.name, await domContractFor(page, contract)] as const));
  const contracts = Object.fromEntries(contractEntries);
  const screenshotPath = join(screenshotDir, `${spec.name}-${viewport.name}-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false });
  await page.close();

  return {
    status: response?.status() ?? null,
    consoleErrors,
    failedRequests,
    badResponses,
    boxes,
    contracts,
    screenshotPath
  };
}

async function boxFor(page: Page, selector: string): Promise<Box | null> {
  const locator = page.locator(selector).first();
  if ((await locator.count()) === 0) {
    return null;
  }
  const box = await locator.boundingBox();
  if (box === null) {
    return null;
  }
  return {
    x: box.x,
    y: box.y,
    width: box.width,
    height: box.height
  };
}

async function domContractFor(page: Page, contract: DomContractSpec): Promise<DomContractSnapshot | null> {
  return await page.evaluate(({ properties, contract }) => {
    const root = document.querySelector(contract.selector);
    if (!(root instanceof HTMLElement)) {
      return null;
    }

    const styleOf = (element: Element): Record<string, string> => {
      const computed = getComputedStyle(element);
      return Object.fromEntries(
        properties.map((property) => [property, normalizeStyleValue(property, (computed as CSSStyleDeclaration & Record<string, string>)[property] ?? "")])
      );
    };
    const normalizeStyleValue = (property: string, value: string) => {
      if (property === "textAlign" && (value.startsWith("-webkit-") || value.startsWith("-moz-"))) {
        return value.replace(/^-(webkit|moz)-/, "");
      }
      return value;
    };
    const rectOf = (element: Element): Box => {
      const rect = element.getBoundingClientRect();
      return {
        x: rect.x,
        y: rect.y,
        width: rect.width,
        height: rect.height
      };
    };

    const snapshot: DomContractSnapshot = {
      tagName: root.tagName.toLowerCase(),
      style: styleOf(root),
      rect: rectOf(root),
      links: Array.from(root.querySelectorAll("a")).map((link) => ({
        text: link.textContent?.trim() ?? "",
        href: link.getAttribute("href"),
        target: link.getAttribute("target"),
        style: styleOf(link),
        rect: rectOf(link),
        images: Array.from(link.querySelectorAll("img")).map((image) => ({
          alt: image.getAttribute("alt"),
          title: image.getAttribute("title"),
          style: styleOf(image),
          rect: rectOf(image)
        }))
      }))
    };
    if (contract.includeInnerHTML) {
      snapshot.innerHTML = root.innerHTML;
    }
    return snapshot;
  }, { properties: [...domContractStyleProperties], contract });
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
  return {
    ...result
  };
}

function compareBoxes(name: string, legacy: Box | null, migrated: Box | null, maxBoxDelta: number): BoxCheck {
  if (legacy === null || migrated === null) {
    return { name, pass: false, maxDelta: Number.POSITIVE_INFINITY, legacy, migrated };
  }
  const maxDelta = Math.max(
    Math.abs(legacy.x - migrated.x),
    Math.abs(legacy.y - migrated.y),
    Math.abs(legacy.width - migrated.width),
    Math.abs(legacy.height - migrated.height)
  );
  return { name, pass: maxDelta <= maxBoxDelta, maxDelta, legacy, migrated };
}

function compareDomContracts(name: string, legacy: DomContractSnapshot | null, migrated: DomContractSnapshot | null): DomContractCheck {
  const mismatches: string[] = [];
  collectMismatches(name, legacy, migrated, mismatches);
  return {
    name,
    pass: mismatches.length === 0,
    mismatches,
    legacy,
    migrated
  };
}

function collectMismatches(path: string, legacy: unknown, migrated: unknown, mismatches: string[]): void {
  if (JSON.stringify(legacy) === JSON.stringify(migrated)) {
    return;
  }
  if (
    legacy !== null &&
    migrated !== null &&
    typeof legacy === "object" &&
    typeof migrated === "object" &&
    !Array.isArray(legacy) &&
    !Array.isArray(migrated)
  ) {
    const keys = new Set([...Object.keys(legacy), ...Object.keys(migrated)]);
    for (const key of keys) {
      collectMismatches(`${path}.${key}`, (legacy as Record<string, unknown>)[key], (migrated as Record<string, unknown>)[key], mismatches);
    }
    return;
  }
  if (Array.isArray(legacy) && Array.isArray(migrated)) {
    const length = Math.max(legacy.length, migrated.length);
    for (let index = 0; index < length; index += 1) {
      collectMismatches(`${path}[${index}]`, legacy[index], migrated[index], mismatches);
    }
    return;
  }
  mismatches.push(path);
}

async function comparePublicBehavior(
  browser: Browser,
  name: string,
  selector: string,
  expected: { cookieValue: string; reloaded: boolean; afterHrefEndsWithHash: boolean }
): Promise<BehaviorResult> {
  const legacy = await observePublicBehavior(browser, legacyBaseURL + "/home.php", selector);
  const migrated = await observePublicBehavior(browser, migratedBaseURL + "/home", selector);
  const mismatches: string[] = [];
  if (legacy.status !== 200) {
    mismatches.push(`legacy status ${legacy.status}`);
  }
  if (migrated.status !== 200) {
    mismatches.push(`migrated status ${migrated.status}`);
  }
  if (legacy.cookieValue !== expected.cookieValue || migrated.cookieValue !== expected.cookieValue) {
    mismatches.push(`cookie ${legacy.cookieValue}/${migrated.cookieValue}`);
  }
  if (legacy.reloaded !== expected.reloaded || migrated.reloaded !== expected.reloaded) {
    mismatches.push(`reloaded ${legacy.reloaded}/${migrated.reloaded}`);
  }
  if ((legacy.probeAfter === undefined) !== expected.reloaded || (migrated.probeAfter === undefined) !== expected.reloaded) {
    mismatches.push(`probeAfter ${legacy.probeAfter ?? "undefined"}/${migrated.probeAfter ?? "undefined"}`);
  }
  if (legacy.beforePathname !== legacy.afterPathname || migrated.beforePathname !== migrated.afterPathname) {
    mismatches.push(`pathname ${legacy.beforePathname}->${legacy.afterPathname}/${migrated.beforePathname}->${migrated.afterPathname}`);
  }
  if (legacy.afterHref.endsWith("#") !== expected.afterHrefEndsWithHash || migrated.afterHref.endsWith("#") !== expected.afterHrefEndsWithHash) {
    mismatches.push(`href hash ${legacy.afterHref}/${migrated.afterHref}`);
  }
  return {
    name,
    pass: mismatches.length === 0,
    mismatches,
    legacy,
    migrated
  };
}

async function observePublicBehavior(browser: Browser, url: string, selector: string): Promise<PublicBehaviorObservation> {
  const context = await browser.newContext({
    viewport: { width: 1024, height: 768 },
    deviceScaleFactor: 1,
    locale: "en-US"
  });
  try {
    await context.clearCookies();
    const page = await context.newPage();
    const response = await page.goto(url, { waitUntil: "networkidle", timeout: 15_000 });
    const before = await page.evaluate(() => ({
      href: window.location.href,
      pathname: window.location.pathname
    }));
    await page.evaluate(() => {
      (window as Window & { __ogameBehaviorProbe?: string }).__ogameBehaviorProbe = "before";
    });
    const navigation = page
      .waitForNavigation({ waitUntil: "domcontentloaded", timeout: 5_000 })
      .then(() => true)
      .catch(() => false);
    await page.locator(selector).click();
    await navigation;
    await page.waitForLoadState("networkidle").catch(() => undefined);
    await page.waitForTimeout(50);
    const after = await page.evaluate(
      ({ status, before }) => ({
        status,
        beforeHref: before.href,
        afterHref: window.location.href,
        beforePathname: before.pathname,
        afterPathname: window.location.pathname,
        afterHash: window.location.hash,
        cookieValue:
          document.cookie
            .split("; ")
            .find((cookie) => cookie.startsWith("ogamelang="))
            ?.split("=")[1] ?? "",
        reloaded: false,
        probeAfter: (window as Window & { __ogameBehaviorProbe?: string }).__ogameBehaviorProbe
      }),
      { status: response?.status() ?? null, before }
    );
    return {
      ...after,
      reloaded: after.probeAfter === undefined
    };
  } finally {
    await context.close();
  }
}

function renderMarkdown(report: {
  generatedAt: string;
  legacyBaseURL: string;
  migratedBaseURL: string;
  browserName?: string;
  browserExecutable?: string;
  thresholds: {
    defaultMaxDiffRatio: number;
    defaultMaxBoxDelta: number;
    colorDeltaThreshold: number;
  };
  allPass: boolean;
  behaviorResults: BehaviorResult[];
  results: CaseResult[];
}): string {
  const lines = [
    "# Playwright Visual E2E Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Legacy: ${report.legacyBaseURL}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName ?? "chromium"} (${report.browserExecutable ?? "playwright-default"})`,
    `- Max Diff Ratio: ${formatNumber(report.thresholds.defaultMaxDiffRatio)}`,
    `- Max Box Delta: ${formatNumber(report.thresholds.defaultMaxBoxDelta)}`,
    `- Color Delta Threshold: ${formatNumber(report.thresholds.colorDeltaThreshold)}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    "",
    "| Page | Viewport | Pass | Diff Ratio | Box Max Delta | Notes |",
    "| --- | --- | --- | ---: | ---: | --- |"
  ];
  for (const result of report.results) {
    const worstBox = result.boxChecks.reduce((current, next) => (next.maxDelta > current.maxDelta ? next : current), result.boxChecks[0]);
    const notes = [
      ...result.legacy.consoleErrors.map((value) => `legacy console: ${value}`),
      ...result.migrated.consoleErrors.map((value) => `migrated console: ${value}`),
      ...result.legacy.failedRequests.map((value) => `legacy failed: ${value}`),
      ...result.migrated.failedRequests.map((value) => `migrated failed: ${value}`),
      ...result.legacy.badResponses.map((value) => `legacy response: ${value}`),
      ...result.migrated.badResponses.map((value) => `migrated response: ${value}`),
      ...result.boxChecks.filter((check) => !check.pass).map((check) => `box ${check.name} delta ${formatNumber(check.maxDelta)}`),
      ...result.contractChecks.flatMap((check) => check.mismatches.map((value) => `contract mismatch: ${value}`))
    ];
    lines.push(
      `| ${result.page} | ${result.viewport} | ${result.pass ? "PASS" : "FAIL"} | ${formatNumber(result.diff.diffRatio)} | ${
        formatNumber(worstBox?.maxDelta ?? 0)
      } | ${notes.join("<br>") || "-"} |`
    );
  }
  lines.push("", "| Behavior | Pass | Notes |", "| --- | --- | --- |");
  for (const result of report.behaviorResults) {
    lines.push(`| ${result.name} | ${result.pass ? "PASS" : "FAIL"} | ${result.mismatches.join("<br>") || "-"} |`);
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

function browserEnv(name: string, fallback: "chromium" | "firefox"): "chromium" | "firefox" {
  const raw = process.env[name];
  if (raw === "chromium" || raw === "firefox") {
    return raw;
  }
  return fallback;
}

function formatNumber(value: number): string {
  if (value === 0 || Number.isInteger(value)) {
    return String(value);
  }
  return value.toPrecision(12).replace(/\.?0+$/, "");
}
