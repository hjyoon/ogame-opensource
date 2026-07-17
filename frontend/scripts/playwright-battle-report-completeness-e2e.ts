import { chromium, firefox } from "@playwright/test";

type BrowserName = "chromium" | "firefox";

const browserName = browserEnv(process.env.OGAME_PLAYWRIGHT_BROWSER ?? "chromium");
const baseURL = (process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890").replace(/\/+$/, "");
const session = requiredEnv("OGAME_BATTLE_REPORT_SESSION");
const reportID = requiredEnv("OGAME_BATTLE_REPORT_ID");
const cookieName = requiredEnv("OGAME_BATTLE_REPORT_COOKIE_NAME");
const cookieValue = requiredEnv("OGAME_BATTLE_REPORT_COOKIE_VALUE");
const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({ headless: true });

try {
  const context = await browser.newContext();
  await context.addCookies([
    { name: "ogamelang", value: "en", url: baseURL },
    { name: cookieName, value: cookieValue, url: baseURL }
  ]);
  const page = await context.newPage();
  const failures: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") {
      failures.push(`console: ${message.text()}`);
    }
  });
  page.on("requestfailed", (request) => failures.push(`request: ${request.url()} ${request.failure()?.errorText ?? ""}`));

  const response = await page.goto(
    `${baseURL}/game/report?bericht=${encodeURIComponent(reportID)}&session=${encodeURIComponent(session)}`,
    { waitUntil: "domcontentloaded" }
  );
  if (!response?.ok()) {
    throw new Error(`report page returned ${response?.status() ?? "no response"}`);
  }
  const report = page.locator(".legacy-report-table");
  await report.waitFor({ state: "visible" });
  const text = (await report.innerText()).replace(/\s+/g, " ").trim();
  const html = await report.innerHTML();

  if (!/(attacker has won|defender has won|battle ended in a draw)/i.test(text)) {
    throw new Error(`battle outcome is missing from rendered report: ${text}`);
  }
  for (const marker of ["The attacker lost a total", "The defender lost a total", "At these space coordinates now float"]) {
    if (!text.includes(marker)) {
      throw new Error(`rendered report is missing tail marker ${JSON.stringify(marker)}: ${text}`);
    }
  }
  if (!html.includes("<table") || !html.includes("</table>")) {
    throw new Error(`rendered report is missing fleet tables: ${html}`);
  }
  if (failures.length > 0) {
    throw new Error(failures.join("\n"));
  }
  await context.close();
  process.stdout.write(`Battle report DOM completeness E2E passed (${browserName}, report=${reportID})\n`);
} finally {
  await browser.close();
}

function browserEnv(value: string): BrowserName {
  if (value === "chromium" || value === "firefox") {
    return value;
  }
  throw new Error(`unsupported browser ${JSON.stringify(value)}`);
}

function requiredEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}
