import { chromium, type Locator, type Page } from "@playwright/test";
import { existsSync } from "node:fs";

const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const loginUser = process.env.OGAME_GALAXY_HOVER_USER ?? "legor";
const loginPassword = process.env.OGAME_GALAXY_HOVER_PASS ?? "admin";
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ?? (existsSync(defaultChromeExecutable) ? defaultChromeExecutable : undefined);

const browser = await chromium.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const page = await browser.newPage({ viewport: { width: 1024, height: 768 }, deviceScaleFactor: 1, locale: "en-US" });
  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  const universe = (await page.locator("select[name='universe'] option").nth(1).getAttribute("value")) ?? "http://localhost:8888";
  await page.locator("select[name='universe']").selectOption(universe);
  await page.locator("input[name='login']").fill(loginUser);
  await page.locator("input[name='pass']").fill(loginPassword);
  await page.locator("input.legacy-public-login-button").click();
  await page.waitForFunction(() => window.location.pathname === "/game/overview" && window.location.search.includes("session="), undefined, {
    timeout: 15_000
  });
  const sessionSearch = windowSearch(page.url());
  await page.goto(`${migratedBaseURL}/game/galaxy${sessionSearch}`, { waitUntil: "networkidle", timeout: 15_000 });
  await page.locator(".legacy-galaxy-table").waitFor({ timeout: 15_000 });
  await page.locator(".legacy-galaxy-table .legacy-galaxy-hover").first().hover();
  await page.waitForTimeout(850);
  const tooltip = page.locator(".legacy-galaxy-hover-open .legacy-galaxy-tooltip").first();
  await tooltip.waitFor({ timeout: 5_000 });
  const tooltipUX = await verifyTooltipUX(page, tooltip);

  const result = await page.evaluate(() => {
    const tooltip = document.querySelector(".legacy-galaxy-hover-open .legacy-galaxy-tooltip");
    const style = tooltip ? window.getComputedStyle(tooltip) : null;
    const text = tooltip?.textContent?.trim().replace(/\s+/g, " ") ?? "";
    const ownPlanetActions = text.includes("Deploy") && text.includes("Transport");
    const targetPlanetActions = text.includes("Espionage") && text.includes("Attack") && text.includes("Defend") && text.includes("Transport");
    return {
      hoverCount: document.querySelectorAll(".legacy-galaxy-hover").length,
      instantLinks: document.querySelectorAll("a[data-galaxy-instant]").length,
      pass: Boolean(style && style.display !== "none" && style.visibility !== "hidden" && text.includes("Planet") && (ownPlanetActions || targetPlanetActions)),
      text,
      tooltipCount: document.querySelectorAll(".legacy-galaxy-tooltip").length
    };
  });

  console.log(JSON.stringify({ migratedBaseURL, loginUser, ...result, tooltipUX }, null, 2));
  if (!result.pass || !tooltipUX.pass) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

function windowSearch(value: string): string {
  const search = new URL(value).search;
  return search || "";
}

async function verifyTooltipUX(page: Page, tooltip: Locator) {
  const before = await tooltip.boundingBox();
  if (!before) {
    return { pass: false, reason: "tooltip bounding box missing" };
  }
  const cursor = await tooltip.evaluate((element) => window.getComputedStyle(element).cursor);
  const firstLink = tooltip.locator("a").first();
  const linkCursor = (await firstLink.count()) > 0 ? await firstLink.evaluate((element) => window.getComputedStyle(element).cursor) : null;
  const outsideX = before.x > 80 ? before.x - 40 : Math.min(1000, before.x + before.width + 40);
  const outsideY = before.y > 80 ? before.y - 40 : Math.min(740, before.y + before.height + 40);
  await page.mouse.move(outsideX, outsideY);
  await page.waitForTimeout(120);
  await page.mouse.move(before.x + before.width / 2, before.y + before.height / 2);
  await page.waitForTimeout(150);
  const after = await tooltip.boundingBox();
  const positionDelta = after ? { x: Math.abs(after.x - before.x), y: Math.abs(after.y - before.y) } : { x: Number.POSITIVE_INFINITY, y: Number.POSITIVE_INFINITY };
  const visibleAfterReentry = await tooltip.isVisible().catch(() => false);
  const positionStable = positionDelta.x <= 1 && positionDelta.y <= 1;
  return {
    cursor,
    linkCursor,
    pass: cursor !== "pointer" && (linkCursor === null || linkCursor === "pointer") && visibleAfterReentry && positionStable,
    positionDelta,
    positionStable,
    visibleAfterReentry
  };
}
