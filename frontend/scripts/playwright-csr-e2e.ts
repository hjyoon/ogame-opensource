import { chromium, firefox, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type StepResult = {
  name: string;
  pass: boolean;
  details: Record<string, unknown>;
};

type LoginFixture = {
  login: string;
  password: string;
  universe: string;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-csr", browserName);
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
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

const results: StepResult[] = [];

try {
  const page = await browser.newPage({ viewport: { width: 1024, height: 768 }, deviceScaleFactor: 1, locale: "en-US" });
  page.on("console", (message) => {
    if (message.type() === "error") {
      results.push({ name: `console error: ${message.text()}`, pass: false, details: {} });
    }
  });
  page.on("requestfailed", (request) => {
    results.push({ name: `request failed: ${request.url()}`, pass: false, details: { method: request.method(), error: request.failure()?.errorText } });
  });

  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "domcontentloaded", timeout: 15_000 });
  await record("initial legacy CSS before app idle", async () => publicChromeState(page));
  await page.waitForLoadState("networkidle");
  await record("legacy CSS remains after app idle", async () => publicChromeState(page));
  await assertPublicLanguageFlagReload(page);

  await assertClientNavigation(page, "mainmenu about alias", "#mainmenu a[href='about.php']", "/about.php");
  await assertClientNavigation(page, "mainmenu screenshots alias", "#mainmenu a[href='screenshots.php']", "/screenshots.php");
  await assertClientNavigation(page, "downmenu rules alias", "#downmenu a[href='regeln.php']", "/regeln.php");
  await assertHistoryNavigation(page, "browser back preserves CSR", "back", "/screenshots.php");
  await assertHistoryNavigation(page, "browser forward preserves CSR", "forward", "/regeln.php");

  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  await assertClientNavigation(page, "home register CTA", "#register.bigbutton", "/register.php");

  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  const loginFixture = await createLoginFixture();
  await assertLoginFormRedirectsToGame(page, loginFixture);
  await record("game overview shell loads with session", async () => gameShellState(page, "login-form-submit", "Overview"));
  await assertRenamePlanetFlow(page);
  await assertGameClientNavigation(page, "game buildings menu preserves CSR", "a[href^='/game/buildings']", "/game/buildings", "Buildings");
  await assertGameClientNavigation(page, "game resources menu preserves CSR", "a[href^='/game/resources']", "/game/resources", "Resources");
  await assertGameClientNavigation(page, "game research menu preserves CSR", "a[href^='/game/research']", "/game/research", "Research");
  await assertGameClientNavigation(page, "game shipyard menu preserves CSR", "a[href^='/game/shipyard']", "/game/shipyard", "Shipyard");
  await assertGameClientNavigation(page, "game defense menu preserves CSR", "a[href^='/game/defense']", "/game/defense", "Defense");
  await assertGameClientNavigation(page, "game fleet menu preserves CSR", "a[href^='/game/fleet']", "/game/fleet", "Fleet");
  await assertGameClientNavigation(page, "game galaxy menu preserves CSR", "a[href^='/game/galaxy']", "/game/galaxy", "Galaxy");
  await assertGameClientNavigation(page, "game technology menu preserves CSR", "a[href^='/game/technology']", "/game/technology", "Technology");
  await assertTechnologyDetailsNavigation(page);
  await assertGameClientNavigation(page, "game statistics menu preserves CSR", "a[href^='/game/statistics']", "/game/statistics", "Statistics");
  await assertStatisticsAllianceMode(page);
  await assertGameClientNavigation(page, "game search menu preserves CSR", "a[href^='/game/search']", "/game/search", "Search");
  await assertSearchForm(page, loginFixture.login);
  await assertGameClientNavigation(page, "game notes menu preserves CSR", "a[href^='/game/notes']", "/game/notes", "Notes");
  await assertNotesMutationFlow(page);
  await assertGameClientNavigation(page, "game overview menu preserves CSR", "a[href^='/game/overview']", "/game/overview", "Overview");
  await assertGameLogout(page);

  const report = {
    generatedAt: new Date().toISOString(),
    migratedBaseURL,
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
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

async function assertClientNavigation(page: Page, name: string, selector: string, expectedPathname: string) {
  const marker = `probe-${name}`;
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator(selector).click();
  await page.waitForFunction((pathname) => window.location.pathname === pathname, expectedPathname, { timeout: 5_000 });
  await record(name, async () => {
    const state = await csrState(page);
    return {
      pass: state.pathname === expectedPathname && state.probe === marker && state.legacyCssLinks === 2 && state.legacyBody,
      details: state
    };
  });
}

async function assertHistoryNavigation(page: Page, name: string, direction: "back" | "forward", expectedPathname: string) {
  const marker = `probe-${name}`;
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  if (direction === "back") {
    await page.goBack();
  } else {
    await page.goForward();
  }
  await page.waitForFunction((pathname) => window.location.pathname === pathname, expectedPathname, { timeout: 5_000 });
  await record(name, async () => {
    const state = await csrState(page);
    return {
      pass: state.pathname === expectedPathname && state.probe === marker && state.legacyCssLinks === 2 && state.legacyBody,
      details: state
    };
  });
}

async function assertLoginFormRedirectsToGame(page: Page, fixture: LoginFixture) {
  await page.evaluate(() => {
    window.__ogameCsrProbe = "login-form-submit";
  });
  await page.locator("select[name='universe']").selectOption(fixture.universe);
  await page.locator("input[name='login']").fill(fixture.login);
  await page.locator("input[name='pass']").fill(fixture.password);
  await page.locator("input.legacy-public-login-button").click();
  await page.waitForFunction(() => window.location.pathname === "/game/overview" && window.location.search.includes("session="), undefined, {
    timeout: 10_000
  });
  await record("login form redirects directly to game", async () => {
    const state = await gameShellState(page, "login-form-submit", "Overview");
    return {
      pass: state.pass && state.details.openOverviewLinks === 0,
      details: state.details
    };
  });
}

async function assertPublicLanguageFlagReload(page: Page) {
  await page.context().clearCookies();
  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  await page.evaluate(() => {
    window.__ogameCsrProbe = "language-flag-before";
  });
  const navigation = page
    .waitForNavigation({ waitUntil: "domcontentloaded", timeout: 5_000 })
    .then(() => true)
    .catch(() => false);
  await page.locator("a:has(img[alt='Deutschland'])").click();
  const reloaded = await navigation;
  await page.waitForLoadState("networkidle").catch(() => undefined);
  await record("public language flag reloads like legacy", async () => {
    const state = await page.evaluate(() => ({
      pathname: window.location.pathname,
      hash: window.location.hash,
      cookie: document.cookie,
      probe: window.__ogameCsrProbe,
      legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
      legacyBody: document.body.classList.contains("legacy-public-body")
    }));
    return {
      pass:
        reloaded &&
        state.pathname === "/home" &&
        state.hash === "" &&
        state.cookie.split("; ").includes("ogamelang=de") &&
        state.probe === undefined &&
        state.legacyCssLinks === 2 &&
        state.legacyBody,
      details: { ...state, reloaded }
    };
  });
}

async function createLoginFixture(): Promise<LoginFixture> {
  const universesResponse = await fetch(`${migratedBaseURL}/api/public/universes`);
  const universesPayload = await universesResponse.json();
  const universe = universesPayload.universes?.[0]?.baseUrl ?? "http://localhost:8888";
  const suffix = `${browserName.slice(0, 2)}${Date.now().toString(36).slice(-8)}${Math.random().toString(36).slice(2, 4)}`;
  const login = `Csr${suffix}`.slice(0, 20);
  const password = "E2E_http123";
  const registrationResponse = await fetch(`${migratedBaseURL}/api/public/registration`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: login,
      password,
      email: `${login.toLowerCase()}@example.local`,
      universe,
      agb: true
    })
  });
  const registrationPayload = await registrationResponse.json();
  if (registrationResponse.status !== 200 || registrationPayload.valid !== true || registrationPayload.created !== true) {
    throw new Error(`Unable to create CSR login fixture: ${JSON.stringify(registrationPayload)}`);
  }
  return { login, password, universe };
}

async function assertGameClientNavigation(
  page: Page,
  name: string,
  selector: string,
  expectedPathname: string,
  expectedMenuLabel: string
) {
  const marker = `probe-${name}`;
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator(selector).click();
  await page.waitForFunction((pathname) => window.location.pathname === pathname, expectedPathname, { timeout: 5_000 });
  if (expectedMenuLabel === "Buildings") {
    await page.locator("[data-building-row]").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Resources") {
    await page.locator(".legacy-resources-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Research") {
    await page.locator(".legacy-research-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Shipyard") {
    await page.locator(".legacy-shipyard-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Fleet") {
    await page.locator(".legacy-fleet-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Galaxy") {
    await page.locator(".legacy-galaxy-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Defense") {
    await page.locator(".legacy-defense-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Technology") {
    await page.locator(".legacy-technology-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Statistics") {
    await page.locator(".legacy-statistics-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Search") {
    await page.locator(".legacy-search-head-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel === "Notes") {
    await page.locator(".legacy-notes-table").first().waitFor({ timeout: 10_000 });
  } else if (expectedMenuLabel !== "Overview") {
    await page.waitForFunction(() => document.body.textContent?.includes("queued for React and Go migration"), undefined, {
      timeout: 5_000
    });
  }
  await record(name, async () => {
    const state = await gameShellState(page, marker, expectedMenuLabel);
    const contentReady =
      expectedMenuLabel === "Buildings"
        ? state.details.buildingRows > 0 && state.details.buildingNames.includes("Metal Mine") && state.details.pendingText === false
        : expectedMenuLabel === "Resources"
          ? state.details.resourcesTable === true && state.details.resourceSettingsText === true && state.details.pendingText === false
          : expectedMenuLabel === "Research"
            ? state.details.researchTable === true && state.details.researchRows >= 0 && state.details.pendingText === false
            : expectedMenuLabel === "Shipyard"
              ? state.details.shipyardTable === true && state.details.shipyardRows >= 0 && state.details.pendingText === false
              : expectedMenuLabel === "Fleet"
                ? state.details.fleetTable === true &&
                  state.details.fleetSelectTable === true &&
                  state.details.fleetHeaderText.includes("Fleets") &&
                  state.details.pendingText === false
                : expectedMenuLabel === "Galaxy"
                  ? state.details.galaxyTable === true &&
                    state.details.galaxyRows === 15 &&
                    state.details.galaxyText.includes("Solar system") &&
                    state.details.pendingText === false
                  : expectedMenuLabel === "Defense"
                    ? state.details.defenseTable === true && state.details.defenseRows >= 0 && state.details.pendingText === false
                    : expectedMenuLabel === "Technology"
                      ? state.details.technologyTable === true &&
                        state.details.technologyRows > 0 &&
                        state.details.technologyNames.includes("Metal Mine") &&
                        state.details.pendingText === false
                      : expectedMenuLabel === "Statistics"
                        ? state.details.statisticsTable === true &&
                          state.details.statisticsRows > 0 &&
                          state.details.statisticsText.includes("Statistics") &&
                          state.details.pendingText === false
                        : expectedMenuLabel === "Search"
                          ? state.details.searchHeadTable === true &&
                            state.details.searchText.includes("Search Universe") &&
                            state.details.pendingText === false
                          : expectedMenuLabel === "Notes"
                            ? state.details.notesTable === true &&
                              state.details.notesText.includes("Create a new note") &&
                              state.details.pendingText === false
                          : expectedMenuLabel === "Overview"
                            ? state.details.pendingText === false
                            : state.details.pendingText === true;
    return {
      pass:
        state.details.pathname === expectedPathname &&
        state.details.probe === marker &&
        state.details.search.includes("session=") &&
        state.details.gameShell === true &&
        state.details.activeMenuLabel === expectedMenuLabel &&
        contentReady &&
        state.details.legacyCssLinks === 0 &&
        state.details.legacyBody === false,
      details: state.details
    };
  });
}

async function assertRenamePlanetFlow(page: Page) {
  const marker = "probe-game-rename-planet";
  const name = `CSR ${Date.now().toString(36).slice(-8)}`.slice(0, 20);
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator("a[title='Planet menu']").first().click();
  await page.waitForFunction((pathname) => window.location.pathname === pathname, "/game/rename-planet", { timeout: 5_000 });
  await page.locator(".legacy-rename-planet-table", { hasText: "Planet information" }).waitFor({ timeout: 10_000 });
  await record("game rename planet form preserves CSR", async () => {
    const state = await gameShellState(page, marker, "");
    return {
      pass:
        state.details.pathname === "/game/rename-planet" &&
        state.details.search.includes("session=") &&
        state.details.probe === marker &&
        state.details.gameShell === true &&
        state.details.renameTable === true &&
        state.details.renameTitle === "Rename/leave the planet" &&
        state.details.renameText.includes("Planet information") &&
        state.details.renameAbandonValue === "Abandon the colony" &&
        state.details.renameSubmitValue === "Rename" &&
        state.details.pendingText === false &&
        state.details.legacyCssLinks === 0 &&
        state.details.legacyBody === false,
      details: state.details
    };
  });

  await page.locator(".legacy-rename-planet-table input[name='newname']").fill(name);
  await page.locator(".legacy-rename-planet-table input[name='aktion'][value='Rename']").click();
  await page.locator(".legacy-rename-planet-table", { hasText: name }).waitFor({ timeout: 10_000 });
  await record("game rename planet mutation preserves CSR", async () => {
    const state = await gameShellState(page, marker, "");
    return {
      pass:
        state.details.pathname === "/game/rename-planet" &&
        state.details.search.includes("session=") &&
        state.details.probe === marker &&
        state.details.gameShell === true &&
        state.details.renameTable === true &&
        state.details.renameText.includes(name) &&
        state.details.pendingText === false &&
        state.details.legacyCssLinks === 0 &&
        state.details.legacyBody === false,
      details: state.details
    };
  });

  await page.locator(".legacy-rename-planet-table input[name='aktion'][value='Abandon the colony']").click();
  await page.locator(".legacy-rename-destroy-table", { hasText: "Just in case" }).waitFor({ timeout: 10_000 });
  await record("game rename planet abandon confirmation preserves CSR", async () => {
    const state = await gameShellState(page, marker, "");
    return {
      pass:
        state.details.pathname === "/game/rename-planet" &&
        state.details.search.includes("session=") &&
        state.details.probe === marker &&
        state.details.gameShell === true &&
        state.details.renameDestroyTable === true &&
        state.details.renameTitle === "Rename/leave the planet" &&
        state.details.renameDestroyText.includes("Just in case") &&
        state.details.renameDestroyText.includes("confirm password") &&
        state.details.renamePasswordInput === true &&
        state.details.renameDeleteValue === "Delete the planet!" &&
        state.details.pendingText === false &&
        state.details.legacyCssLinks === 0 &&
        state.details.legacyBody === false,
      details: state.details
    };
  });

  await page.locator(".legacy-rename-destroy-table input[name='pw']").fill("wrong-password");
  await page.locator(".legacy-rename-destroy-table input[name='aktion'][value='Delete the planet!']").click();
  await page.locator(".legacy-overview-table", { hasText: "The password is wrong." }).waitFor({ timeout: 10_000 });
  await record("game rename planet delete wrong password preserves CSR", async () => {
    const state = await gameShellState(page, marker, "");
    return {
      pass:
        state.details.pathname === "/game/rename-planet" &&
        state.details.search.includes("session=") &&
        state.details.probe === marker &&
        state.details.gameShell === true &&
        state.details.renameTable === true &&
        state.details.overviewErrorText.includes("The password is wrong.") &&
        state.details.pendingText === false &&
        state.details.legacyCssLinks === 0 &&
        state.details.legacyBody === false,
      details: state.details
    };
  });
}

async function assertTechnologyDetailsNavigation(page: Page) {
  const marker = "probe-game-technology-details";
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator("[data-technology-row] a", { hasText: "[i]" }).first().click();
  await page.waitForFunction(() => window.location.pathname === "/game/technology" && window.location.search.includes("tid="), undefined, {
    timeout: 5_000
  });
  await page.locator(".legacy-technology-details-table").first().waitFor({ timeout: 10_000 });
  await record("game technology details preserves CSR", async () => {
    const state = await gameShellState(page, marker, "Technology");
    return {
      pass:
        state.details.pathname === "/game/technology" &&
        state.details.search.includes("session=") &&
        state.details.search.includes("tid=") &&
        state.details.probe === marker &&
        state.details.gameShell === true &&
        state.details.activeMenuLabel === "Technology" &&
        state.details.technologyDetailTable === true &&
        state.details.technologyDetailTarget.includes("Building conditions for") &&
        state.details.pendingText === false &&
        state.details.legacyCssLinks === 0 &&
        state.details.legacyBody === false,
      details: state.details
    };
  });
}

async function assertStatisticsAllianceMode(page: Page) {
  const marker = "probe-game-statistics-alliance";
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator("select[name='who']").selectOption("ally");
  await page.locator(".legacy-statistics-head-table input[type='submit']").click();
  await page.waitForFunction(() => window.location.pathname === "/game/statistics" && window.location.search.includes("who=ally"), undefined, {
    timeout: 5_000
  });
  await page.locator(".legacy-statistics-table", { hasText: "Per person" }).waitFor({ timeout: 10_000 });
  await record("game statistics alliance mode preserves CSR", async () => {
    const state = await gameShellState(page, marker, "Statistics");
    return {
      pass:
        state.pass &&
        state.details.pathname === "/game/statistics" &&
        state.details.search.includes("who=ally") &&
        !state.details.search.includes("tid=") &&
        state.details.statisticsTable === true &&
        state.details.statisticsRows >= 0 &&
        state.details.statisticsBodyText.includes("Alliance") &&
        state.details.statisticsBodyText.includes("Num.") &&
        state.details.statisticsBodyText.includes("Per person") &&
        state.details.pendingText === false,
      details: state.details
    };
  });
}

async function assertSearchForm(page: Page, login: string) {
  const marker = "probe-game-search-form";
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator("select[name='type']").selectOption("playername");
  await page.locator("input[name='searchtext']").fill(login);
  await page.locator(".legacy-search-head-table input[type='submit']").click();
  await page.waitForFunction(() => window.location.pathname === "/game/search" && window.location.search.includes("searchtext="), undefined, {
    timeout: 5_000
  });
  await page.locator(".legacy-search-results-table", { hasText: login }).waitFor({ timeout: 10_000 });
  await record("game search form preserves CSR", async () => {
    const state = await gameShellState(page, marker, "Search");
    return {
      pass:
        state.pass &&
        state.details.pathname === "/game/search" &&
        state.details.search.includes("type=playername") &&
        state.details.search.includes("searchtext=") &&
        state.details.searchHeadTable === true &&
        state.details.searchRows > 0 &&
        state.details.searchBodyText.includes(login) &&
        state.details.searchBodyText.includes("Position") &&
        state.details.pendingText === false,
      details: state.details
    };
  });
}

async function assertNotesMutationFlow(page: Page) {
  const marker = "probe-game-notes-create";
  const subject = `CSR note ${Date.now()}`;
  const updatedSubject = `${subject} updated`;
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator(".legacy-notes-table a", { hasText: "Create a new note" }).click();
  await page.waitForFunction(() => window.location.pathname === "/game/notes" && window.location.search.includes("a=1"), undefined, {
    timeout: 5_000
  });
  await page.locator(".legacy-notes-form-table", { hasText: "Create note" }).waitFor({ timeout: 10_000 });
  await record("game notes create form preserves CSR", async () => {
    const state = await gameShellState(page, marker, "Notes");
    return {
      pass:
        state.details.pathname === "/game/notes" &&
        state.details.search.includes("session=") &&
        state.details.search.includes("a=1") &&
        state.details.probe === marker &&
        state.details.notesFormTable === true &&
        state.details.notesFormText.includes("Create note") &&
        state.details.notesFormText.includes("Priority") &&
        state.details.notesFormText.includes("Notice") &&
        state.details.pendingText === false,
      details: state.details
    };
  });

  await page.locator(".legacy-notes-form-table input[name='betreff']").fill(subject);
  await page.locator(".legacy-notes-form-table textarea[name='text']").fill("Body");
  await page.locator(".legacy-notes-form-table select[name='u']").selectOption("2");
  await page.locator(".legacy-notes-form-table input[type='submit']").click();
  await page.waitForFunction(() => window.location.pathname === "/game/notes" && !window.location.search.includes("a=1"), undefined, {
    timeout: 5_000
  });
  await page.locator(".legacy-notes-table", { hasText: subject }).waitFor({ timeout: 10_000 });

  await page.locator(".legacy-notes-table a", { hasText: subject }).first().click();
  await page.waitForFunction(() => window.location.pathname === "/game/notes" && window.location.search.includes("a=2"), undefined, {
    timeout: 5_000
  });
  await page.locator(".legacy-notes-form-table", { hasText: "Edit note" }).waitFor({ timeout: 10_000 });
  await page.locator(".legacy-notes-form-table input[name='betreff']").fill(updatedSubject);
  await page.locator(".legacy-notes-form-table textarea[name='text']").fill("Updated body");
  await page.locator(".legacy-notes-form-table select[name='u']").selectOption("0");
  await page.locator(".legacy-notes-form-table input[type='submit']").click();
  await page.waitForFunction(() => window.location.pathname === "/game/notes" && !window.location.search.includes("a=2"), undefined, {
    timeout: 5_000
  });
  await page.locator(".legacy-notes-table", { hasText: updatedSubject }).waitFor({ timeout: 10_000 });

  const row = page.locator("[data-note-row]", { hasText: updatedSubject }).first();
  await row.locator("input[type='checkbox']").check();
  await page.locator(".legacy-notes-table input[type='submit']").click();
  await page.waitForFunction((text) => !document.body.textContent?.includes(text), updatedSubject, { timeout: 10_000 });
  await record("game notes mutations preserve CSR", async () => {
    const state = await gameShellState(page, marker, "Notes");
    return {
      pass:
        state.details.pathname === "/game/notes" &&
        state.details.search.includes("session=") &&
        state.details.probe === marker &&
        state.details.notesTable === true &&
        !state.details.notesText.includes(updatedSubject) &&
        state.details.pendingText === false,
      details: state.details
    };
  });
}

async function assertGameLogout(page: Page) {
  const marker = "probe-game-logout";
  await page.evaluate((value) => {
    window.__ogameCsrProbe = value;
  }, marker);
  await page.locator("a[href^='/game/logout']").click();
  await page.waitForFunction((pathname) => window.location.pathname === pathname, "/game/logout", { timeout: 5_000 });
  await page.locator(".legacy-logout-table", { hasText: "See you soon!!" }).waitFor({ timeout: 10_000 });
  await record("game logout preserves CSR and shows legacy message", async () => {
    const state = await gameShellState(page, marker, "Logout");
    return {
      pass:
        state.pass &&
        state.details.pathname === "/game/logout" &&
        state.details.logoutTable === true &&
        state.details.logoutText.includes("See you soon!!") &&
        state.details.pendingText === false,
      details: state.details
    };
  });
  await page.waitForFunction(() => window.location.pathname === "/home", undefined, { timeout: 6_000 });
  await record("game logout redirects home through CSR", async () => {
    const state = await csrState(page);
    return {
      pass: state.pathname === "/home" && state.probe === marker && state.legacyCssLinks === 2 && state.legacyBody === true,
      details: state
    };
  });
}

async function publicChromeState(page: Page) {
  const state = await page.evaluate(() => ({
    legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
    legacyBody: document.body.classList.contains("legacy-public-body"),
    bodyBackground: getComputedStyle(document.body).backgroundImage
  }));
  return {
    pass: state.legacyCssLinks === 2 && state.legacyBody && state.bodyBackground.includes("sterne_bg2.jpg"),
    details: state
  };
}

async function gameShellState(page: Page, expectedProbe: string, expectedMenuLabel: string) {
  const state = await page.evaluate(() => ({
    pathname: window.location.pathname,
    search: window.location.search,
    probe: window.__ogameCsrProbe,
    gameShell: document.querySelector(".legacy-game-shell") !== null,
    activeMenuLabel: document.querySelector(".legacy-menu a[aria-current='page']")?.textContent?.trim() ?? "",
    legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
    legacyBody: document.body.classList.contains("legacy-public-body"),
    openOverviewLinks: Array.from(document.querySelectorAll("a")).filter((link) => link.textContent?.trim() === "Open overview").length,
    overviewErrorText:
      Array.from(document.querySelectorAll(".legacy-overview-table"))
        .find((table) => table.textContent?.includes("The password is wrong."))
        ?.textContent?.trim()
        .replace(/\s+/g, " ") ?? "",
    buildingRows: document.querySelectorAll("[data-building-row]").length,
    buildingNames: Array.from(document.querySelectorAll("[data-building-row] .legacy-building-description a")).map(
      (link) => link.textContent?.trim() ?? ""
    ),
    resourceRows: document.querySelectorAll("[data-resource-row]").length,
    resourcesTable: document.querySelector(".legacy-resources-table") !== null,
    resourceSettingsText: document.body.textContent?.includes("Resource settings on planet") ?? false,
    resourceNames: Array.from(document.querySelectorAll("[data-resource-row] th:first-child")).map(
      (cell) => cell.textContent?.trim().replace(/\s+/g, " ") ?? ""
    ),
    researchRows: document.querySelectorAll("[data-research-row]").length,
    researchTable: document.querySelector(".legacy-research-table") !== null,
    researchNames: Array.from(document.querySelectorAll("[data-research-row] .legacy-building-description a")).map(
      (link) => link.textContent?.trim() ?? ""
    ),
    shipyardRows: document.querySelectorAll("[data-shipyard-row]").length,
    shipyardTable: document.querySelector(".legacy-shipyard-table") !== null,
    shipyardNames: Array.from(document.querySelectorAll("[data-shipyard-row] .legacy-building-description a")).map(
      (link) => link.textContent?.trim() ?? ""
    ),
    fleetTable: document.querySelector(".legacy-fleet-table") !== null,
    fleetSelectTable: document.querySelector(".legacy-fleet-select-table") !== null,
    fleetMissionRows: document.querySelectorAll("[data-fleet-mission-row]").length,
    fleetShipRows: document.querySelectorAll("[data-fleet-ship-row]").length,
    fleetHeaderText: document.querySelector(".legacy-fleet-table tr:first-child td")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    galaxyTable: document.querySelector(".legacy-galaxy-table") !== null,
    galaxyRows: document.querySelectorAll("[data-galaxy-position]").length,
    galaxyText: document.querySelector(".legacy-galaxy-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    defenseRows: document.querySelectorAll("[data-defense-row]").length,
    defenseTable: document.querySelector(".legacy-defense-table") !== null,
    defenseNames: Array.from(document.querySelectorAll("[data-defense-row] .legacy-building-description a")).map(
      (link) => link.textContent?.trim() ?? ""
    ),
    technologyRows: document.querySelectorAll("[data-technology-row]").length,
    technologyTable: document.querySelector(".legacy-technology-table") !== null,
    technologyNames: Array.from(document.querySelectorAll("[data-technology-row] .legacy-technology-name-link")).map(
      (link) => link.textContent?.trim() ?? ""
    ),
    technologyDetailRows: document.querySelectorAll("[data-technology-detail-row]").length,
    technologyDetailTable: document.querySelector(".legacy-technology-details-table") !== null,
    technologyDetailTarget: document.querySelector(".legacy-technology-details-table tr:first-child td")?.textContent?.trim() ?? "",
    statisticsTable: document.querySelector(".legacy-statistics-table") !== null,
    statisticsRows: document.querySelectorAll("[data-statistics-row]").length,
    statisticsText: document.querySelector(".legacy-statistics-head-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    statisticsBodyText: document.querySelector(".legacy-statistics-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    searchHeadTable: document.querySelector(".legacy-search-head-table") !== null,
    searchResultsTable: document.querySelector(".legacy-search-results-table") !== null,
    searchRows: document.querySelectorAll("[data-search-row]").length,
    searchText: document.querySelector(".legacy-search-head-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    searchBodyText: document.querySelector(".legacy-search-results-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    notesTable: document.querySelector(".legacy-notes-table") !== null,
    notesRows: document.querySelectorAll("[data-note-row]").length,
    notesText: document.querySelector(".legacy-notes-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    notesFormTable: document.querySelector(".legacy-notes-form-table") !== null,
    notesFormText: document.querySelector(".legacy-notes-form-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    renameTable: document.querySelector(".legacy-rename-planet-table") !== null,
    renameTitle: document.querySelector("h1")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    renameText: document.querySelector(".legacy-rename-planet-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    renameAbandonValue:
      document.querySelector<HTMLInputElement>(".legacy-rename-planet-table input[name='aktion'][value='Abandon the colony']")
        ?.value ?? "",
    renameSubmitValue:
      document.querySelector<HTMLInputElement>(".legacy-rename-planet-table input[name='aktion'][value='Rename']")?.value ?? "",
    renameDestroyTable: document.querySelector(".legacy-rename-destroy-table") !== null,
    renameDestroyText: document.querySelector(".legacy-rename-destroy-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    renamePasswordInput: document.querySelector(".legacy-rename-destroy-table input[name='pw']") !== null,
    renameDeleteValue:
      document.querySelector<HTMLInputElement>(".legacy-rename-destroy-table input[name='aktion'][value='Delete the planet!']")
        ?.value ?? "",
    logoutTable: document.querySelector(".legacy-logout-table") !== null,
    logoutText: document.querySelector(".legacy-logout-table")?.textContent?.trim().replace(/\s+/g, " ") ?? "",
    pendingText: document.body.textContent?.includes("queued for React and Go migration") ?? false
  }));
  return {
    pass:
      state.gameShell &&
      state.search.includes("session=") &&
      state.activeMenuLabel === expectedMenuLabel &&
      state.probe === expectedProbe &&
      state.legacyCssLinks === 0 &&
      !state.legacyBody,
    details: state
  };
}

async function csrState(page: Page) {
  return await page.evaluate(() => ({
    pathname: window.location.pathname,
    probe: window.__ogameCsrProbe,
    legacyCssLinks: document.head.querySelectorAll("link[data-legacy-public-css]").length,
    legacyBody: document.body.classList.contains("legacy-public-body")
  }));
}

async function record(name: string, run: () => Promise<{ pass: boolean; details: Record<string, unknown> }>) {
  try {
    const result = await run();
    results.push({ name, ...result });
  } catch (error) {
    results.push({ name, pass: false, details: { error: error instanceof Error ? error.message : String(error) } });
  }
}

function renderMarkdown(report: {
  generatedAt: string;
  migratedBaseURL: string;
  browserName: BrowserName;
  browserExecutable: string;
  allPass: boolean;
  results: StepResult[];
}) {
  const lines = [
    "# Playwright CSR E2E Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    "",
    "| Step | Pass | Details |",
    "| --- | --- | --- |"
  ];
  for (const result of report.results) {
    lines.push(`| ${result.name} | ${result.pass ? "PASS" : "FAIL"} | ${JSON.stringify(result.details).replaceAll("|", "\\|")} |`);
  }
  lines.push("");
  return lines.join("\n");
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name];
  if (raw === "chromium" || raw === "firefox") {
    return raw;
  }
  return fallback;
}

declare global {
  interface Window {
    __ogameCsrProbe?: string;
  }
}
