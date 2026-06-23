import { chromium } from "@playwright/test";
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
  await page.locator(".legacy-galaxy-hover-open .legacy-galaxy-tooltip").first().waitFor({ timeout: 5_000 });

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

  console.log(JSON.stringify({ migratedBaseURL, loginUser, ...result }, null, 2));
  if (!result.pass) {
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
