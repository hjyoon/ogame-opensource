import { chromium, firefox, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { compareScreenshots, trimTrailingSlash, waitForImages, waitForStablePaint, type DiffResult } from "./visual/game-visual-utils";

type BrowserName = "chromium" | "firefox";
type SideName = "legacy" | "migrated";

type RegisterState = {
  url: string;
  status: number | null;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  states: Record<string, { info: string; status: string; statusClass: string }>;
};

type LegacyNewFormState = {
  url: string;
  status: number | null;
  screenshotPath: string;
  title: string;
  playerInfoHeader: string;
  playerNameLabel: string;
  emailLabel: string;
  acceptLabel: string;
  submitValue: string;
  infoHeader: string;
  formMethod: string;
  formAction: string;
  inputNames: string[];
  cssHrefs: string[];
  legacyPublicChrome: boolean;
  legacyGameChrome: boolean;
  legacyRegistrationChrome: boolean;
};

type LegacyNewFormComparison = {
  pass: boolean;
  diff: DiffResult;
  mismatches: string[];
  legacy: LegacyNewFormState;
  migrated: LegacyNewFormState;
};

type ActivationFlowState = {
  pass: boolean;
  username: string;
  email: string;
  activationURL: string;
  activationOriginMatches: boolean;
  finalURL: string;
  overviewLoaded: boolean;
  optionsLoaded: boolean;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  error: string;
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, `.tmp/playwright-public-registration-dynamic/${browserName}`);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const mailhogAPIURL = process.env.OGAME_MAILHOG_API_URL ?? "http://127.0.0.1:8026/api/v2/messages?limit=100";
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);

await mkdir(screenshotDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

if (process.env.OGAME_PUBLIC_REGISTRATION_ACTIVATION_ONLY === "1") {
  const context = await newContext(migratedBaseURL);
  let pass = false;
  try {
    const activationFlow = await runMigratedActivationFlow(context);
    pass = activationFlow.pass;
    const reportPath = join(outputDir, "activation-flow-report.json");
    await writeFile(
      reportPath,
      JSON.stringify({ generatedAt: new Date().toISOString(), browserName, migratedBaseURL, mailhogAPIURL, activationFlow }, null, 2)
    );
    process.stdout.write(JSON.stringify({ pass: activationFlow.pass, report: reportPath }, null, 2) + "\n");
  } finally {
    await context.close();
    await browser.close();
  }
  process.exit(pass ? 0 : 1);
}

try {
  const legacyContext = await newContext(legacyBaseURL);
  const migratedContext = await newContext(migratedBaseURL);
  const legacy = await runSide(legacyContext, "legacy");
  const migrated = await runSide(migratedContext, "migrated");
  const legacyNewForm = await compareLegacyNewForm();
  await legacyContext.close();
  await migratedContext.close();

  const comparisons = compareStates(legacy, migrated);
  const pass =
    legacy.status === 200 &&
    migrated.status === 200 &&
    legacy.consoleErrors.length === 0 &&
    migrated.consoleErrors.length === 0 &&
    legacy.failedRequests.length === 0 &&
    migrated.failedRequests.length === 0 &&
    legacy.badResponses.length === 0 &&
    migrated.badResponses.length === 0 &&
    comparisons.length === 0 &&
    legacyNewForm.pass;

  const report = {
    generatedAt: new Date().toISOString(),
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    legacyBaseURL,
    migratedBaseURL,
    pass,
    legacy,
    migrated,
    legacyNewForm,
    comparisons
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(JSON.stringify({ pass, report: join(outputDir, "report.json") }, null, 2) + "\n");
  if (!pass) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

async function runMigratedActivationFlow(context: BrowserContext): Promise<ActivationFlowState> {
  const page = await context.newPage();
  const consoleErrors: string[] = [];
  const failedRequests: string[] = [];
  const badResponses: string[] = [];
  const suffix = `${browserName.slice(0, 2)}${Date.now().toString(36).slice(-8)}${Math.random().toString(36).slice(2, 4)}`;
  const username = `Act${suffix}`.slice(0, 20);
  const email = `activation-${username.toLowerCase()}@example.local`;
  let activationURL = "";
  let activationOriginMatches = false;
  let finalURL = "";
  let overviewLoaded = false;
  let optionsLoaded = false;
  let error = "";

  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });
  page.on("requestfailed", (request) => {
    failedRequests.push(`${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`.trim());
  });
  page.on("response", (response) => {
    if (response.status() >= 400 && !response.url().endsWith("/favicon.ico")) {
      badResponses.push(`${response.status()} ${response.url()}`);
    }
  });

  try {
    await page.goto(`${migratedBaseURL}/register`, { waitUntil: "domcontentloaded", timeout: 20_000 });
    await page.locator("form[name='registerForm']").waitFor({ timeout: 10_000 });
    await waitForUniverse(page);
    await page.locator("input[name='character']").fill(username);
    await page.locator("input[name='email']").fill(email);
    await page.locator("input[name='password']").fill("E2E_http123");
    await page.locator("input[name='agb']").check();
    await page.locator("#register_submit").click();
    await page.waitForURL((url) => url.pathname === "/game/overview", { timeout: 20_000 });
    await page.locator(".legacy-overview-main-table").waitFor({ timeout: 20_000 });

    activationURL = await waitForActivationLink(email);
    activationOriginMatches = new URL(activationURL).origin === new URL(migratedBaseURL).origin;
    await page.goto(activationURL, { waitUntil: "domcontentloaded", timeout: 20_000 });
    await page.waitForURL((url) => url.pathname === "/game/overview", { timeout: 20_000 });
    await page.locator(".legacy-overview-main-table").waitFor({ timeout: 20_000 });
    overviewLoaded = true;
    finalURL = page.url();

    const activeURL = new URL(finalURL);
    await page.goto(`${activeURL.origin}/game/options${activeURL.search}`, { waitUntil: "domcontentloaded", timeout: 20_000 });
    await page.locator(".legacy-options-table").first().waitFor({ timeout: 20_000 });
    optionsLoaded = !(await page.locator("body").innerText()).includes("Unexpected token");
  } catch (caught) {
    error = caught instanceof Error ? caught.message : String(caught);
  } finally {
    await page.close();
  }

  return {
    pass:
      error === "" &&
      activationURL !== "" &&
      activationOriginMatches &&
      overviewLoaded &&
      optionsLoaded &&
      consoleErrors.length === 0 &&
      failedRequests.length === 0 &&
      badResponses.length === 0,
    username,
    email,
    activationURL,
    activationOriginMatches,
    finalURL,
    overviewLoaded,
    optionsLoaded,
    consoleErrors,
    failedRequests,
    badResponses,
    error
  };
}

async function waitForActivationLink(email: string): Promise<string> {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const response = await fetch(mailhogAPIURL);
    if (response.ok) {
      const payload = (await response.json()) as {
        items?: Array<{ To?: Array<{ Mailbox?: string; Domain?: string }>; Content?: { Body?: string } }>;
      };
      const message = payload.items?.find((item) =>
        item.To?.some((recipient) => `${recipient.Mailbox ?? ""}@${recipient.Domain ?? ""}`.toLowerCase() === email.toLowerCase())
      );
      const match = message?.Content?.Body?.match(/https?:\/\/[^\s]+\/game\/validate\.php\?ack=[^\s]+/);
      if (match?.[0]) {
        return match[0].replace(/&amp;/g, "&");
      }
    }
    await new Promise((resolvePromise) => setTimeout(resolvePromise, 250));
  }
  throw new Error(`Activation mail was not delivered for ${email}`);
}

async function newContext(baseURL: string): Promise<BrowserContext> {
  const context = await browser.newContext({
    viewport: { width: 1024, height: 768 },
    deviceScaleFactor: 1,
    locale: "en-US",
    reducedMotion: "reduce"
  });
  await context.addCookies([{ name: "ogamelang", value: "en", url: baseURL }]);
  return context;
}

async function runSide(context: BrowserContext, side: SideName): Promise<RegisterState> {
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

  const response = await page.goto(registerURL(side), { waitUntil: "domcontentloaded", timeout: 20_000 });
  await page.locator("form[name='registerForm']").waitFor({ timeout: 10_000 });
  const selectedUniverse = await waitForUniverse(page);

  const username = `Dyn${browserName}${Date.now().toString(36).slice(-5)}`.slice(0, 18);
  const states: RegisterState["states"] = {};
  await page.locator("input[name='email']").focus();
  await page.locator("input[name='character']").focus();
  await page.waitForTimeout(100);
  states.usernameHelp = await readRegisterState(page);

  await page.locator("input[name='character']").fill(username);
  await page.waitForTimeout(2200);
  states.usernamePollOK = await readRegisterState(page);

  await page.locator("input[name='email']").focus();
  await page.waitForTimeout(100);
  states.emailHelp = await readRegisterState(page);

  await page.locator("input[name='email']").fill(`dyn-${username.toLowerCase()}@example.local`);
  await page.waitForTimeout(2200);
  states.emailValidInputNoPoll = await readRegisterState(page);

  await page.locator("input[name='email']").fill("invalid-email");
  await page.waitForTimeout(2200);
  states.emailInvalidInputNoPoll = await readRegisterState(page);

  await page.locator("input[name='password']").focus();
  await page.waitForTimeout(100);
  states.passwordHelp = await readRegisterState(page);

  await page.locator("input[name='agb']").focus();
  await page.waitForTimeout(100);
  states.termsHelp = await readRegisterState(page);

  states.passwordErrorDirect = await captureURLState(page, side, {
    errorCode: "107",
    agb: "1",
    character: "DynErrorUser",
    email: "dyn-error@example.local",
    universe: selectedUniverse
  });
  for (const errorCode of ["101", "102", "103", "104", "108", "109"]) {
    states[`directError${errorCode}`] = await captureURLState(page, side, {
      errorCode,
      agb: "1",
      character: `DynDirect${errorCode}`,
      email: `dyn-direct-${errorCode}@example.local`,
      universe: selectedUniverse
    });
  }
  states.termsOnlyDirect = await captureURLState(page, side, {
    errorCode: "0",
    agb: "0",
    character: "DynTermsUser",
    email: "dyn-terms@example.local",
    universe: selectedUniverse
  });
  states.submitPasswordTermsError = await captureSubmitError(page, side);

  const currentURL = page.url();
  await page.close();
  return {
    url: currentURL,
    status: response?.status() ?? null,
    consoleErrors,
    failedRequests,
    badResponses,
    states
  };
}

async function captureURLState(page: Page, side: SideName, params: Record<string, string>) {
  await page.goto(registerURL(side, params), { waitUntil: "domcontentloaded", timeout: 20_000 });
  await page.locator("form[name='registerForm']").waitFor({ timeout: 10_000 });
  if (params.errorCode === "0" && params.agb === "0") {
    await page.locator("#infotext").filter({ hasText: "T&C" }).waitFor({ timeout: 10_000 });
  } else {
    const expected = legacyRegistrationMessage(params.errorCode ?? "");
    if (expected) {
      await page.locator("#statustext").filter({ hasText: expected }).waitFor({ timeout: 10_000 });
    }
  }
  return readRegisterState(page);
}

async function captureSubmitError(page: Page, side: SideName) {
  await page.goto(registerURL(side), { waitUntil: "domcontentloaded", timeout: 20_000 });
  await page.locator("form[name='registerForm']").waitFor({ timeout: 10_000 });
  await waitForUniverse(page);
  const suffix = `${browserName}${Date.now().toString(36).slice(-6)}`;
  await page.locator("input[name='character']").fill(`Err${suffix}`.slice(0, 18));
  await page.locator("input[name='email']").fill(`err-${suffix}@example.local`);
  await page.locator("input[name='password']").fill("short");
  const terms = page.locator("input[name='agb']");
  if (await terms.isChecked()) {
    await terms.click();
  }
  await page.locator("#register_submit").click();
  await page.locator("#infotext").filter({ hasText: "T&C" }).waitFor({ timeout: 15_000 });
  try {
    await page.locator("#statustext").filter({ hasText: "Password must be at least 8 characters long!" }).waitFor({ timeout: 15_000 });
  } catch (error) {
    const state = await readRegisterState(page);
    throw new Error(`${side} submit error state at ${page.url()}: ${JSON.stringify(state)}`, { cause: error });
  }
  return readRegisterState(page);
}

async function waitForUniverse(page: Page): Promise<string> {
  const selector = "form[name='registerForm'] select[name='universe']";
  await page.locator(selector).waitFor({ timeout: 10_000 });
  await page.waitForFunction(() => {
    const select = document.querySelector<HTMLSelectElement>("form[name='registerForm'] select[name='universe']");
    return Boolean(select?.value);
  }, null, { timeout: 10_000 });
  return page.locator(selector).inputValue({ timeout: 5_000 });
}

function registerURL(side: SideName, params?: Record<string, string>): string {
  const base = side === "legacy" ? `${legacyBaseURL}/register.php` : `${migratedBaseURL}/register`;
  if (!params) {
    return base;
  }
  const query = new URLSearchParams(params);
  return `${base}?${query.toString()}`;
}

async function readRegisterState(page: Page) {
  const statusSpan = page.locator("#statustext span").first();
  const statusClass = (await statusSpan.count()) > 0 ? (await statusSpan.getAttribute("class")) ?? "" : "";
  return {
    info: compact(await page.locator("#infotext").innerText({ timeout: 5_000 }).catch(() => "")),
    status: compact(await page.locator("#statustext").innerText({ timeout: 5_000 }).catch(() => "")),
    statusClass
  };
}

function compareStates(legacy: RegisterState, migrated: RegisterState): string[] {
  const errors: string[] = [];
  for (const key of Object.keys(legacy.states)) {
    const legacyState = legacy.states[key];
    const migratedState = migrated.states[key];
    if (!migratedState) {
      errors.push(`${key} missing on migrated`);
      continue;
    }
    for (const field of ["info", "status", "statusClass"] as const) {
      if (legacyState[field] !== migratedState[field]) {
        errors.push(`${key}.${field} differs: legacy=${legacyState[field]} migrated=${migratedState[field]}`);
      }
    }
  }
  return errors;
}

async function compareLegacyNewForm(): Promise<LegacyNewFormComparison> {
  const legacyContext = await newContext(legacyBaseURL);
  const migratedContext = await newContext(migratedBaseURL);
  try {
    const legacy = await captureLegacyNewForm(legacyContext, "legacy");
    const migrated = await captureLegacyNewForm(migratedContext, "migrated");
    const diffPath = join(screenshotDir, "legacy-new-form-diff.png");
    const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath, 0);
    const mismatches = compareLegacyNewFormState(legacy, migrated, diff);
    return {
      pass: mismatches.length === 0,
      diff,
      mismatches,
      legacy,
      migrated
    };
  } finally {
    await legacyContext.close();
    await migratedContext.close();
  }
}

async function captureLegacyNewForm(context: BrowserContext, side: SideName): Promise<LegacyNewFormState> {
  const page = await context.newPage();
  const response = await page.goto(`${side === "legacy" ? legacyBaseURL : migratedBaseURL}/game/reg/new.php`, {
    waitUntil: "networkidle",
    timeout: 20_000
  });
  await page.locator("form#registration").waitFor({ timeout: 10_000 });
  await waitForImages(page);
  await waitForStablePaint(page);
  await page.waitForTimeout(100);
  const screenshotPath = join(screenshotDir, `legacy-new-form-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false, animations: "disabled" });
  const state = await page.evaluate(() => {
    const compactText = (selector: string) => document.querySelector(selector)?.textContent?.replace(/\s+/g, " ").trim() ?? "";
    const form = document.querySelector<HTMLFormElement>("form#registration");
    return {
      url: window.location.href,
      title: compactText("h1"),
      playerInfoHeader: compactText("form#registration table table td.c"),
      playerNameLabel: compactText("input[name='character']") || compactText("form#registration table table tr:nth-of-type(2) th:first-child"),
      emailLabel: compactText("form#registration table table tr:nth-of-type(3) th:first-child"),
      acceptLabel: compactText("input[name='agb']") || compactText("form#registration table table tr:nth-of-type(4) th:first-child"),
      submitValue: document.querySelector<HTMLInputElement>("form#registration input[type='submit']")?.value ?? "",
      infoHeader: Array.from(document.querySelectorAll("td.c")).map((node) => node.textContent?.replace(/\s+/g, " ").trim() ?? "")[1] ?? "",
      formMethod: form?.method.toLowerCase() ?? "",
      formAction: form?.getAttribute("action") ?? "",
      inputNames: Array.from(document.querySelectorAll<HTMLInputElement>("form#registration input[name]")).map((input) => input.name),
      cssHrefs: Array.from(document.querySelectorAll<HTMLLinkElement>("link[rel='stylesheet']")).map((link) => new URL(link.href).pathname),
      legacyPublicChrome: document.body.classList.contains("legacy-public-body"),
      legacyGameChrome: document.body.classList.contains("legacy-game-body"),
      legacyRegistrationChrome: document.body.classList.contains("legacy-registration-body")
    };
  });
  await page.close();
  return {
    ...state,
    status: response?.status() ?? null,
    screenshotPath
  };
}

function compareLegacyNewFormState(legacy: LegacyNewFormState, migrated: LegacyNewFormState, diff: DiffResult): string[] {
  const errors: string[] = [];
  const comparableFields = [
    "title",
    "playerInfoHeader",
    "emailLabel",
    "acceptLabel",
    "submitValue",
    "infoHeader",
    "formMethod",
    "formAction"
  ] as const;
  if (legacy.status !== 200 || migrated.status !== 200) {
    errors.push(`legacy new.php status ${legacy.status}/${migrated.status}`);
  }
  for (const field of comparableFields) {
    if (legacy[field] !== migrated[field]) {
      errors.push(`legacy new.php ${field} differs: legacy=${legacy[field]} migrated=${migrated[field]}`);
    }
  }
  if (JSON.stringify(legacy.inputNames) !== JSON.stringify(migrated.inputNames)) {
    errors.push(`legacy new.php input names differ: legacy=${legacy.inputNames.join(",")} migrated=${migrated.inputNames.join(",")}`);
  }
  if (!migrated.cssHrefs.includes("/evolution/formate.css") || !migrated.cssHrefs.includes("/game/css/registration.css")) {
    errors.push(`legacy new.php migrated css missing: ${migrated.cssHrefs.join(",")}`);
  }
  if (migrated.legacyPublicChrome || migrated.legacyGameChrome || !migrated.legacyRegistrationChrome) {
    errors.push(
      `legacy new.php chrome flags public=${migrated.legacyPublicChrome} game=${migrated.legacyGameChrome} registration=${migrated.legacyRegistrationChrome}`
    );
  }
  if (diff.diffRatio > 0) {
    errors.push(`legacy new.php exact diff ${diff.diffRatio} (${diff.changedPixels}/${diff.totalPixels})`);
  }
  return errors;
}

function compact(value: string): string {
  return value.replace(/\s+/g, " ").trim();
}

function legacyRegistrationMessage(errorCode: string): string | null {
  switch (errorCode) {
    case "101":
      return "Player's name is already taken!";
    case "102":
      return "E-Mail-Address is already in use!";
    case "103":
      return "The name must be between 3 and 20 characters long!";
    case "104":
      return "You need to enter a valid e-mail-address!";
    case "107":
      return "Password must be at least 8 characters long!";
    case "108":
      return "Cannot register from same IP in next 10 minutes!";
    case "109":
      return "The maximum number of players has been reached!";
    default:
      return null;
  }
}

function renderMarkdown(report: {
  generatedAt: string;
  browserName: BrowserName;
  browserExecutable: string;
  pass: boolean;
  comparisons: string[];
  legacyNewForm: LegacyNewFormComparison;
  legacy: RegisterState;
  migrated: RegisterState;
}) {
  const lines = [
    "# Public Registration Dynamic Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Result: ${report.pass ? "PASS" : "FAIL"}`,
    "",
    "| State | Legacy | Migrated |",
    "| --- | --- | --- |"
  ];
  for (const key of Object.keys(report.legacy.states)) {
    lines.push(`| ${key} | ${stateSummary(report.legacy.states[key])} | ${stateSummary(report.migrated.states[key])} |`);
  }
  lines.push(
    "",
    "## Legacy /game/reg/new.php",
    "",
    `- Result: ${report.legacyNewForm.pass ? "PASS" : "FAIL"}`,
    `- Diff Ratio: ${report.legacyNewForm.diff.diffRatio}`,
    `- Changed Pixels: ${report.legacyNewForm.diff.changedPixels}`,
    `- Legacy title: ${report.legacyNewForm.legacy.title}`,
    `- Migrated title: ${report.legacyNewForm.migrated.title}`
  );
  if (report.legacyNewForm.mismatches.length > 0) {
    lines.push("", "### Legacy new.php Differences", "", ...report.legacyNewForm.mismatches.map((item) => `- ${item}`));
  }
  if (report.comparisons.length > 0) {
    lines.push("", "## Differences", "", ...report.comparisons.map((item) => `- ${item}`));
  }
  return `${lines.join("\n")}\n`;
}

function stateSummary(state: { info: string; status: string; statusClass: string } | undefined): string {
  if (!state) {
    return "missing";
  }
  return `${state.info} / ${state.status} / ${state.statusClass}`;
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name] ?? fallback;
  return raw === "firefox" ? "firefox" : "chromium";
}
