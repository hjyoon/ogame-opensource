import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type Fixture = {
  user: string;
  player_id: number;
  planet_id: number;
  own_fleet_id: number;
  own_elapsed_seconds: number;
  prepared_at: number;
  session: string;
  private_cookie_name: string;
  private_cookie_value: string;
};

type FleetMission = {
  id: number;
  own: boolean;
  mission: number;
  missionName: string;
  stateShort: string;
  departureAt: number;
  arrivalAt: number;
  canRecall: boolean;
};

type FleetStatus = {
  authenticated: boolean;
  issues: Array<{ code: string; message: string }>;
  actionIssue?: { code: string; message: string };
  fleet?: { missions: FleetMission[] };
};

type Diagnostics = {
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const baseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const fixturePath = resolve(
  process.env.OGAME_OVERVIEW_FLEET_FIXTURE_FILE ?? join(rootDir, ".tmp/overview-fleet-fixture.json")
);
const outputDir = resolve(rootDir, ".tmp/playwright-fleet-recall-timing", browserName);
const fixture = JSON.parse(await readFile(fixturePath, "utf8")) as Fixture;
const chromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const executablePath =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (browserName === "chromium" && existsSync(chromeExecutable) ? chromeExecutable : undefined);

if (fixture.own_elapsed_seconds <= 0) {
  throw new Error("Recall timing fixture must set OGAME_OVERVIEW_FLEET_OWN_ELAPSED_SECONDS to a positive value.");
}

await mkdir(outputDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(executablePath ? { executablePath } : {}),
  headless: true
});

try {
  const context = await authenticatedContext(browser, baseURL, fixture);
  const { page, diagnostics } = await monitoredPage(context);
  const fleetURL = `${baseURL}/game/fleet?${new URLSearchParams({
    session: fixture.session,
    cp: String(fixture.planet_id)
  }).toString()}`;

  const navigation = await page.goto(fleetURL, { waitUntil: "networkidle", timeout: 20_000 });
  const recallButton = page.locator(".legacy-fleet-table input[type='submit'][value='Recall']");
  await recallButton.waitFor({ timeout: 10_000 });

  const buttonCountBefore = await recallButton.count();
  const beforeClick = Math.floor(Date.now() / 1000);
  const responsePromise = page.waitForResponse(
    (response) => response.request().method() === "POST" && new URL(response.url()).pathname === "/api/game/fleet",
    { timeout: 15_000 }
  );
  await recallButton.first().click();
  const recallResponse = await responsePromise;
  const payload = (await recallResponse.json()) as FleetStatus;
  const afterResponse = Math.floor(Date.now() / 1000);

  await recallButton.waitFor({ state: "detached", timeout: 10_000 });
  const buttonCountAfter = await recallButton.count();
  const returnMission = payload.fleet?.missions.find((mission) => mission.own && mission.mission === 103);
  const queueStart = fixture.prepared_at - fixture.own_elapsed_seconds;
  const derivedRecallAt = returnMission ? (returnMission.arrivalAt + queueStart) / 2 : 0;
  const derivedRecallAtIsInteger = Number.isInteger(derivedRecallAt);
  const returnDuration = returnMission ? returnMission.arrivalAt - derivedRecallAt : 0;
  const outboundElapsed = derivedRecallAt - queueStart;
  const timingFormulaPass =
    returnMission !== undefined &&
    derivedRecallAtIsInteger &&
    derivedRecallAt >= beforeClick - 1 &&
    derivedRecallAt <= afterResponse + 1 &&
    returnDuration === outboundElapsed &&
    returnDuration >= fixture.own_elapsed_seconds;
  const missionPass =
    returnMission !== undefined &&
    returnMission.id !== fixture.own_fleet_id &&
    returnMission.canRecall === false &&
    !payload.fleet?.missions.some((mission) => mission.id === fixture.own_fleet_id);

  const reload = await page.reload({ waitUntil: "networkidle", timeout: 20_000 });
  await page.locator(".legacy-fleet-table").waitFor({ timeout: 10_000 });
  const buttonCountAfterReload = await page.locator(".legacy-fleet-table input[type='submit'][value='Recall']").count();
  const returningRowCount = await page
    .locator(".legacy-fleet-table tr")
    .filter({ hasText: returnMission?.missionName ?? "Fleet Returns home" })
    .count();

  const pass =
    navigation?.status() === 200 &&
    recallResponse.status() === 200 &&
    reload?.status() === 200 &&
    payload.authenticated &&
    payload.issues.length === 0 &&
    payload.actionIssue === undefined &&
    buttonCountBefore === 1 &&
    buttonCountAfter === 0 &&
    buttonCountAfterReload === 0 &&
    returningRowCount >= 1 &&
    missionPass &&
    timingFormulaPass &&
    diagnostics.consoleErrors.length === 0 &&
    diagnostics.failedRequests.length === 0 &&
    diagnostics.badResponses.length === 0;

  const report = {
    generatedAt: new Date().toISOString(),
    browserName,
    baseURL,
    fixture: {
      user: fixture.user,
      playerID: fixture.player_id,
      planetID: fixture.planet_id,
      originalFleetID: fixture.own_fleet_id,
      preparedAt: fixture.prepared_at,
      forcedElapsedSeconds: fixture.own_elapsed_seconds,
      queueStart
    },
    pass,
    missionPass,
    timingFormulaPass,
    navigationStatus: navigation?.status() ?? null,
    recallStatus: recallResponse.status(),
    reloadStatus: reload?.status() ?? null,
    beforeClick,
    afterResponse,
    derivedRecallAt,
    returnDuration,
    outboundElapsed,
    buttonCountBefore,
    buttonCountAfter,
    buttonCountAfterReload,
    returningRowCount,
    returnMission: returnMission ?? null,
    actionIssue: payload.actionIssue ?? null,
    diagnostics
  };
  const reportPath = join(outputDir, "report.json");
  await writeFile(reportPath, JSON.stringify(report, null, 2));
  process.stdout.write(
    JSON.stringify(
      {
        pass,
        browserName,
        forcedElapsedSeconds: fixture.own_elapsed_seconds,
        returnDuration,
        derivedRecallAt,
        clickWindow: [beforeClick, afterResponse],
        report: reportPath
      },
      null,
      2
    ) + "\n"
  );
  if (!pass) {
    process.exitCode = 1;
  }

  await context.close();
} finally {
  await browser.close();
}

async function authenticatedContext(browser: Browser, url: string, fixtureData: Fixture): Promise<BrowserContext> {
  const context = await browser.newContext({
    viewport: { width: 1024, height: 768 },
    deviceScaleFactor: 1,
    locale: "en-US"
  });
  await context.addCookies([
    {
      name: fixtureData.private_cookie_name,
      value: fixtureData.private_cookie_value,
      url
    }
  ]);
  return context;
}

async function monitoredPage(context: BrowserContext): Promise<{ page: Page; diagnostics: Diagnostics }> {
  const page = await context.newPage();
  const diagnostics: Diagnostics = {
    consoleErrors: [],
    failedRequests: [],
    badResponses: []
  };
  page.on("console", (message) => {
    if (message.type() === "error") {
      diagnostics.consoleErrors.push(message.text());
    }
  });
  page.on("requestfailed", (request) => {
    diagnostics.failedRequests.push(`${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`.trim());
  });
  page.on("response", (response) => {
    if (response.status() >= 400 && !response.url().endsWith("/favicon.ico")) {
      diagnostics.badResponses.push(`${response.status()} ${response.url()}`);
    }
  });
  return { page, diagnostics };
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const value = process.env[name] ?? fallback;
  if (value !== "chromium" && value !== "firefox") {
    throw new Error(`${name} must be chromium or firefox, got ${value}`);
  }
  return value;
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
