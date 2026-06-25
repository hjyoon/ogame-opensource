import { readFile } from "node:fs/promises";

import { publicRouteAliases, publicRoutes } from "../../frontend/src/routes.ts";

function envURL(value, fallback) {
  return Array.from(String(value ?? fallback)).join("").replace(/\/+$/, "");
}

const baseUrl = envURL(process.env.OGAME_GO_BASE_URL, "http://127.0.0.1:8890");
const mailhogBaseUrl = envURL(process.env.OGAME_MAILHOG_BASE_URL, "http://127.0.0.1:8026");
const loginSmokeUser = process.env.OGAME_GO_LOGIN_SMOKE_USER ?? "legor";
const loginSmokePassword = process.env.OGAME_GO_LOGIN_SMOKE_PASS ?? "admin";
const smokeFixtureFile = process.env.OGAME_GO_SMOKE_FIXTURE_FILE ?? "";

function check(pass, message, context = {}) {
  return { pass, message, context };
}

function finalize(testCase) {
  testCase.pass = testCase.checks.every((item) => item.pass === true);
  return testCase;
}

async function request(path, options = {}) {
  let response;
  try {
    response = await fetch(`${baseUrl}${path}`, {
      redirect: "manual",
      ...options
    });
  } catch (error) {
    throw new Error(`request failed for ${path}: ${error instanceof Error ? error.message : String(error)}`);
  }
  const headers = Object.fromEntries(response.headers.entries());
  const body = await response.text();
  return { status: response.status, headers, body };
}

function parseJSON(response) {
  try {
    return JSON.parse(response.body);
  } catch {
    return {};
  }
}

async function readOptionalJSON(path) {
  if (!path) {
    return {};
  }
  try {
    return JSON.parse(await readFile(path, "utf8"));
  } catch {
    return {};
  }
}

function pathFromURL(value) {
  try {
    const url = new URL(value, baseUrl);
    return `${url.pathname}${url.search}`;
  } catch {
    return "";
  }
}

function hasHeader(response, name, expected) {
  const actual = response.headers[name.toLowerCase()] ?? "";
  return expected === undefined ? actual !== "" : actual.toLowerCase().includes(expected.toLowerCase());
}

function withQueryParam(search, key, value) {
  const query = new URLSearchParams(search);
  query.set(key, String(value));
  return `?${query.toString()}`;
}

function withQueryParams(search, params) {
  const query = new URLSearchParams(search);
  for (const [key, value] of Object.entries(params)) {
    query.set(key, String(value));
  }
  return `?${query.toString()}`;
}

function noLoopbackAsset(body) {
  return !/(?:src|href|background)=["']https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?\//i.test(body);
}

async function mailhogRequest(path, options = {}) {
  try {
    const response = await fetch(`${mailhogBaseUrl}${path}`, options);
    return {
      ok: response.ok,
      status: response.status,
      body: await response.text()
    };
  } catch (error) {
    return {
      ok: false,
      status: 0,
      body: "",
      error: String(error)
    };
  }
}

async function clearMailhog() {
  let last = { ok: false, status: 0, body: "" };
  for (let index = 0; index < 20; index += 1) {
    last = await mailhogRequest("/api/v1/messages", { method: "DELETE" });
    if (last.ok) {
      return last;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  return last;
}

async function readMailhogMessages() {
  const response = await mailhogRequest("/api/v2/messages");
  let parsed = {};
  try {
    parsed = JSON.parse(response.body);
  } catch {
    parsed = {};
  }
  return {
    response,
    messages: Array.isArray(parsed.items) ? parsed.items : []
  };
}

function mailhogRecipients(message) {
  const rawTo = Array.isArray(message?.Raw?.To) ? message.Raw.To : [];
  const headerTo = Array.isArray(message?.Content?.Headers?.To) ? message.Content.Headers.To : [];
  return [...rawTo, ...headerTo].map((item) => String(item).toLowerCase());
}

function mailhogBody(message) {
  return String(message?.Content?.Body ?? "");
}

async function waitForMailhogMessage(email, needle) {
  let last = { response: { ok: false, status: 0, body: "" }, messages: [] };
  for (let index = 0; index < 20; index += 1) {
    last = await readMailhogMessages();
    const message = last.messages.find((item) => {
      const recipients = mailhogRecipients(item);
      return recipients.some((recipient) => recipient.includes(email.toLowerCase())) &&
        mailhogBody(item).toLowerCase().includes(needle.toLowerCase());
    });
    if (message) {
      return { ...last, message };
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  return { ...last, message: null };
}

const cases = [];

try {
  const runId = Date.now().toString(36);
  const smokeFixture = await readOptionalJSON(smokeFixtureFile);
  const phalanxFixture = smokeFixture?.phalanx ?? {};
  const health = await request("/api/healthz");
  let healthBody = {};
  try {
    healthBody = JSON.parse(health.body);
  } catch {
    healthBody = {};
  }
  cases.push(finalize({
    case: "go_health_endpoint",
    checks: [
      check(health.status === 200, "health endpoint returns HTTP 200", { status: health.status }),
      check(healthBody.status === "ok", "health endpoint reports ok status", healthBody),
      check(healthBody.goTarget === "1.25", "health endpoint reports Go 1.25 target", healthBody),
      check(healthBody.bunTarget === "1.3", "health endpoint reports Bun 1.3 target", healthBody),
      check(healthBody.reactTarget === "19", "health endpoint reports React 19 target", healthBody),
      check(healthBody.staticReady === true, "health endpoint sees React build output", healthBody),
      check(healthBody.legacyAssetsReady === true, "health endpoint sees legacy assets", healthBody),
      check(hasHeader(health, "content-type", "application/json"), "health endpoint returns JSON content type"),
      check(hasHeader(health, "x-frame-options", "SAMEORIGIN"), "health endpoint has frame protection"),
      check(hasHeader(health, "x-content-type-options", "nosniff"), "health endpoint has nosniff")
    ]
  }));

  const universeCatalog = await request("/api/public/universes");
  let universeCatalogBody = {};
  try {
    universeCatalogBody = JSON.parse(universeCatalog.body);
  } catch {
    universeCatalogBody = {};
  }
  const universes = Array.isArray(universeCatalogBody.universes) ? universeCatalogBody.universes : [];
  cases.push(finalize({
    case: "go_universe_catalog_api",
    checks: [
      check(universeCatalog.status === 200, "universe catalog returns HTTP 200", { status: universeCatalog.status }),
      check(hasHeader(universeCatalog, "content-type", "application/json"), "universe catalog returns JSON content type"),
      check(universes.length > 0, "universe catalog lists at least one universe", universeCatalogBody),
      check(universes[0]?.number === 1, "default universe keeps legacy universe number", universes[0] ?? {}),
      check(typeof universes[0]?.baseUrl === "string" && universes[0].baseUrl.length > 0, "universe exposes a base URL", universes[0] ?? {}),
      check(universes[0]?.open === true, "default universe is open", universes[0] ?? {})
    ]
  }));

  const validRegistration = await request("/api/public/registration/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: `Pilot${runId}`,
      password: "E2E_http123",
      email: `pilot-${runId}@example.local`,
      universe: universes[0]?.baseUrl ?? "http://localhost:8888",
      agb: true
    })
  });
  let validRegistrationBody = {};
  try {
    validRegistrationBody = JSON.parse(validRegistration.body);
  } catch {
    validRegistrationBody = {};
  }

  const invalidRegistration = await request("/api/public/registration/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: "ad",
      password: "short",
      email: "invalid",
      universe: "",
      agb: false
    })
  });
  let invalidRegistrationBody = {};
  try {
    invalidRegistrationBody = JSON.parse(invalidRegistration.body);
  } catch {
    invalidRegistrationBody = {};
  }
  const invalidIssues = Array.isArray(invalidRegistrationBody.issues) ? invalidRegistrationBody.issues : [];
  cases.push(finalize({
    case: "go_registration_validation_api",
    checks: [
      check(validRegistration.status === 200, "valid registration draft returns HTTP 200", { status: validRegistration.status }),
      check(hasHeader(validRegistration, "content-type", "application/json"), "valid registration draft returns JSON"),
      check(validRegistrationBody.valid === true, "valid registration draft is accepted", validRegistrationBody),
      check(!validRegistration.body.includes("E2E_http123"), "registration validation response does not echo password"),
      check(invalidRegistration.status === 200, "invalid registration draft returns HTTP 200", { status: invalidRegistration.status }),
      check(invalidRegistrationBody.valid === false, "invalid registration draft is rejected", invalidRegistrationBody),
      check(invalidIssues.some((issue) => issue.code === "character_invalid" && issue.legacyErrorCode === 103), "invalid name maps to legacy error 103", invalidRegistrationBody),
      check(invalidIssues.some((issue) => issue.code === "password_too_short" && issue.legacyErrorCode === 107), "short password maps to legacy error 107", invalidRegistrationBody),
      check(invalidIssues.some((issue) => issue.code === "email_invalid" && issue.legacyErrorCode === 104), "invalid email maps to legacy error 104", invalidRegistrationBody),
      check(invalidIssues.some((issue) => issue.code === "terms_required" && issue.legacyErrorCode === 204), "missing terms maps to legacy registration policy issue", invalidRegistrationBody)
    ]
  }));

  const registrationPassword = "E2E_http123";
  const registrationEmail = `new-pilot-${runId}@example.local`;
  const mailhogClear = await clearMailhog();
  const createdRegistration = await request("/api/public/registration", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: `NewPilot${runId}`,
      password: registrationPassword,
      email: registrationEmail,
      universe: universes[0]?.baseUrl ?? "http://localhost:8888",
      agb: true
    })
  });
  let createdRegistrationBody = {};
  try {
    createdRegistrationBody = JSON.parse(createdRegistration.body);
  } catch {
    createdRegistrationBody = {};
  }
  const createdRegistrationCookie = createdRegistration.headers["set-cookie"] ?? "";
  const createdRegistrationCookiePair = createdRegistrationCookie.split(";")[0] ?? "";
  let createdRegistrationSession = "";
  try {
    createdRegistrationSession = new URL(createdRegistrationBody.session?.redirectTo ?? "", baseUrl).searchParams.get("session") ?? "";
  } catch {
    createdRegistrationSession = "";
  }
  const createdRegistrationSearch = createdRegistrationSession
    ? `?session=${encodeURIComponent(createdRegistrationSession)}`
    : "";
  const createdOverview = createdRegistrationSession
    ? await request(`/api/game/overview${createdRegistrationSearch}`, {
      headers: { Cookie: createdRegistrationCookiePair }
    })
    : { status: 0, headers: {}, body: "" };
  let createdOverviewBody = {};
  try {
    createdOverviewBody = JSON.parse(createdOverview.body);
  } catch {
    createdOverviewBody = {};
  }
  const welcomeMail = await waitForMailhogMessage(registrationEmail, "activate your account");
  const welcomeMailBody = welcomeMail.message ? mailhogBody(welcomeMail.message) : "";
  const activationLinkPattern = /https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?\/game\/validate\.php\?ack=[a-f0-9]+/i;
  const welcomeActivationLink = welcomeMailBody.match(activationLinkPattern)?.[0] ?? "";
  const welcomeActivationPath = pathFromURL(welcomeActivationLink);
  const welcomeActivation = welcomeActivationPath
    ? await request(welcomeActivationPath)
    : { status: 0, headers: {}, body: "" };
  const welcomeActivationCookie = welcomeActivation.headers["set-cookie"] ?? "";
  const welcomeActivationCookiePair = welcomeActivationCookie.split(";")[0] ?? "";
  let welcomeActivationSession = "";
  try {
    welcomeActivationSession = new URL(welcomeActivation.headers.location ?? "", baseUrl).searchParams.get("session") ?? "";
  } catch {
    welcomeActivationSession = "";
  }
  const welcomeActivationSearch = welcomeActivationSession
    ? `?session=${encodeURIComponent(welcomeActivationSession)}`
    : "";
  const welcomeActivationOverview = welcomeActivationSession
    ? await request(`/api/game/overview${welcomeActivationSearch}`, {
      headers: { Cookie: welcomeActivationCookiePair }
    })
    : { status: 0, headers: {}, body: "" };
  let welcomeActivationOverviewBody = {};
  try {
    welcomeActivationOverviewBody = JSON.parse(welcomeActivationOverview.body);
  } catch {
    welcomeActivationOverviewBody = {};
  }
  const repeatedWelcomeActivation = welcomeActivationPath
    ? await request(welcomeActivationPath)
    : { status: 0, headers: {}, body: "" };
  cases.push(finalize({
    case: "go_registration_creation_api",
    checks: [
      check(mailhogClear.ok, "MailHog inbox can be cleared before registration", mailhogClear),
      check(createdRegistration.status === 200, "registration creation returns HTTP 200", { status: createdRegistration.status }),
      check(hasHeader(createdRegistration, "content-type", "application/json"), "registration creation returns JSON"),
      check(createdRegistrationBody.valid === true && createdRegistrationBody.created === true, "registration creation succeeds", createdRegistrationBody),
      check(Number.isInteger(createdRegistrationBody.account?.playerId) && createdRegistrationBody.account.playerId > 0, "registration returns the new player id", createdRegistrationBody.account ?? {}),
      check(Number.isInteger(createdRegistrationBody.account?.homePlanetId) && createdRegistrationBody.account.homePlanetId > 0, "registration creates a home planet", createdRegistrationBody.account ?? {}),
      check(typeof createdRegistrationBody.session?.redirectTo === "string" && createdRegistrationBody.session.redirectTo.includes("/game/overview"), "registration returns overview redirect", createdRegistrationBody.session ?? {}),
      check(createdRegistrationCookiePair.startsWith(`prsess_${createdRegistrationBody.account?.playerId ?? ""}_`), "registration sets private session cookie", { cookie: createdRegistrationCookiePair }),
      check(!createdRegistration.body.includes(registrationPassword), "registration creation response does not echo password"),
      check(!createdRegistration.body.includes("validatemd") && !createdRegistration.body.includes("activationCode"), "registration creation response does not expose activation code"),
      check(createdOverview.status === 200, "created registration session can read game overview", { status: createdOverview.status }),
      check(createdOverviewBody.authenticated === true, "created registration overview is authenticated", createdOverviewBody),
      check(createdOverviewBody.overview?.currentPlanet?.id === createdRegistrationBody.account?.homePlanetId, "created overview uses home planet", createdOverviewBody.overview?.currentPlanet ?? {}),
      check(welcomeMail.message !== null, "registration sends a welcome mail through MailHog", {
        mailhogStatus: welcomeMail.response.status,
        recipients: welcomeMail.message ? mailhogRecipients(welcomeMail.message) : []
      }),
      check(welcomeMailBody.includes("Click on this link to activate your account:"), "welcome mail contains legacy activation prompt"),
      check(welcomeMailBody.includes(`Password: ${registrationPassword}`), "welcome mail contains the registration password"),
      check(activationLinkPattern.test(welcomeMailBody), "welcome mail contains a legacy activation link", {
        match: welcomeActivationLink
      }),
      check(welcomeActivation.status === 302, "welcome activation link redirects after activation", {
        status: welcomeActivation.status,
        location: welcomeActivation.headers.location ?? ""
      }),
      check(typeof welcomeActivation.headers.location === "string" && welcomeActivation.headers.location.includes("/game/overview?"), "welcome activation redirects to overview", {
        location: welcomeActivation.headers.location ?? ""
      }),
      check(welcomeActivationCookiePair.startsWith(`prsess_${createdRegistrationBody.account?.playerId ?? ""}_`), "welcome activation sets a private session cookie", {
        cookie: welcomeActivationCookiePair
      }),
      check(welcomeActivationOverview.status === 200, "welcome activation session can read game overview", {
        status: welcomeActivationOverview.status
      }),
      check(welcomeActivationOverviewBody.authenticated === true, "welcome activation overview is authenticated", welcomeActivationOverviewBody),
      check(repeatedWelcomeActivation.status === 302 && repeatedWelcomeActivation.headers.location === "/home", "consumed activation link redirects home on reuse", {
        status: repeatedWelcomeActivation.status,
        location: repeatedWelcomeActivation.headers.location ?? ""
      })
    ]
  }));

  const validLogin = await request("/api/public/login/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      login: loginSmokeUser,
      pass: loginSmokePassword,
      universe: universes[0]?.baseUrl ?? "http://localhost:8888"
    })
  });
  let validLoginBody = {};
  try {
    validLoginBody = JSON.parse(validLogin.body);
  } catch {
    validLoginBody = {};
  }

  const wrongCredentialsLogin = await request("/api/public/login/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      login: loginSmokeUser,
      pass: `${loginSmokePassword}-wrong`,
      universe: universes[0]?.baseUrl ?? "http://localhost:8888"
    })
  });
  let wrongCredentialsLoginBody = {};
  try {
    wrongCredentialsLoginBody = JSON.parse(wrongCredentialsLogin.body);
  } catch {
    wrongCredentialsLoginBody = {};
  }

  const sessionLogin = await request("/api/public/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      login: loginSmokeUser,
      pass: loginSmokePassword,
      universe: universes[0]?.baseUrl ?? "http://localhost:8888"
    })
  });
  let sessionLoginBody = {};
  try {
    sessionLoginBody = JSON.parse(sessionLogin.body);
  } catch {
    sessionLoginBody = {};
  }
  const sessionCookie = sessionLogin.headers["set-cookie"] ?? "";
  const sessionCookiePair = sessionCookie.split(";")[0] ?? "";
  const sessionCookieName = sessionCookiePair.split("=")[0] ?? "";
  const loginPlayerId = Number(/^prsess_(\d+)_/.exec(sessionCookieName)?.[1] ?? 0);
  const sessionSearch = typeof sessionLoginBody.session?.redirectTo === "string"
    ? new URL(sessionLoginBody.session.redirectTo, baseUrl).search
    : "?session=";
  const gameSession = await request(`/api/game/session${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameSessionBody = {};
  try {
    gameSessionBody = JSON.parse(gameSession.body);
  } catch {
    gameSessionBody = {};
  }

  const gameSessionWithoutCookie = await request(`/api/game/session${sessionSearch}`);
  let gameSessionWithoutCookieBody = {};
  try {
    gameSessionWithoutCookieBody = JSON.parse(gameSessionWithoutCookie.body);
  } catch {
    gameSessionWithoutCookieBody = {};
  }

  const gameOverview = await request(`/api/game/overview${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameOverviewBody = {};
  try {
    gameOverviewBody = JSON.parse(gameOverview.body);
  } catch {
    gameOverviewBody = {};
  }

  const gameOverviewWithoutCookie = await request(`/api/game/overview${sessionSearch}`);
  let gameOverviewWithoutCookieBody = {};
  try {
    gameOverviewWithoutCookieBody = JSON.parse(gameOverviewWithoutCookie.body);
  } catch {
    gameOverviewWithoutCookieBody = {};
  }
  const planetSwitcher = Array.isArray(gameOverviewBody.overview?.planetSwitcher) ? gameOverviewBody.overview.planetSwitcher : [];
  const currentPlanetID = gameOverviewBody.overview?.currentPlanet?.id;
  const basePlanetID = planetSwitcher.find((planet) => planet.type === 1)?.id ?? currentPlanetID;
  const switchPlanetID = planetSwitcher.find((planet) => planet.id !== basePlanetID)?.id ?? basePlanetID;
  const switchedSearch = switchPlanetID ? withQueryParam(sessionSearch, "cp", switchPlanetID) : sessionSearch;
  const gameOverviewSwitched = await request(`/api/game/overview${switchedSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameOverviewSwitchedBody = {};
  try {
    gameOverviewSwitchedBody = JSON.parse(gameOverviewSwitched.body);
  } catch {
    gameOverviewSwitchedBody = {};
  }
  const gameOverviewAfterSwitch = await request(`/api/game/overview${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameOverviewAfterSwitchBody = {};
  try {
    gameOverviewAfterSwitchBody = JSON.parse(gameOverviewAfterSwitch.body);
  } catch {
    gameOverviewAfterSwitchBody = {};
  }
  const restoreSearch = basePlanetID ? withQueryParam(sessionSearch, "cp", basePlanetID) : sessionSearch;
  const gameOverviewRestored = await request(`/api/game/overview${restoreSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameOverviewRestoredBody = {};
  try {
    gameOverviewRestoredBody = JSON.parse(gameOverviewRestored.body);
  } catch {
    gameOverviewRestoredBody = {};
  }
  const originalPlanetName = gameOverviewRestoredBody.overview?.currentPlanet?.name ?? "";
  const renamedPlanetName = `Smoke ${runId.slice(0, 8)}`.slice(0, 20);
  const gameOverviewRenamed = await request(`/api/game/overview${restoreSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
    body: JSON.stringify({ action: "rename", name: renamedPlanetName })
  });
  let gameOverviewRenamedBody = {};
  try {
    gameOverviewRenamedBody = JSON.parse(gameOverviewRenamed.body);
  } catch {
    gameOverviewRenamedBody = {};
  }
  const gameOverviewRenameForbidden = await request(`/api/game/overview${restoreSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
    body: JSON.stringify({ action: "rename", name: "bad;name" })
  });
  let gameOverviewRenameForbiddenBody = {};
  try {
    gameOverviewRenameForbiddenBody = JSON.parse(gameOverviewRenameForbidden.body);
  } catch {
    gameOverviewRenameForbiddenBody = {};
  }
  const gameOverviewRenameRestored = originalPlanetName
    ? await request(`/api/game/overview${restoreSearch}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
      body: JSON.stringify({ action: "rename", name: originalPlanetName })
    })
    : { status: 0, headers: {}, body: "" };
  let gameOverviewRenameRestoredBody = {};
  try {
    gameOverviewRenameRestoredBody = JSON.parse(gameOverviewRenameRestored.body);
  } catch {
    gameOverviewRenameRestoredBody = {};
  }
  const gameOverviewDeleteWrongPassword = await request(`/api/game/overview${restoreSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
    body: JSON.stringify({ action: "delete", deleteId: basePlanetID, password: `${loginSmokePassword}-wrong` })
  });
  let gameOverviewDeleteWrongPasswordBody = {};
  try {
    gameOverviewDeleteWrongPasswordBody = JSON.parse(gameOverviewDeleteWrongPassword.body);
  } catch {
    gameOverviewDeleteWrongPasswordBody = {};
  }
  const gameOverviewDeleteHome = await request(`/api/game/overview${restoreSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
    body: JSON.stringify({ action: "delete", deleteId: basePlanetID, password: loginSmokePassword })
  });
  let gameOverviewDeleteHomeBody = {};
  try {
    gameOverviewDeleteHomeBody = JSON.parse(gameOverviewDeleteHome.body);
  } catch {
    gameOverviewDeleteHomeBody = {};
  }
  const missingPlanetSearch = withQueryParam(sessionSearch, "cp", "987654321");
  const gameOverviewMissingPlanet = await request(`/api/game/overview${missingPlanetSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameOverviewMissingPlanetBody = {};
  try {
    gameOverviewMissingPlanetBody = JSON.parse(gameOverviewMissingPlanet.body);
  } catch {
    gameOverviewMissingPlanetBody = {};
  }
  const gameOverviewAfterMissingPlanet = await request(`/api/game/overview${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameOverviewAfterMissingPlanetBody = {};
  try {
    gameOverviewAfterMissingPlanetBody = JSON.parse(gameOverviewAfterMissingPlanet.body);
  } catch {
    gameOverviewAfterMissingPlanetBody = {};
  }

  const gameBuildings = await request(`/api/game/buildings${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameBuildingsBody = {};
  try {
    gameBuildingsBody = JSON.parse(gameBuildings.body);
  } catch {
    gameBuildingsBody = {};
  }

  const gameBuildingsWithoutCookie = await request(`/api/game/buildings${sessionSearch}`);
  let gameBuildingsWithoutCookieBody = {};
  try {
    gameBuildingsWithoutCookieBody = JSON.parse(gameBuildingsWithoutCookie.body);
  } catch {
    gameBuildingsWithoutCookieBody = {};
  }

  const gameBuildingsMutation = await request(`/api/game/buildings${sessionSearch}`, {
    method: "POST",
    headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
    body: JSON.stringify({ action: "remove", listId: 0 })
  });
  let gameBuildingsMutationBody = {};
  try {
    gameBuildingsMutationBody = JSON.parse(gameBuildingsMutation.body);
  } catch {
    gameBuildingsMutationBody = {};
  }

  const gameBuildingsDemolishMutation = await request(`/api/game/buildings${welcomeActivationSearch}`, {
    method: "POST",
    headers: { Cookie: welcomeActivationCookiePair, "Content-Type": "application/json" },
    body: JSON.stringify({ action: "destroy", techId: 33 })
  });
  let gameBuildingsDemolishMutationBody = {};
  try {
    gameBuildingsDemolishMutationBody = JSON.parse(gameBuildingsDemolishMutation.body);
  } catch {
    gameBuildingsDemolishMutationBody = {};
  }

  const gameResearch = await request(`/api/game/research${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameResearchBody = {};
  try {
    gameResearchBody = JSON.parse(gameResearch.body);
  } catch {
    gameResearchBody = {};
  }

  const gameResearchWithoutCookie = await request(`/api/game/research${sessionSearch}`);
  let gameResearchWithoutCookieBody = {};
  try {
    gameResearchWithoutCookieBody = JSON.parse(gameResearchWithoutCookie.body);
  } catch {
    gameResearchWithoutCookieBody = {};
  }

  const gameShipyard = await request(`/api/game/shipyard${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameShipyardBody = {};
  try {
    gameShipyardBody = JSON.parse(gameShipyard.body);
  } catch {
    gameShipyardBody = {};
  }

  const gameShipyardWithoutCookie = await request(`/api/game/shipyard${sessionSearch}`);
  let gameShipyardWithoutCookieBody = {};
  try {
    gameShipyardWithoutCookieBody = JSON.parse(gameShipyardWithoutCookie.body);
  } catch {
    gameShipyardWithoutCookieBody = {};
  }

  const gameFleet = await request(`/api/game/fleet${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameFleetBody = {};
  try {
    gameFleetBody = JSON.parse(gameFleet.body);
  } catch {
    gameFleetBody = {};
  }
  const selectableFleetShip = Array.isArray(gameFleetBody.fleet?.ships)
    ? gameFleetBody.fleet.ships.find((ship) => ship?.selectable === true && Number(ship?.count) > 0)
    : null;
  const fleetTarget = gameFleetBody.fleet?.currentPlanet?.coordinates ?? gameOverviewBody.overview?.currentPlanet?.coordinates ?? { galaxy: 1, system: 1, position: 1 };
  const gameFleetPrepare = selectableFleetShip
    ? await request(`/api/game/fleet${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "prepare",
          ships: { [String(selectableFleetShip.id)]: Number(selectableFleetShip.count) + 1000 },
          target: fleetTarget,
          targetType: 1,
          mission: 3,
          speed: 9
        })
      })
    : { status: 0, body: "", headers: {} };
  let gameFleetPrepareBody = {};
  try {
    gameFleetPrepareBody = JSON.parse(gameFleetPrepare.body);
  } catch {
    gameFleetPrepareBody = {};
  }
  const fleetCurrentType = gameFleetBody.fleet?.currentPlanet?.type === 0 ? 3 : 1;
  const gameFleetValidate = selectableFleetShip
    ? await request(`/api/game/fleet${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "validate-dispatch",
          ships: { [String(selectableFleetShip.id)]: 1 },
          resources: { 700: 1 },
          target: fleetTarget,
          targetType: fleetCurrentType,
          mission: 3,
          speed: 9
        })
      })
    : { status: 0, body: "", headers: {} };
  let gameFleetValidateBody = {};
  try {
    gameFleetValidateBody = JSON.parse(gameFleetValidate.body);
  } catch {
    gameFleetValidateBody = {};
  }
  const gameFleetLaunch = selectableFleetShip
    ? await request(`/api/game/fleet${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "launch-dispatch",
          ships: { [String(selectableFleetShip.id)]: 1 },
          resources: { 700: 1 },
          target: fleetTarget,
          targetType: fleetCurrentType,
          mission: 3,
          speed: 9
        })
      })
    : { status: 0, body: "", headers: {} };
  let gameFleetLaunchBody = {};
  try {
    gameFleetLaunchBody = JSON.parse(gameFleetLaunch.body);
  } catch {
    gameFleetLaunchBody = {};
  }
  const alternateFleetTarget = {
    galaxy: fleetTarget.galaxy ?? 1,
    system: fleetTarget.system ?? 1,
    position: Number(fleetTarget.position ?? 1) >= 15 ? 14 : Number(fleetTarget.position ?? 1) + 1
  };
  const gameFleetNoShips = selectableFleetShip
    ? await request(`/api/game/fleet${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "validate-dispatch",
          ships: {},
          resources: {},
          target: alternateFleetTarget,
          targetType: 1,
          mission: 3,
          speed: 9
        })
      })
    : { status: 0, body: "", headers: {} };
  const gameFleetNoShipsBody = parseJSON(gameFleetNoShips);
  const gameFleetInvalidOrder = selectableFleetShip
    ? await request(`/api/game/fleet${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "validate-dispatch",
          ships: { [String(selectableFleetShip.id)]: 1 },
          resources: {},
          target: alternateFleetTarget,
          targetType: 1,
          mission: 999,
          speed: 9
        })
      })
    : { status: 0, body: "", headers: {} };
  const gameFleetInvalidOrderBody = parseJSON(gameFleetInvalidOrder);
  const gameFleetInvalidExpeditionTarget = selectableFleetShip
    ? await request(`/api/game/fleet${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "validate-dispatch",
          ships: { [String(selectableFleetShip.id)]: 1 },
          resources: {},
          target: {
            galaxy: fleetTarget.galaxy ?? 1,
            system: fleetTarget.system ?? 1,
            position: 16
          },
          targetType: 2,
          mission: 15,
          speed: 9,
          expeditionHours: 1
        })
      })
    : { status: 0, body: "", headers: {} };
  const gameFleetInvalidExpeditionTargetBody = parseJSON(gameFleetInvalidExpeditionTarget);

  const gameFleetWithoutCookie = await request(`/api/game/fleet${sessionSearch}`);
  let gameFleetWithoutCookieBody = {};
  try {
    gameFleetWithoutCookieBody = JSON.parse(gameFleetWithoutCookie.body);
  } catch {
    gameFleetWithoutCookieBody = {};
  }

  const gameFleetTemplates = await request(`/api/game/fleet-templates${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameFleetTemplatesBody = {};
  try {
    gameFleetTemplatesBody = JSON.parse(gameFleetTemplates.body);
  } catch {
    gameFleetTemplatesBody = {};
  }

  const gameFleetTemplatesWithoutCookie = await request(`/api/game/fleet-templates${sessionSearch}`);
  let gameFleetTemplatesWithoutCookieBody = {};
  try {
    gameFleetTemplatesWithoutCookieBody = JSON.parse(gameFleetTemplatesWithoutCookie.body);
  } catch {
    gameFleetTemplatesWithoutCookieBody = {};
  }

  const gameGalaxy = await request(`/api/game/galaxy${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameGalaxyBody = {};
  try {
    gameGalaxyBody = JSON.parse(gameGalaxy.body);
  } catch {
    gameGalaxyBody = {};
  }
  const galaxyCoordinates = gameGalaxyBody.galaxy?.coordinates ?? { galaxy: 1, system: 1, position: 1 };
  const gameGalaxySpyDispatch = await request(`/api/game/galaxy${sessionSearch}`, {
    method: "POST",
    headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
    body: JSON.stringify({
      action: "dispatch-spy",
      targetGalaxy: galaxyCoordinates.galaxy,
      targetSystem: galaxyCoordinates.system,
      targetPosition: Math.min(15, Number(galaxyCoordinates.position ?? 1) + 1),
      targetType: 1,
      amount: 0
    })
  });
  let gameGalaxySpyDispatchBody = {};
  try {
    gameGalaxySpyDispatchBody = JSON.parse(gameGalaxySpyDispatch.body);
  } catch {
    gameGalaxySpyDispatchBody = {};
  }
  const gameGalaxyRecycleDispatch = await request(`/api/game/galaxy${sessionSearch}`, {
    method: "POST",
    headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
    body: JSON.stringify({
      action: "dispatch-recycle",
      targetGalaxy: galaxyCoordinates.galaxy,
      targetSystem: galaxyCoordinates.system,
      targetPosition: Math.min(15, Number(galaxyCoordinates.position ?? 1) + 1),
      targetType: 2,
      amount: 0
    })
  });
  let gameGalaxyRecycleDispatchBody = {};
  try {
    gameGalaxyRecycleDispatchBody = JSON.parse(gameGalaxyRecycleDispatch.body);
  } catch {
    gameGalaxyRecycleDispatchBody = {};
  }

  const gameGalaxyWithoutCookie = await request(`/api/game/galaxy${sessionSearch}`);
  let gameGalaxyWithoutCookieBody = {};
  try {
    gameGalaxyWithoutCookieBody = JSON.parse(gameGalaxyWithoutCookie.body);
  } catch {
    gameGalaxyWithoutCookieBody = {};
  }

  const gameDefense = await request(`/api/game/defense${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameDefenseBody = {};
  try {
    gameDefenseBody = JSON.parse(gameDefense.body);
  } catch {
    gameDefenseBody = {};
  }

  const gameDefenseWithoutCookie = await request(`/api/game/defense${sessionSearch}`);
  let gameDefenseWithoutCookieBody = {};
  try {
    gameDefenseWithoutCookieBody = JSON.parse(gameDefenseWithoutCookie.body);
  } catch {
    gameDefenseWithoutCookieBody = {};
  }

  const gameEmpire = await request(`/api/game/empire${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameEmpireBody = {};
  try {
    gameEmpireBody = JSON.parse(gameEmpire.body);
  } catch {
    gameEmpireBody = {};
  }

  const gameEmpireMoons = await request(`/api/game/empire${sessionSearch}&planettype=3`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameEmpireMoonsBody = {};
  try {
    gameEmpireMoonsBody = JSON.parse(gameEmpireMoons.body);
  } catch {
    gameEmpireMoonsBody = {};
  }

  const gameEmpireInvalidShortcut = await request(`/api/game/empire${sessionSearch}&modus=add&planet=${basePlanetID ?? 0}&techid=999999`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameEmpireInvalidShortcutBody = {};
  try {
    gameEmpireInvalidShortcutBody = JSON.parse(gameEmpireInvalidShortcut.body);
  } catch {
    gameEmpireInvalidShortcutBody = {};
  }

  const gameEmpireWithoutCookie = await request(`/api/game/empire${sessionSearch}`);
  let gameEmpireWithoutCookieBody = {};
  try {
    gameEmpireWithoutCookieBody = JSON.parse(gameEmpireWithoutCookie.body);
  } catch {
    gameEmpireWithoutCookieBody = {};
  }

  const gameTechnology = await request(`/api/game/technology${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameTechnologyBody = {};
  try {
    gameTechnologyBody = JSON.parse(gameTechnology.body);
  } catch {
    gameTechnologyBody = {};
  }

  const gameTechnologyDetails = await request(`/api/game/technology${sessionSearch}&tid=206`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameTechnologyDetailsBody = {};
  try {
    gameTechnologyDetailsBody = JSON.parse(gameTechnologyDetails.body);
  } catch {
    gameTechnologyDetailsBody = {};
  }

  const gameTechnologyWithoutCookie = await request(`/api/game/technology${sessionSearch}`);
  let gameTechnologyWithoutCookieBody = {};
  try {
    gameTechnologyWithoutCookieBody = JSON.parse(gameTechnologyWithoutCookie.body);
  } catch {
    gameTechnologyWithoutCookieBody = {};
  }

  const gameStatistics = await request(`/api/game/statistics${sessionSearch}&type=ressources&start=1`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameStatisticsBody = {};
  try {
    gameStatisticsBody = JSON.parse(gameStatistics.body);
  } catch {
    gameStatisticsBody = {};
  }

  const gameFleetStatistics = await request(`/api/game/statistics${sessionSearch}&type=fleet&start=1`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameFleetStatisticsBody = {};
  try {
    gameFleetStatisticsBody = JSON.parse(gameFleetStatistics.body);
  } catch {
    gameFleetStatisticsBody = {};
  }

  const gameResearchStatistics = await request(`/api/game/statistics${sessionSearch}&type=research&start=1`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameResearchStatisticsBody = {};
  try {
    gameResearchStatisticsBody = JSON.parse(gameResearchStatistics.body);
  } catch {
    gameResearchStatisticsBody = {};
  }

  const gameAllianceStatistics = await request(`/api/game/statistics${sessionSearch}&who=ally&type=ressources&start=1`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameAllianceStatisticsBody = {};
  try {
    gameAllianceStatisticsBody = JSON.parse(gameAllianceStatistics.body);
  } catch {
    gameAllianceStatisticsBody = {};
  }

  const gameStatisticsWithoutCookie = await request(`/api/game/statistics${sessionSearch}`);
  let gameStatisticsWithoutCookieBody = {};
  try {
    gameStatisticsWithoutCookieBody = JSON.parse(gameStatisticsWithoutCookie.body);
  } catch {
    gameStatisticsWithoutCookieBody = {};
  }

  const gameSearch = await request(`/api/game/search${sessionSearch}&type=playername&searchtext=leg`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameSearchBody = {};
  try {
    gameSearchBody = JSON.parse(gameSearch.body);
  } catch {
    gameSearchBody = {};
  }

  const gameAllianceSearch = await request(`/api/game/search${sessionSearch}&type=allytag&searchtext=TA`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameAllianceSearchBody = {};
  try {
    gameAllianceSearchBody = JSON.parse(gameAllianceSearch.body);
  } catch {
    gameAllianceSearchBody = {};
  }

  const gameSearchWithoutCookie = await request(`/api/game/search${sessionSearch}`);
  let gameSearchWithoutCookieBody = {};
  try {
    gameSearchWithoutCookieBody = JSON.parse(gameSearchWithoutCookie.body);
  } catch {
    gameSearchWithoutCookieBody = {};
  }

  const gameBuddy = await request(`/api/game/buddy${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameBuddyBody = {};
  try {
    gameBuddyBody = JSON.parse(gameBuddy.body);
  } catch {
    gameBuddyBody = {};
  }

  const gameBuddyRequest = await request(`/api/game/buddy${sessionSearch}&action=7&buddy_id=999999`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameBuddyRequestBody = {};
  try {
    gameBuddyRequestBody = JSON.parse(gameBuddyRequest.body);
  } catch {
    gameBuddyRequestBody = {};
  }

  const gameBuddyMutation = await request(`/api/game/buddy${sessionSearch}`, {
    method: "POST",
    headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
    body: JSON.stringify({ action: 8, buddyId: 0 })
  });
  let gameBuddyMutationBody = {};
  try {
    gameBuddyMutationBody = JSON.parse(gameBuddyMutation.body);
  } catch {
    gameBuddyMutationBody = {};
  }

  const gameBuddyWithoutCookie = await request(`/api/game/buddy${sessionSearch}`);
  let gameBuddyWithoutCookieBody = {};
  try {
    gameBuddyWithoutCookieBody = JSON.parse(gameBuddyWithoutCookie.body);
  } catch {
    gameBuddyWithoutCookieBody = {};
  }

  const gameMessages = await request(`/api/game/messages${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameMessagesBody = {};
  try {
    gameMessagesBody = JSON.parse(gameMessages.body);
  } catch {
    gameMessagesBody = {};
  }

  const gameMessagesCompose = loginPlayerId > 0
    ? await request(`/api/game/messages${sessionSearch}&messageziel=${loginPlayerId}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  let gameMessagesComposeBody = {};
  try {
    gameMessagesComposeBody = JSON.parse(gameMessagesCompose.body);
  } catch {
    gameMessagesComposeBody = {};
  }

  const gameMessagesSend = loginPlayerId > 0
    ? await request(`/api/game/messages${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "send",
          targetPlayerId: loginPlayerId,
          subject: "Go smoke PM",
          text: "Go migration message smoke"
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  let gameMessagesSendBody = {};
  try {
    gameMessagesSendBody = JSON.parse(gameMessagesSend.body);
  } catch {
    gameMessagesSendBody = {};
  }

  const gameMessagesAfterSend = await request(`/api/game/messages${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameMessagesAfterSendBody = {};
  try {
    gameMessagesAfterSendBody = JSON.parse(gameMessagesAfterSend.body);
  } catch {
    gameMessagesAfterSendBody = {};
  }
  const sentMessageRow = Array.isArray(gameMessagesAfterSendBody.messages?.rows)
    ? gameMessagesAfterSendBody.messages.rows.find((row) => String(row.subject ?? "").includes("Go smoke PM") || String(row.text ?? "").includes("Go migration message smoke"))
    : null;
  const sentReportID = Number(sentMessageRow?.id ?? 0);
  const gameReport = sentReportID > 0
    ? await request(`/api/game/report${sessionSearch}&bericht=${sentReportID}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  let gameReportBody = {};
  try {
    gameReportBody = JSON.parse(gameReport.body);
  } catch {
    gameReportBody = {};
  }
  const gameReportWithoutCookie = sentReportID > 0
    ? await request(`/api/game/report${sessionSearch}&bericht=${sentReportID}`)
    : { status: 0, headers: {}, body: "{}" };
  let gameReportWithoutCookieBody = {};
  try {
    gameReportWithoutCookieBody = JSON.parse(gameReportWithoutCookie.body);
  } catch {
    gameReportWithoutCookieBody = {};
  }

  const gameMessagesWithoutCookie = await request(`/api/game/messages${sessionSearch}`);
  let gameMessagesWithoutCookieBody = {};
  try {
    gameMessagesWithoutCookieBody = JSON.parse(gameMessagesWithoutCookie.body);
  } catch {
    gameMessagesWithoutCookieBody = {};
  }

  const gameNotes = await request(`/api/game/notes${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameNotesBody = {};
  try {
    gameNotesBody = JSON.parse(gameNotes.body);
  } catch {
    gameNotesBody = {};
  }

  const gameNotesCreate = await request(`/api/game/notes${sessionSearch}&a=1`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameNotesCreateBody = {};
  try {
    gameNotesCreateBody = JSON.parse(gameNotesCreate.body);
  } catch {
    gameNotesCreateBody = {};
  }

  const gameNotesWithoutCookie = await request(`/api/game/notes${sessionSearch}`);
  let gameNotesWithoutCookieBody = {};
  try {
    gameNotesWithoutCookieBody = JSON.parse(gameNotesWithoutCookie.body);
  } catch {
    gameNotesWithoutCookieBody = {};
  }

  const noteSubject = `smoke-note-${runId}`;
  const updatedNoteSubject = `${noteSubject}-updated`;
  const gameNotesCreatePost = await request(`/api/game/notes${sessionSearch}`, {
    method: "POST",
    headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
    body: JSON.stringify({ action: "create", subject: noteSubject, text: "smoke body", priority: 2 })
  });
  let gameNotesCreatePostBody = {};
  try {
    gameNotesCreatePostBody = JSON.parse(gameNotesCreatePost.body);
  } catch {
    gameNotesCreatePostBody = {};
  }
  const createdNote = Array.isArray(gameNotesCreatePostBody.notes?.rows)
    ? gameNotesCreatePostBody.notes.rows.find((row) => row.subject === noteSubject)
    : null;

  const gameNotesUpdatePost = createdNote
    ? await request(`/api/game/notes${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({ action: "update", noteId: createdNote.id, subject: updatedNoteSubject, text: "updated body", priority: 0 })
      })
    : { status: 0, headers: {}, body: "{}" };
  let gameNotesUpdatePostBody = {};
  try {
    gameNotesUpdatePostBody = JSON.parse(gameNotesUpdatePost.body);
  } catch {
    gameNotesUpdatePostBody = {};
  }
  const updatedNote = Array.isArray(gameNotesUpdatePostBody.notes?.rows)
    ? gameNotesUpdatePostBody.notes.rows.find((row) => row.subject === updatedNoteSubject)
    : null;

  const gameNotesDeletePost = updatedNote
    ? await request(`/api/game/notes${sessionSearch}`, {
        method: "POST",
        headers: { Cookie: sessionCookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({ action: "delete", noteIds: [updatedNote.id] })
      })
    : { status: 0, headers: {}, body: "{}" };
  let gameNotesDeletePostBody = {};
  try {
    gameNotesDeletePostBody = JSON.parse(gameNotesDeletePost.body);
  } catch {
    gameNotesDeletePostBody = {};
  }

  const gameResources = await request(`/api/game/resources${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameResourcesBody = {};
  try {
    gameResourcesBody = JSON.parse(gameResources.body);
  } catch {
    gameResourcesBody = {};
  }

  const gameResourcesUpdate = await request(`/api/game/resources${sessionSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
    body: JSON.stringify({
      production: {
        1: -250,
        2: "not-a-number",
        3: 35,
        4: 100,
        12: 70,
        212: 10
      }
    })
  });
  let gameResourcesUpdateBody = {};
  try {
    gameResourcesUpdateBody = JSON.parse(gameResourcesUpdate.body);
  } catch {
    gameResourcesUpdateBody = {};
  }

  const gameResourcesWithoutCookie = await request(`/api/game/resources${sessionSearch}`);
  let gameResourcesWithoutCookieBody = {};
  try {
    gameResourcesWithoutCookieBody = JSON.parse(gameResourcesWithoutCookie.body);
  } catch {
    gameResourcesWithoutCookieBody = {};
  }

  const gameMerchant = await request(`/api/game/merchant${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameMerchantBody = {};
  try {
    gameMerchantBody = JSON.parse(gameMerchant.body);
  } catch {
    gameMerchantBody = {};
  }

  const gameMerchantWithoutCookie = await request(`/api/game/merchant${sessionSearch}`);
  let gameMerchantWithoutCookieBody = {};
  try {
    gameMerchantWithoutCookieBody = JSON.parse(gameMerchantWithoutCookie.body);
  } catch {
    gameMerchantWithoutCookieBody = {};
  }

  const gameOfficers = await request(`/api/game/officers${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameOfficersBody = {};
  try {
    gameOfficersBody = JSON.parse(gameOfficers.body);
  } catch {
    gameOfficersBody = {};
  }

  const gameOfficersInvalid = await request(`/api/game/officers${sessionSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", Cookie: sessionCookiePair },
    body: "type=99&days=7"
  });
  let gameOfficersInvalidBody = {};
  try {
    gameOfficersInvalidBody = JSON.parse(gameOfficersInvalid.body);
  } catch {
    gameOfficersInvalidBody = {};
  }

  const gameOfficersWithoutCookie = await request(`/api/game/officers${sessionSearch}`);
  let gameOfficersWithoutCookieBody = {};
  try {
    gameOfficersWithoutCookieBody = JSON.parse(gameOfficersWithoutCookie.body);
  } catch {
    gameOfficersWithoutCookieBody = {};
  }

  const gameAdmin = await request(`/api/game/admin${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameAdminBody = {};
  try {
    gameAdminBody = JSON.parse(gameAdmin.body);
  } catch {
    gameAdminBody = {};
  }

  const gameAdminWithoutCookie = await request(`/api/game/admin${sessionSearch}`);
  let gameAdminWithoutCookieBody = {};
  try {
    gameAdminWithoutCookieBody = JSON.parse(gameAdminWithoutCookie.body);
  } catch {
    gameAdminWithoutCookieBody = {};
  }
  const adminSubmodeSpecs = [
    { name: "Users", mode: "Users", arrayKey: "userRows" },
    { name: "Planets", mode: "Planets", arrayKey: "planetRows" },
    { name: "Queue", mode: "Queue", arrayKey: "queueRows" },
    { name: "Fleetlogs", mode: "Fleetlogs", arrayKey: "fleetLogRows" },
    { name: "BattleReport", mode: "BattleReport", arrayKey: "battleReports" },
    { name: "Checksum", mode: "Checksum", arrayKey: "checksumGroups" },
    { name: "DB", mode: "DB", arrayKey: "databaseBackups" },
    { name: "BotEdit", mode: "BotEdit", arrayKey: "botStrategies" },
    { name: "Uni", mode: "Uni", objectKey: "universe" },
    { name: "Expedition", mode: "Expedition", objectKey: "expedition" },
    { name: "Unknown", mode: "DefinitelyNotALegacyMode", expectedMode: "Home" }
  ];
  const gameAdminSubmodes = await Promise.all(adminSubmodeSpecs.map(async (spec) => {
    const search = withQueryParam(sessionSearch, "mode", spec.mode);
    const response = await request(`/api/game/admin${search}`, {
      headers: { Cookie: sessionCookiePair }
    });
    return { ...spec, response, body: parseJSON(response) };
  }));

  const allianceRouteSpecs = [
    { name: "home", query: {}, allowedViews: ["home", "no_alliance"] },
    { name: "members", query: { a: "4" }, allowedViews: ["members", "no_alliance"] },
    { name: "management", query: { a: "5" }, allowedViews: ["management", "no_alliance"] },
    { name: "ranks", query: { a: "6" }, allowedViews: ["ranks", "no_alliance"] },
    { name: "applications", query: { page: "bewerbungen" }, allowedViews: ["applications", "no_alliance"] },
    { name: "text", query: { a: "11", d: "1", t: "3" }, allowedViews: ["management", "no_alliance"] },
    { name: "settings", query: { a: "11", d: "2" }, allowedViews: ["management", "no_alliance"] },
    { name: "circular", query: { a: "17" }, allowedViews: ["circular", "no_alliance"] },
    { name: "search", query: { a: "2", suchtext: "AV" }, allowedViews: ["search", "home", "no_alliance"] },
    { name: "create", query: { a: "1" }, allowedViews: ["create", "home", "no_alliance"] }
  ];
  const gameAllianceRoutes = await Promise.all(allianceRouteSpecs.map(async (spec) => {
    const search = withQueryParams(sessionSearch, spec.query);
    const response = await request(`/api/game/alliance${search}`, {
      headers: { Cookie: sessionCookiePair }
    });
    return { ...spec, response, body: parseJSON(response) };
  }));
  const gameAllianceWithoutCookie = await request(`/api/game/alliance${sessionSearch}`);
  const gameAllianceWithoutCookieBody = parseJSON(gameAllianceWithoutCookie);

  const gameOptions = await request(`/api/game/options${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameOptionsBody = {};
  try {
    gameOptionsBody = JSON.parse(gameOptions.body);
  } catch {
    gameOptionsBody = {};
  }

  const gameOptionsUpdate = await request(`/api/game/options${sessionSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", Cookie: sessionCookiePair },
    body: "lang=en&dpath=http%3A%2F%2F127.0.0.1%3A8890%2Fevolution&design=on&settings_sort=9999&settings_order=-9999&spio_anz=-42&settings_fleetactions=99999"
  });
  let gameOptionsUpdateBody = {};
  try {
    gameOptionsUpdateBody = JSON.parse(gameOptionsUpdate.body);
  } catch {
    gameOptionsUpdateBody = {};
  }

  const gameOptionsWithoutCookie = await request(`/api/game/options${sessionSearch}`);
  let gameOptionsWithoutCookieBody = {};
  try {
    gameOptionsWithoutCookieBody = JSON.parse(gameOptionsWithoutCookie.body);
  } catch {
    gameOptionsWithoutCookieBody = {};
  }

  const phalanxSourceMoonID = Number(phalanxFixture.source_moon_id ?? 0);
  const phalanxTargetPlanetID = Number(phalanxFixture.target_planet_id ?? 0);
  const phalanxFixtureReady = phalanxSourceMoonID > 0 && phalanxTargetPlanetID > 0;
  const phalanxSearch = phalanxSourceMoonID > 0 && phalanxTargetPlanetID > 0
    ? withQueryParams(sessionSearch, { cp: phalanxSourceMoonID, spid: phalanxTargetPlanetID })
    : "";
  const gamePhalanxMissingSensor = phalanxTargetPlanetID > 0 && basePlanetID > 0
    ? await request(`/api/game/phalanx${withQueryParams(sessionSearch, { cp: basePlanetID, spid: phalanxTargetPlanetID })}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const gamePhalanxMissingSensorBody = parseJSON(gamePhalanxMissingSensor);
  const gamePhalanx = phalanxSearch
    ? await request(`/api/game/phalanx${phalanxSearch}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const gamePhalanxBody = parseJSON(gamePhalanx);
  const gamePhalanxWithoutCookie = phalanxSearch
    ? await request(`/api/game/phalanx${phalanxSearch}`)
    : { status: 0, headers: {}, body: "{}" };
  const gamePhalanxWithoutCookieBody = parseJSON(gamePhalanxWithoutCookie);

  const gameLogout = await request(`/api/game/logout${sessionSearch}`, {
    method: "POST",
    headers: { Cookie: sessionCookiePair }
  });
  let gameLogoutBody = {};
  try {
    gameLogoutBody = JSON.parse(gameLogout.body);
  } catch {
    gameLogoutBody = {};
  }
  const gameLogoutCookie = gameLogout.headers["set-cookie"] ?? "";
  const gameSessionAfterLogout = await request(`/api/game/session${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  let gameSessionAfterLogoutBody = {};
  try {
    gameSessionAfterLogoutBody = JSON.parse(gameSessionAfterLogout.body);
  } catch {
    gameSessionAfterLogoutBody = {};
  }

  const invalidLogin = await request("/api/public/login/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      login: "",
      pass: "",
      universe: ""
    })
  });
  let invalidLoginBody = {};
  try {
    invalidLoginBody = JSON.parse(invalidLogin.body);
  } catch {
    invalidLoginBody = {};
  }
  const wrongCredentialsIssues = Array.isArray(wrongCredentialsLoginBody.issues) ? wrongCredentialsLoginBody.issues : [];
  const invalidLoginIssues = Array.isArray(invalidLoginBody.issues) ? invalidLoginBody.issues : [];
  cases.push(finalize({
    case: "go_login_validation_api",
    checks: [
      check(validLogin.status === 200, "valid login draft returns HTTP 200", { status: validLogin.status }),
      check(hasHeader(validLogin, "content-type", "application/json"), "valid login draft returns JSON"),
      check(validLoginBody.valid === true, "valid login draft is accepted", validLoginBody),
      check(!validLogin.body.includes(loginSmokePassword), "login validation response does not echo password"),
      check(wrongCredentialsLogin.status === 200, "wrong login credentials return HTTP 200", { status: wrongCredentialsLogin.status }),
      check(wrongCredentialsLoginBody.valid === false, "wrong login credentials are rejected", wrongCredentialsLoginBody),
      check(wrongCredentialsIssues.some((issue) => issue.code === "credentials_invalid" && issue.legacyErrorCode === 2), "wrong login credentials map to legacy error 2", wrongCredentialsLoginBody),
      check(sessionLogin.status === 200, "login submit returns HTTP 200", { status: sessionLogin.status }),
      check(sessionLoginBody.valid === true, "login submit creates a session", sessionLoginBody),
      check(typeof sessionLoginBody.session?.redirectTo === "string" && sessionLoginBody.session.redirectTo.startsWith("/game/overview?"), "login submit returns natural overview redirect", sessionLoginBody),
      check(sessionCookie.includes("prsess_") && sessionCookie.includes("HttpOnly"), "login submit sets private session cookie", { setCookie: sessionCookie }),
      check(sessionCookie.includes("Max-Age=86400"), "login submit sets a 24 hour private session cookie", { setCookie: sessionCookie }),
      check(sessionCookie.includes("SameSite=Lax"), "login submit sets lax same-site cookie policy", { setCookie: sessionCookie }),
      check(!sessionLogin.body.includes(loginSmokePassword), "login submit response does not echo password"),
      check(gameSession.status === 200, "game session lookup returns HTTP 200 with private cookie", { status: gameSession.status }),
      check(gameSessionBody.authenticated === true, "game session lookup authenticates the login session", gameSessionBody),
      check(gameSessionBody.session?.commander === loginSmokeUser, "game session lookup returns commander identity", gameSessionBody),
      check(!gameSession.body.includes(sessionCookiePair), "game session lookup response does not echo private cookie"),
      check(gameSessionWithoutCookie.status === 401, "game session lookup rejects missing private cookie", { status: gameSessionWithoutCookie.status }),
      check(gameSessionWithoutCookieBody.authenticated === false, "missing private cookie is unauthenticated", gameSessionWithoutCookieBody),
      check(gameOverview.status === 200, "game overview returns HTTP 200 with private cookie", { status: gameOverview.status }),
      check(gameOverviewBody.authenticated === true, "game overview authenticates the login session", gameOverviewBody),
      check(
        typeof gameOverviewBody.overview?.commander === "string"
          && gameOverviewBody.overview.commander.toLowerCase() === loginSmokeUser.toLowerCase(),
        "game overview returns commander identity",
        gameOverviewBody
      ),
      check(typeof gameOverviewBody.overview?.currentPlanet?.name === "string" && gameOverviewBody.overview.currentPlanet.name.length > 0, "game overview returns current planet", gameOverviewBody),
      check(Number.isFinite(gameOverviewBody.overview?.currentPlanet?.coordinates?.galaxy), "game overview returns coordinates", gameOverviewBody),
      check(Number.isFinite(gameOverviewBody.overview?.currentPlanet?.resources?.metal), "game overview returns resources", gameOverviewBody),
      check(!gameOverview.body.includes(sessionCookiePair), "game overview response does not echo private cookie"),
      check(gameOverviewWithoutCookie.status === 401, "game overview rejects missing private cookie", { status: gameOverviewWithoutCookie.status }),
      check(gameOverviewWithoutCookieBody.authenticated === false, "game overview missing private cookie is unauthenticated", gameOverviewWithoutCookieBody),
      check(gameOverviewSwitched.status === 200, "game overview accepts selected cp", { status: gameOverviewSwitched.status, switchPlanetID }),
      check(gameOverviewSwitchedBody.overview?.currentPlanet?.id === switchPlanetID, "game overview switches to requested planet", gameOverviewSwitchedBody),
      check(gameOverviewAfterSwitchBody.overview?.currentPlanet?.id === switchPlanetID, "game overview persists selected planet like legacy", gameOverviewAfterSwitchBody),
      check(gameOverviewRestoredBody.overview?.currentPlanet?.id === basePlanetID, "game overview can switch back to base planet", gameOverviewRestoredBody),
      check(gameOverviewRenamed.status === 200, "game overview rename mutation returns HTTP 200", { status: gameOverviewRenamed.status }),
      check(gameOverviewRenamedBody.authenticated === true, "game overview rename mutation stays authenticated", gameOverviewRenamedBody),
      check(gameOverviewRenamedBody.overview?.currentPlanet?.name === renamedPlanetName, "game overview rename mutation updates the current planet name", gameOverviewRenamedBody.overview?.currentPlanet ?? {}),
      check(gameOverviewRenameForbidden.status === 200, "game overview forbidden legacy rename is accepted as a no-op", { status: gameOverviewRenameForbidden.status }),
      check(gameOverviewRenameForbiddenBody.overview?.currentPlanet?.name === renamedPlanetName, "forbidden legacy rename keeps the previous planet name", gameOverviewRenameForbiddenBody.overview?.currentPlanet ?? {}),
      check(gameOverviewRenameRestored.status === 200, "game overview rename mutation can restore the original planet name", { status: gameOverviewRenameRestored.status }),
      check(gameOverviewRenameRestoredBody.overview?.currentPlanet?.name === originalPlanetName, "game overview rename restore updates the current planet name", gameOverviewRenameRestoredBody.overview?.currentPlanet ?? {}),
      check(gameOverviewDeleteWrongPassword.status === 200, "game overview delete wrong password returns HTTP 200", { status: gameOverviewDeleteWrongPassword.status }),
      check(gameOverviewDeleteWrongPasswordBody.actionIssue?.code === "password_invalid", "game overview delete wrong password returns legacy issue", gameOverviewDeleteWrongPasswordBody.actionIssue ?? {}),
      check(gameOverviewDeleteWrongPasswordBody.overview?.currentPlanet?.id === basePlanetID, "game overview delete wrong password keeps current planet", gameOverviewDeleteWrongPasswordBody.overview?.currentPlanet ?? {}),
      check(gameOverviewDeleteHome.status === 200, "game overview home delete returns HTTP 200", { status: gameOverviewDeleteHome.status }),
      check(gameOverviewDeleteHomeBody.actionIssue?.code === "home_planet", "game overview home delete is blocked", gameOverviewDeleteHomeBody.actionIssue ?? {}),
      check(gameOverviewDeleteHomeBody.overview?.currentPlanet?.id === basePlanetID, "game overview home delete keeps current planet", gameOverviewDeleteHomeBody.overview?.currentPlanet ?? {}),
      check(gameOverviewMissingPlanet.status === 200, "game overview accepts missing cp fallback", { status: gameOverviewMissingPlanet.status }),
      check(gameOverviewMissingPlanetBody.overview?.currentPlanet?.id === basePlanetID, "game overview missing cp falls back to base planet", gameOverviewMissingPlanetBody),
      check(gameOverviewAfterMissingPlanetBody.overview?.currentPlanet?.id === basePlanetID, "game overview persists missing cp fallback", gameOverviewAfterMissingPlanetBody),
      check(gameBuildings.status === 200, "game buildings returns HTTP 200 with private cookie", { status: gameBuildings.status }),
      check(gameBuildingsBody.authenticated === true, "game buildings authenticates the login session", gameBuildingsBody),
      check(
        Array.isArray(gameBuildingsBody.buildings?.items)
          && gameBuildingsBody.buildings.items.some((item) => item.name === "Metal Mine"),
        "game buildings returns migrated building rows",
        gameBuildingsBody
      ),
      check(Number.isFinite(gameBuildingsBody.buildings?.items?.[0]?.durationSeconds), "game buildings returns build durations", gameBuildingsBody),
      check(gameBuildingsMutation.status === 200, "game buildings mutation endpoint accepts POST with private cookie", {
        status: gameBuildingsMutation.status
      }),
      check(gameBuildingsMutationBody.authenticated === true, "game buildings mutation authenticates the login session", gameBuildingsMutationBody),
      check(Array.isArray(gameBuildingsMutationBody.buildings?.items), "game buildings mutation returns the refreshed screen", gameBuildingsMutationBody),
      check(gameBuildingsDemolishMutation.status === 200, "game buildings demolish mutation returns HTTP 200", {
        status: gameBuildingsDemolishMutation.status
      }),
      check(gameBuildingsDemolishMutationBody.authenticated === true, "game buildings demolish mutation authenticates the login session", gameBuildingsDemolishMutationBody),
      check(gameBuildingsDemolishMutationBody.actionIssue?.code === "no_such_building", "game buildings demolish mutation reports absent buildings without writing", gameBuildingsDemolishMutationBody.actionIssue ?? {}),
      check(!gameBuildings.body.includes(sessionCookiePair), "game buildings response does not echo private cookie"),
      check(gameBuildingsWithoutCookie.status === 401, "game buildings rejects missing private cookie", { status: gameBuildingsWithoutCookie.status }),
      check(gameBuildingsWithoutCookieBody.authenticated === false, "game buildings missing private cookie is unauthenticated", gameBuildingsWithoutCookieBody),
      check(gameResearch.status === 200, "game research returns HTTP 200 with private cookie", { status: gameResearch.status }),
      check(gameResearchBody.authenticated === true, "game research authenticates the login session", gameResearchBody),
      check(Array.isArray(gameResearchBody.research?.items), "game research returns migrated research rows array", gameResearchBody),
      check(!gameResearch.body.includes(sessionCookiePair), "game research response does not echo private cookie"),
      check(gameResearchWithoutCookie.status === 401, "game research rejects missing private cookie", { status: gameResearchWithoutCookie.status }),
      check(gameResearchWithoutCookieBody.authenticated === false, "game research missing private cookie is unauthenticated", gameResearchWithoutCookieBody),
      check(gameShipyard.status === 200, "game shipyard returns HTTP 200 with private cookie", { status: gameShipyard.status }),
      check(gameShipyardBody.authenticated === true, "game shipyard authenticates the login session", gameShipyardBody),
      check(Array.isArray(gameShipyardBody.shipyard?.items), "game shipyard returns migrated shipyard rows array", gameShipyardBody),
      check(typeof gameShipyardBody.shipyard?.hasShipyard === "boolean", "game shipyard returns shipyard availability", gameShipyardBody),
      check(!gameShipyard.body.includes(sessionCookiePair), "game shipyard response does not echo private cookie"),
      check(gameShipyardWithoutCookie.status === 401, "game shipyard rejects missing private cookie", { status: gameShipyardWithoutCookie.status }),
      check(gameShipyardWithoutCookieBody.authenticated === false, "game shipyard missing private cookie is unauthenticated", gameShipyardWithoutCookieBody),
      check(gameFleet.status === 200, "game fleet returns HTTP 200 with private cookie", { status: gameFleet.status }),
      check(gameFleetBody.authenticated === true, "game fleet authenticates the login session", gameFleetBody),
      check(Number.isFinite(gameFleetBody.fleet?.slots?.used), "game fleet returns used fleet slots", gameFleetBody),
      check(Number.isFinite(gameFleetBody.fleet?.slots?.max), "game fleet returns max fleet slots", gameFleetBody),
      check(Number.isFinite(gameFleetBody.fleet?.expeditions?.max), "game fleet returns expedition slots", gameFleetBody),
      check(Array.isArray(gameFleetBody.fleet?.missions), "game fleet returns active mission rows array", gameFleetBody),
      check(Array.isArray(gameFleetBody.fleet?.ships), "game fleet returns selectable ship rows array", gameFleetBody),
      check(Array.isArray(gameFleetBody.fleet?.templates?.items), "game fleet returns standard fleet templates array", gameFleetBody),
      check(!selectableFleetShip || gameFleetPrepare.status === 200, "game fleet prepares a dispatch draft when ships are available", {
        status: gameFleetPrepare.status,
        selectableFleetShip
      }),
      check(
        !selectableFleetShip || gameFleetPrepareBody.fleet?.dispatchDraft?.ships?.[0]?.count === Number(selectableFleetShip.count),
        "game fleet dispatch draft clamps selected ships to the available count",
        gameFleetPrepareBody.fleet?.dispatchDraft ?? {}
      ),
      check(
        !selectableFleetShip || gameFleetPrepareBody.fleet?.dispatchDraft?.mission === 3,
        "game fleet dispatch draft preserves the requested legacy mission",
        gameFleetPrepareBody.fleet?.dispatchDraft ?? {}
      ),
      check(
        !selectableFleetShip || Array.isArray(gameFleetPrepareBody.fleet?.dispatchDraft?.missionOptions),
        "game fleet dispatch draft returns legacy mission options",
        gameFleetPrepareBody.fleet?.dispatchDraft ?? {}
      ),
      check(
        !selectableFleetShip || Array.isArray(gameFleetPrepareBody.fleet?.dispatchDraft?.resources) && gameFleetPrepareBody.fleet.dispatchDraft.resources.length === 3,
        "game fleet dispatch draft returns transportable resource rows",
        gameFleetPrepareBody.fleet?.dispatchDraft ?? {}
      ),
      check(
        !selectableFleetShip || Number.isFinite(gameFleetPrepareBody.fleet?.dispatchDraft?.distance) && gameFleetPrepareBody.fleet.dispatchDraft.distance > 0,
        "game fleet dispatch draft returns legacy flight distance",
        gameFleetPrepareBody.fleet?.dispatchDraft ?? {}
      ),
      check(
        !selectableFleetShip || Number.isFinite(gameFleetPrepareBody.fleet?.dispatchDraft?.durationSeconds) && gameFleetPrepareBody.fleet.dispatchDraft.durationSeconds > 0,
        "game fleet dispatch draft returns legacy flight duration",
        gameFleetPrepareBody.fleet?.dispatchDraft ?? {}
      ),
      check(
        !selectableFleetShip || Number.isFinite(gameFleetPrepareBody.fleet?.dispatchDraft?.maxSpeed) && gameFleetPrepareBody.fleet.dispatchDraft.maxSpeed > 0,
        "game fleet dispatch draft returns legacy slowest fleet speed",
        gameFleetPrepareBody.fleet?.dispatchDraft ?? {}
      ),
      check(
        !selectableFleetShip || Number.isFinite(gameFleetPrepareBody.fleet?.dispatchDraft?.fuelConsumption) && gameFleetPrepareBody.fleet.dispatchDraft.fuelConsumption >= 0,
        "game fleet dispatch draft returns legacy fuel consumption",
        gameFleetPrepareBody.fleet?.dispatchDraft ?? {}
      ),
      check(!selectableFleetShip || gameFleetValidate.status === 200, "game fleet validates final dispatch payload", {
        status: gameFleetValidate.status,
        selectableFleetShip
      }),
      check(
        !selectableFleetShip || gameFleetValidateBody.actionIssue?.code === "same_planet",
        "game fleet final dispatch validation reports same-planet legacy issue",
        gameFleetValidateBody.actionIssue ?? {}
      ),
      check(
        !selectableFleetShip || Array.isArray(gameFleetValidateBody.fleet?.dispatchDraft?.resources),
        "game fleet final dispatch validation returns resource loading rows",
        gameFleetValidateBody.fleet?.dispatchDraft ?? {}
      ),
      check(!selectableFleetShip || gameFleetLaunch.status === 200, "game fleet accepts final launch dispatch action", {
        status: gameFleetLaunch.status,
        selectableFleetShip
      }),
      check(
        !selectableFleetShip || gameFleetLaunchBody.actionIssue?.code === "same_planet",
        "game fleet launch action reuses final dispatch validation issues",
        gameFleetLaunchBody.actionIssue ?? {}
      ),
      check(!selectableFleetShip || gameFleetNoShips.status === 200, "game fleet no-ships validation returns HTTP 200", {
        status: gameFleetNoShips.status,
        selectableFleetShip
      }),
      check(
        !selectableFleetShip || gameFleetNoShipsBody.actionIssue?.code === "no_ships",
        "game fleet no-ships validation keeps legacy no_ships issue",
        gameFleetNoShipsBody.actionIssue ?? {}
      ),
      check(!selectableFleetShip || gameFleetInvalidOrder.status === 200, "game fleet invalid mission validation returns HTTP 200", {
        status: gameFleetInvalidOrder.status,
        selectableFleetShip
      }),
      check(
        !selectableFleetShip || gameFleetInvalidOrderBody.actionIssue?.code === "invalid_order",
        "game fleet invalid mission validation keeps legacy invalid_order issue",
        gameFleetInvalidOrderBody.actionIssue ?? {}
      ),
      check(!selectableFleetShip || gameFleetInvalidExpeditionTarget.status === 200, "game fleet invalid expedition target validation returns HTTP 200", {
        status: gameFleetInvalidExpeditionTarget.status,
        selectableFleetShip
      }),
      check(
        !selectableFleetShip || gameFleetInvalidExpeditionTargetBody.actionIssue?.code === "invalid_target",
        "game fleet invalid expedition target validation keeps legacy invalid_target issue",
        gameFleetInvalidExpeditionTargetBody.actionIssue ?? {}
      ),
      check(!gameFleet.body.includes(sessionCookiePair), "game fleet response does not echo private cookie"),
      check(gameFleetWithoutCookie.status === 401, "game fleet rejects missing private cookie", { status: gameFleetWithoutCookie.status }),
      check(gameFleetWithoutCookieBody.authenticated === false, "game fleet missing private cookie is unauthenticated", gameFleetWithoutCookieBody),
      check(gameFleetTemplates.status === 200, "game fleet templates return HTTP 200 with private cookie", { status: gameFleetTemplates.status }),
      check(gameFleetTemplatesBody.authenticated === true, "game fleet templates authenticate the login session", gameFleetTemplatesBody),
      check(Array.isArray(gameFleetTemplatesBody.fleet?.templates?.items), "game fleet templates endpoint returns template rows array", gameFleetTemplatesBody),
      check(Number.isFinite(gameFleetTemplatesBody.fleet?.templates?.max), "game fleet templates endpoint returns max standard fleets", gameFleetTemplatesBody),
      check(!gameFleetTemplates.body.includes(sessionCookiePair), "game fleet templates response does not echo private cookie"),
      check(gameFleetTemplatesWithoutCookie.status === 401, "game fleet templates reject missing private cookie", { status: gameFleetTemplatesWithoutCookie.status }),
      check(gameFleetTemplatesWithoutCookieBody.authenticated === false, "game fleet templates missing private cookie is unauthenticated", gameFleetTemplatesWithoutCookieBody),
      check(gameGalaxy.status === 200, "game galaxy returns HTTP 200 with private cookie", { status: gameGalaxy.status }),
      check(gameGalaxyBody.authenticated === true, "game galaxy authenticates the login session", gameGalaxyBody),
      check(Array.isArray(gameGalaxyBody.galaxy?.rows) && gameGalaxyBody.galaxy.rows.length === 15, "game galaxy returns 15 visible system rows", gameGalaxyBody),
      check(Number.isFinite(gameGalaxyBody.galaxy?.coordinates?.galaxy), "game galaxy returns selected galaxy coordinate", gameGalaxyBody),
      check(Number.isFinite(gameGalaxyBody.galaxy?.coordinates?.system), "game galaxy returns selected system coordinate", gameGalaxyBody),
      check(Number.isFinite(gameGalaxyBody.galaxy?.slots?.max), "game galaxy returns fleet slot summary", gameGalaxyBody),
      check(typeof gameGalaxyBody.galaxy?.extra?.commander === "boolean", "game galaxy returns commander extra info state", gameGalaxyBody),
      check(Number.isFinite(gameGalaxyBody.galaxy?.extra?.maxSpy), "game galaxy returns max spy shortcut setting", gameGalaxyBody),
      check(gameGalaxySpyDispatch.status === 200, "game galaxy accepts instant spy dispatch action", {
        status: gameGalaxySpyDispatch.status,
        body: gameGalaxySpyDispatchBody
      }),
      check(gameGalaxySpyDispatchBody.actionIssue?.code === "fleet_no_ships", "game galaxy instant spy reaches fleet validation", gameGalaxySpyDispatchBody),
      check(gameGalaxyRecycleDispatch.status === 200, "game galaxy accepts instant recycle dispatch action", {
        status: gameGalaxyRecycleDispatch.status,
        body: gameGalaxyRecycleDispatchBody
      }),
      check(gameGalaxyRecycleDispatchBody.actionIssue?.code === "fleet_no_ships", "game galaxy instant recycle reaches fleet validation", gameGalaxyRecycleDispatchBody),
      check(!gameGalaxy.body.includes(sessionCookiePair), "game galaxy response does not echo private cookie"),
      check(gameGalaxyWithoutCookie.status === 401, "game galaxy rejects missing private cookie", { status: gameGalaxyWithoutCookie.status }),
      check(gameGalaxyWithoutCookieBody.authenticated === false, "game galaxy missing private cookie is unauthenticated", gameGalaxyWithoutCookieBody),
      check(gameDefense.status === 200, "game defense returns HTTP 200 with private cookie", { status: gameDefense.status }),
      check(gameDefenseBody.authenticated === true, "game defense authenticates the login session", gameDefenseBody),
      check(Array.isArray(gameDefenseBody.defense?.items), "game defense returns migrated defense rows array", gameDefenseBody),
      check(typeof gameDefenseBody.defense?.hasShipyard === "boolean", "game defense returns shipyard availability", gameDefenseBody),
      check(!gameDefense.body.includes(sessionCookiePair), "game defense response does not echo private cookie"),
      check(gameDefenseWithoutCookie.status === 401, "game defense rejects missing private cookie", { status: gameDefenseWithoutCookie.status }),
      check(gameDefenseWithoutCookieBody.authenticated === false, "game defense missing private cookie is unauthenticated", gameDefenseWithoutCookieBody),
      check(gameEmpire.status === 200, "game empire returns HTTP 200 with private cookie", { status: gameEmpire.status }),
      check(gameEmpireBody.authenticated === true, "game empire authenticates the login session", gameEmpireBody),
      check(Array.isArray(gameEmpireBody.empire?.planets), "game empire returns planet columns array", gameEmpireBody),
      check(Array.isArray(gameEmpireBody.empire?.resources), "game empire returns resource rows array", gameEmpireBody),
      check(Array.isArray(gameEmpireBody.empire?.buildings), "game empire returns building rows array", gameEmpireBody),
      check(Array.isArray(gameEmpireBody.empire?.research), "game empire returns research rows array", gameEmpireBody),
      check(Array.isArray(gameEmpireBody.empire?.fleet), "game empire returns fleet rows array", gameEmpireBody),
      check(Array.isArray(gameEmpireBody.empire?.defense), "game empire returns defense rows array", gameEmpireBody),
      check(gameEmpireMoons.status === 200, "game empire accepts moon planet type", { status: gameEmpireMoons.status }),
      check([1, 3].includes(gameEmpireMoonsBody.empire?.planetType), "game empire normalizes planet type like legacy", gameEmpireMoonsBody),
      check(gameEmpireInvalidShortcut.status === 200, "game empire accepts legacy GET shortcut parameters", {
        status: gameEmpireInvalidShortcut.status
      }),
      check(gameEmpireInvalidShortcutBody.authenticated === true, "game empire shortcut authenticates the login session", gameEmpireInvalidShortcutBody),
      check(gameEmpireInvalidShortcutBody.actionIssue?.code === "invalid_building", "game empire shortcut reports invalid building without writing", gameEmpireInvalidShortcutBody.actionIssue ?? {}),
      check(!gameEmpire.body.includes(sessionCookiePair), "game empire response does not echo private cookie"),
      check(gameEmpireWithoutCookie.status === 401, "game empire rejects missing private cookie", { status: gameEmpireWithoutCookie.status }),
      check(gameEmpireWithoutCookieBody.authenticated === false, "game empire missing private cookie is unauthenticated", gameEmpireWithoutCookieBody),
      check(gameTechnology.status === 200, "game technology returns HTTP 200 with private cookie", { status: gameTechnology.status }),
      check(gameTechnologyBody.authenticated === true, "game technology authenticates the login session", gameTechnologyBody),
      check(Array.isArray(gameTechnologyBody.technology?.groups), "game technology returns migrated technology groups", gameTechnologyBody),
      check(
        gameTechnologyBody.technology?.groups?.some((group) => group.name === "Buildings" && Array.isArray(group.items)),
        "game technology returns building requirement group",
        gameTechnologyBody
      ),
      check(gameTechnologyDetails.status === 200, "game technology details returns HTTP 200 with private cookie", {
        status: gameTechnologyDetails.status
      }),
      check(
        gameTechnologyDetailsBody.technology?.details?.target?.name === "Cruiser",
        "game technology details returns selected target",
        gameTechnologyDetailsBody
      ),
      check(
        Array.isArray(gameTechnologyDetailsBody.technology?.details?.levels),
        "game technology details returns recursive requirement levels",
        gameTechnologyDetailsBody
      ),
      check(!gameTechnology.body.includes(sessionCookiePair), "game technology response does not echo private cookie"),
      check(gameTechnologyWithoutCookie.status === 401, "game technology rejects missing private cookie", { status: gameTechnologyWithoutCookie.status }),
      check(gameTechnologyWithoutCookieBody.authenticated === false, "game technology missing private cookie is unauthenticated", gameTechnologyWithoutCookieBody),
      check(gameStatistics.status === 200, "game statistics returns HTTP 200 with private cookie", { status: gameStatistics.status }),
      check(gameStatisticsBody.authenticated === true, "game statistics authenticates the login session", gameStatisticsBody),
      check(gameStatisticsBody.statistics?.type === "ressources", "game statistics keeps legacy points type spelling", gameStatisticsBody),
      check(Array.isArray(gameStatisticsBody.statistics?.rows), "game statistics returns ranking rows array", gameStatisticsBody),
      check(Number.isFinite(gameStatisticsBody.statistics?.start), "game statistics returns selected ranking window", gameStatisticsBody),
      check(
        gameStatisticsBody.statistics?.rows?.some((row) => typeof row.player?.name === "string" && row.player.name.length > 0),
        "game statistics rows include player names",
        gameStatisticsBody
      ),
      check(gameFleetStatistics.status === 200, "game fleet statistics returns HTTP 200 with private cookie", {
        status: gameFleetStatistics.status
      }),
      check(gameFleetStatisticsBody.statistics?.type === "fleet", "game fleet statistics returns fleet type", gameFleetStatisticsBody),
      check(gameResearchStatistics.status === 200, "game research statistics returns HTTP 200 with private cookie", {
        status: gameResearchStatistics.status
      }),
      check(gameResearchStatisticsBody.statistics?.type === "research", "game research statistics returns research type", gameResearchStatisticsBody),
      check(gameAllianceStatistics.status === 200, "game alliance statistics returns HTTP 200 with private cookie", {
        status: gameAllianceStatistics.status
      }),
      check(gameAllianceStatisticsBody.statistics?.who === "ally", "game alliance statistics keeps alliance mode", gameAllianceStatisticsBody),
      check(
        Array.isArray(gameAllianceStatisticsBody.statistics?.rows) &&
          gameAllianceStatisticsBody.statistics.rows.every((row) => Number.isFinite(row.members) && Number.isFinite(row.perMember)),
        "game alliance statistics rows expose member and per-member scores",
        gameAllianceStatisticsBody
      ),
      check(!gameStatistics.body.includes(sessionCookiePair), "game statistics response does not echo private cookie"),
      check(gameStatisticsWithoutCookie.status === 401, "game statistics rejects missing private cookie", { status: gameStatisticsWithoutCookie.status }),
      check(gameStatisticsWithoutCookieBody.authenticated === false, "game statistics missing private cookie is unauthenticated", gameStatisticsWithoutCookieBody),
      check(gameSearch.status === 200, "game search returns HTTP 200 with private cookie", { status: gameSearch.status }),
      check(gameSearchBody.authenticated === true, "game search authenticates the login session", gameSearchBody),
      check(gameSearchBody.search?.type === "playername", "game search keeps legacy player search type", gameSearchBody),
      check(Array.isArray(gameSearchBody.search?.playerRows), "game search returns player rows array", gameSearchBody),
      check(gameAllianceSearch.status === 200, "game alliance search returns HTTP 200 with private cookie", {
        status: gameAllianceSearch.status
      }),
      check(gameAllianceSearchBody.search?.type === "allytag", "game alliance search keeps alliance tag type", gameAllianceSearchBody),
      check(Array.isArray(gameAllianceSearchBody.search?.allianceRows), "game alliance search returns alliance rows array", gameAllianceSearchBody),
      check(!gameSearch.body.includes(sessionCookiePair), "game search response does not echo private cookie"),
      check(gameSearchWithoutCookie.status === 401, "game search rejects missing private cookie", { status: gameSearchWithoutCookie.status }),
      check(gameSearchWithoutCookieBody.authenticated === false, "game search missing private cookie is unauthenticated", gameSearchWithoutCookieBody),
      check(gameBuddy.status === 200, "game buddy returns HTTP 200 with private cookie", { status: gameBuddy.status }),
      check(gameBuddyBody.authenticated === true, "game buddy authenticates the login session", gameBuddyBody),
      check(gameBuddyBody.buddy?.action === 0, "game buddy defaults to home action", gameBuddyBody),
      check(Array.isArray(gameBuddyBody.buddy?.rows), "game buddy returns buddy rows array", gameBuddyBody),
      check(gameBuddyRequest.status === 200, "game buddy request form returns HTTP 200 with private cookie", {
        status: gameBuddyRequest.status
      }),
      check(gameBuddyRequestBody.buddy?.action === 7, "game buddy keeps legacy request action", gameBuddyRequestBody),
      check(gameBuddyMutation.status === 200, "game buddy mutation endpoint accepts POST with private cookie", {
        status: gameBuddyMutation.status
      }),
      check(gameBuddyMutationBody.authenticated === true, "game buddy mutation authenticates the login session", gameBuddyMutationBody),
      check(gameBuddyMutationBody.buddy?.action === 0, "game buddy mutation returns the next legacy screen", gameBuddyMutationBody),
      check(!gameBuddy.body.includes(sessionCookiePair), "game buddy response does not echo private cookie"),
      check(gameBuddyWithoutCookie.status === 401, "game buddy rejects missing private cookie", { status: gameBuddyWithoutCookie.status }),
      check(gameBuddyWithoutCookieBody.authenticated === false, "game buddy missing private cookie is unauthenticated", gameBuddyWithoutCookieBody),
      check(gameMessages.status === 200, "game messages returns HTTP 200 with private cookie", { status: gameMessages.status }),
      check(gameMessagesBody.authenticated === true, "game messages authenticates the login session", gameMessagesBody),
      check(gameMessagesBody.messages?.action === "inbox", "game messages defaults to inbox action", gameMessagesBody),
      check(Array.isArray(gameMessagesBody.messages?.rows), "game messages returns message rows array", gameMessagesBody),
      check(gameMessagesCompose.status === 200, "game message compose returns HTTP 200 with private cookie", {
        status: gameMessagesCompose.status
      }),
      check(gameMessagesComposeBody.messages?.action === "compose", "game messages keeps legacy compose action", gameMessagesComposeBody),
      check(gameMessagesComposeBody.messages?.compose?.target?.playerId === loginPlayerId, "game messages compose returns target player", {
        loginPlayerId,
        body: gameMessagesComposeBody
      }),
      check(gameMessagesSend.status === 200, "game message send accepts POST with private cookie", { status: gameMessagesSend.status }),
      check(gameMessagesSendBody.authenticated === true, "game message send authenticates the login session", gameMessagesSendBody),
      check(gameMessagesSendBody.actionIssue?.code === "sent", "game message send returns sent action issue", gameMessagesSendBody),
      check(gameMessagesSendBody.messages?.action === "compose", "game message send returns compose screen", gameMessagesSendBody),
      check(gameMessagesAfterSend.status === 200, "game messages inbox can reload after sending a PM", {
        status: gameMessagesAfterSend.status
      }),
      check(sentReportID > 0, "game messages exposes the sent PM id for report-popup compatibility", sentMessageRow ?? {}),
      check(gameReport.status === 200, "game report returns HTTP 200 with private cookie", { status: gameReport.status }),
      check(gameReportBody.authenticated === true, "game report authenticates the login session", gameReportBody),
      check(gameReportBody.report?.id === sentReportID, "game report maps the requested bericht id", gameReportBody),
      check(gameReportBody.report?.allowed === true, "game report allows owner access", gameReportBody),
      check(String(gameReportBody.report?.text ?? "").includes("Go migration message smoke"), "game report renders the report body text", gameReportBody),
      check(gameReportWithoutCookie.status === 401, "game report rejects missing private cookie", {
        status: gameReportWithoutCookie.status
      }),
      check(gameReportWithoutCookieBody.authenticated === false, "game report missing private cookie is unauthenticated", gameReportWithoutCookieBody),
      check(!gameMessages.body.includes(sessionCookiePair), "game messages response does not echo private cookie"),
      check(gameMessagesWithoutCookie.status === 401, "game messages rejects missing private cookie", { status: gameMessagesWithoutCookie.status }),
      check(gameMessagesWithoutCookieBody.authenticated === false, "game messages missing private cookie is unauthenticated", gameMessagesWithoutCookieBody),
      check(gameNotes.status === 200, "game notes returns HTTP 200 with private cookie", { status: gameNotes.status }),
      check(gameNotesBody.authenticated === true, "game notes authenticates the login session", gameNotesBody),
      check(gameNotesBody.notes?.action === "list", "game notes defaults to list action", gameNotesBody),
      check(Array.isArray(gameNotesBody.notes?.rows), "game notes returns notes rows array", gameNotesBody),
      check(gameNotesCreate.status === 200, "game notes create form returns HTTP 200 with private cookie", {
        status: gameNotesCreate.status
      }),
      check(gameNotesCreateBody.notes?.action === "create", "game notes keeps legacy create action", gameNotesCreateBody),
      check(gameNotesCreatePost.status === 200, "game notes creates notes over POST", { status: gameNotesCreatePost.status }),
      check(createdNote?.subject === noteSubject && createdNote?.priority === 2, "game notes create returns the new note", {
        createdNote
      }),
      check(gameNotesUpdatePost.status === 200, "game notes updates notes over POST", { status: gameNotesUpdatePost.status }),
      check(updatedNote?.subject === updatedNoteSubject && updatedNote?.priority === 0, "game notes update returns the updated note", {
        updatedNote
      }),
      check(gameNotesDeletePost.status === 200, "game notes deletes notes over POST", { status: gameNotesDeletePost.status }),
      check(
        Array.isArray(gameNotesDeletePostBody.notes?.rows) &&
          !gameNotesDeletePostBody.notes.rows.some((row) => row.subject === updatedNoteSubject),
        "game notes delete removes the note from the returned list",
        gameNotesDeletePostBody
      ),
      check(!gameNotes.body.includes(sessionCookiePair), "game notes response does not echo private cookie"),
      check(gameNotesWithoutCookie.status === 401, "game notes rejects missing private cookie", { status: gameNotesWithoutCookie.status }),
      check(gameNotesWithoutCookieBody.authenticated === false, "game notes missing private cookie is unauthenticated", gameNotesWithoutCookieBody),
      check(gameResources.status === 200, "game resources returns HTTP 200 with private cookie", { status: gameResources.status }),
      check(gameResourcesBody.authenticated === true, "game resources authenticates the login session", gameResourcesBody),
      check(Number.isFinite(gameResourcesBody.resources?.factor), "game resources returns production factor", gameResourcesBody),
      check(Number.isFinite(gameResourcesBody.resources?.natural?.metal), "game resources returns natural production", gameResourcesBody),
      check(Number.isFinite(gameResourcesBody.resources?.totals?.hour?.metal), "game resources returns hourly totals", gameResourcesBody),
      check(Array.isArray(gameResourcesBody.resources?.rows), "game resources returns production rows array", gameResourcesBody),
      check(!gameResources.body.includes(sessionCookiePair), "game resources response does not echo private cookie"),
      check(gameResourcesUpdate.status === 200, "game resources production update returns HTTP 200 with private cookie", { status: gameResourcesUpdate.status }),
      check(gameResourcesUpdateBody.authenticated === true, "game resources production update authenticates the login session", gameResourcesUpdateBody),
      check(Number.isFinite(gameResourcesUpdateBody.resources?.factor), "game resources production update returns recalculated resources", gameResourcesUpdateBody),
      check(!gameResourcesUpdate.body.includes(sessionCookiePair), "game resources production update response does not echo private cookie"),
      check(gameResourcesWithoutCookie.status === 401, "game resources rejects missing private cookie", { status: gameResourcesWithoutCookie.status }),
      check(gameResourcesWithoutCookieBody.authenticated === false, "game resources missing private cookie is unauthenticated", gameResourcesWithoutCookieBody),
      check(gameMerchant.status === 200, "game merchant returns HTTP 200 with private cookie", { status: gameMerchant.status }),
      check(gameMerchantBody.authenticated === true, "game merchant authenticates the login session", gameMerchantBody),
      check(Number.isFinite(gameMerchantBody.merchant?.activeOfferId), "game merchant returns active offer state", gameMerchantBody),
      check(Array.isArray(gameMerchantBody.merchant?.rows), "game merchant returns resource rows array", gameMerchantBody),
      check(Array.isArray(gameMerchantBody.merchant?.planetSwitcher), "game merchant returns planet switcher", gameMerchantBody),
      check(!gameMerchant.body.includes(sessionCookiePair), "game merchant response does not echo private cookie"),
      check(gameMerchantWithoutCookie.status === 401, "game merchant rejects missing private cookie", { status: gameMerchantWithoutCookie.status }),
      check(gameMerchantWithoutCookieBody.authenticated === false, "game merchant missing private cookie is unauthenticated", gameMerchantWithoutCookieBody),
      check(gameOfficers.status === 200, "game officers returns HTTP 200 with private cookie", { status: gameOfficers.status }),
      check(gameOfficersBody.authenticated === true, "game officers authenticates the login session", gameOfficersBody),
      check(Array.isArray(gameOfficersBody.officers?.rows), "game officers returns officer rows array", gameOfficersBody),
      check(gameOfficersBody.officers?.rows?.some((row) => row.name === "Commander"), "game officers returns commander row", gameOfficersBody),
      check(Array.isArray(gameOfficersBody.officers?.planetSwitcher), "game officers returns planet switcher", gameOfficersBody),
      check(gameOfficersInvalid.status === 200, "game officers accepts legacy form POST", { status: gameOfficersInvalid.status }),
      check(gameOfficersInvalidBody.authenticated === true, "game officers invalid legacy POST authenticates without mutating", gameOfficersInvalidBody),
      check(!gameOfficers.body.includes(sessionCookiePair), "game officers response does not echo private cookie"),
      check(gameOfficersWithoutCookie.status === 401, "game officers rejects missing private cookie", { status: gameOfficersWithoutCookie.status }),
      check(gameOfficersWithoutCookieBody.authenticated === false, "game officers missing private cookie is unauthenticated", gameOfficersWithoutCookieBody),
      check(gameAdmin.status === 200, "game admin returns HTTP 200 with private cookie", { status: gameAdmin.status }),
      check(gameAdminBody.authenticated === true, "game admin authenticates the login session", gameAdminBody),
      check(Array.isArray(gameAdminBody.admin?.menu), "game admin returns home menu items", gameAdminBody),
      check(gameAdminBody.admin?.menu?.some((row) => row.label === "Fleet Logs"), "game admin menu includes Fleet Logs", gameAdminBody),
      check(!gameAdmin.body.includes(sessionCookiePair), "game admin response does not echo private cookie"),
      check(gameAdminWithoutCookie.status === 401, "game admin rejects missing private cookie", { status: gameAdminWithoutCookie.status }),
      check(gameAdminWithoutCookieBody.authenticated === false, "game admin missing private cookie is unauthenticated", gameAdminWithoutCookieBody),
      check(gameOptions.status === 200, "game options returns HTTP 200 with private cookie", { status: gameOptions.status }),
      check(gameOptionsBody.authenticated === true, "game options authenticates the login session", gameOptionsBody),
      check(typeof gameOptionsBody.options?.user?.name === "string" && gameOptionsBody.options.user.name.length > 0, "game options returns user data", gameOptionsBody),
      check(Number.isFinite(gameOptionsBody.options?.settings?.maxSpy), "game options returns galaxy settings", gameOptionsBody),
      check(Array.isArray(gameOptionsBody.options?.planetSwitcher), "game options returns planet switcher", gameOptionsBody),
      check(gameOptionsUpdate.status === 200, "game options accepts legacy form POST", { status: gameOptionsUpdate.status }),
      check(gameOptionsUpdateBody.authenticated === true, "game options update authenticates the login session", gameOptionsUpdateBody),
      check(gameOptionsUpdateBody.options?.settings?.skinPath === "/evolution/", "game options normalizes loopback skin path", gameOptionsUpdateBody),
      check(gameOptionsUpdateBody.options?.settings?.sortBy === 2, "game options clamps sort field like legacy", gameOptionsUpdateBody),
      check(gameOptionsUpdateBody.options?.settings?.sortOrder === 0, "game options clamps sort direction like legacy", gameOptionsUpdateBody),
      check(gameOptionsUpdateBody.options?.settings?.maxSpy === 1, "game options clamps spy probes like legacy", gameOptionsUpdateBody),
      check(gameOptionsUpdateBody.options?.settings?.maxFleetMessages === 99, "game options clamps max fleet messages like legacy", gameOptionsUpdateBody),
      check(!gameOptions.body.includes(sessionCookiePair), "game options response does not echo private cookie"),
      check(gameOptionsWithoutCookie.status === 401, "game options rejects missing private cookie", { status: gameOptionsWithoutCookie.status }),
      check(gameOptionsWithoutCookieBody.authenticated === false, "game options missing private cookie is unauthenticated", gameOptionsWithoutCookieBody),
      check(!smokeFixtureFile || phalanxFixtureReady, "go smoke fixture exposes phalanx moon and target ids", {
        smokeFixtureFile,
        phalanxFixture
      }),
      check(!phalanxFixtureReady || gamePhalanxMissingSensor.status === 200, "game phalanx missing-sensor scan returns HTTP 200", {
        status: gamePhalanxMissingSensor.status
      }),
      check(
        !phalanxFixtureReady || gamePhalanxMissingSensorBody.phalanx?.actionIssue?.code === "missing_sensor",
        "game phalanx keeps legacy missing-sensor rejection",
        gamePhalanxMissingSensorBody.phalanx?.actionIssue ?? {}
      ),
      check(!phalanxFixtureReady || gamePhalanx.status === 200, "game phalanx success scan returns HTTP 200", {
        status: gamePhalanx.status
      }),
      check(!phalanxFixtureReady || gamePhalanxBody.authenticated === true, "game phalanx authenticates the login session", gamePhalanxBody),
      check(!phalanxFixtureReady || gamePhalanxBody.phalanx?.source?.id === phalanxSourceMoonID, "game phalanx uses selected source moon", gamePhalanxBody.phalanx?.source ?? {}),
      check(!phalanxFixtureReady || gamePhalanxBody.phalanx?.target?.id === phalanxTargetPlanetID, "game phalanx scans selected target planet", gamePhalanxBody.phalanx?.target ?? {}),
      check(!phalanxFixtureReady || gamePhalanxBody.phalanx?.actionIssue === undefined, "game phalanx success scan has no action issue", gamePhalanxBody.phalanx ?? {}),
      check(
        !phalanxFixtureReady || gamePhalanxBody.phalanx?.remainingDeuterium === Number(phalanxFixture.initial_deuterium ?? 0) - Number(phalanxFixture.cost ?? 0),
        "game phalanx success scan spends exactly the legacy deuterium cost",
        gamePhalanxBody.phalanx ?? {}
      ),
      check(
        !phalanxFixtureReady || Array.isArray(gamePhalanxBody.phalanx?.events) && gamePhalanxBody.phalanx.events.some((event) => Number(event.id) === Number(phalanxFixture.fleet_id ?? 0) || Number(event.mission) === 3),
        "game phalanx success scan returns the visible fixture fleet event",
        gamePhalanxBody.phalanx?.events ?? []
      ),
      check(!phalanxFixtureReady || !gamePhalanx.body.includes(sessionCookiePair), "game phalanx response does not echo private cookie"),
      check(!phalanxFixtureReady || gamePhalanxWithoutCookie.status === 401, "game phalanx rejects missing private cookie", { status: gamePhalanxWithoutCookie.status }),
      check(!phalanxFixtureReady || gamePhalanxWithoutCookieBody.authenticated === false, "game phalanx missing private cookie is unauthenticated", gamePhalanxWithoutCookieBody),
      check(gameLogout.status === 200, "game logout returns HTTP 200 with private cookie", { status: gameLogout.status }),
      check(gameLogoutBody.loggedOut === true, "game logout clears the active legacy session", gameLogoutBody),
      check(gameLogoutBody.redirectTo === "/home", "game logout redirects to public home", gameLogoutBody),
      check(
        gameLogoutCookie.includes(`${sessionCookieName}=;`) && gameLogoutCookie.includes("Max-Age=0"),
        "game logout expires the private session cookie",
        { setCookie: gameLogoutCookie }
      ),
      check(gameSessionAfterLogout.status === 401, "game session lookup rejects the logged-out public session", {
        status: gameSessionAfterLogout.status
      }),
      check(gameSessionAfterLogoutBody.authenticated === false, "logged-out public session is unauthenticated", gameSessionAfterLogoutBody),
      check(invalidLogin.status === 200, "invalid login draft returns HTTP 200", { status: invalidLogin.status }),
      check(invalidLoginBody.valid === false, "invalid login draft is rejected", invalidLoginBody),
      check(invalidLoginIssues.some((issue) => issue.code === "login_required" && issue.legacyErrorCode === 2), "missing login maps to legacy error 2", invalidLoginBody),
      check(invalidLoginIssues.some((issue) => issue.code === "password_required" && issue.legacyErrorCode === 2), "missing password maps to legacy error 2", invalidLoginBody),
      check(invalidLoginIssues.some((issue) => issue.code === "universe_required"), "missing universe is reported for multi-universe entry", invalidLoginBody)
    ]
  }));

  const adminSubmodeChecks = gameAdminSubmodes.flatMap((item) => {
    const expectedMode = item.expectedMode ?? item.mode;
    const payloadCheck = item.arrayKey
      ? check(
          item.body.admin?.[item.arrayKey] === undefined || Array.isArray(item.body.admin?.[item.arrayKey]),
          `admin ${item.name} returns ${item.arrayKey} array or omits an empty payload`,
          item.body.admin ?? {}
        )
      : item.objectKey
        ? check(item.body.admin?.[item.objectKey] !== undefined && item.body.admin?.[item.objectKey] !== null, `admin ${item.name} returns ${item.objectKey} payload`, item.body.admin ?? {})
        : check(Array.isArray(item.body.admin?.menu), `admin ${item.name} returns menu payload`, item.body.admin ?? {});
    return [
      check(item.response.status === 200, `admin ${item.name} returns HTTP 200`, { status: item.response.status }),
      check(item.body.authenticated === true, `admin ${item.name} authenticates`, item.body),
      check(item.body.admin?.mode === expectedMode, `admin ${item.name} resolves legacy mode`, item.body.admin ?? {}),
      check(item.body.actionIssue === undefined, `admin ${item.name} is not permission-denied for admin smoke user`, item.body.actionIssue ?? {}),
      payloadCheck
    ];
  });
  cases.push(finalize({
    case: "go_admin_submode_matrix_api",
    checks: adminSubmodeChecks
  }));

  const allianceRouteChecks = gameAllianceRoutes.flatMap((item) => [
    check(item.response.status === 200, `alliance ${item.name} returns HTTP 200`, { status: item.response.status }),
    check(item.body.authenticated === true, `alliance ${item.name} authenticates`, item.body),
    check(item.allowedViews.includes(item.body.alliance?.view), `alliance ${item.name} resolves an expected legacy view`, {
      expected: item.allowedViews,
      actual: item.body.alliance?.view,
      body: item.body
    }),
    check(Array.isArray(item.body.alliance?.members), `alliance ${item.name} returns members array`, item.body.alliance ?? {}),
    check(Array.isArray(item.body.alliance?.applications), `alliance ${item.name} returns applications array`, item.body.alliance ?? {}),
    check(Array.isArray(item.body.alliance?.ranks), `alliance ${item.name} returns ranks array`, item.body.alliance ?? {})
  ]);
  cases.push(finalize({
    case: "go_alliance_deep_state_routes_api",
    checks: [
      ...allianceRouteChecks,
      check(!gameAllianceRoutes.some((item) => item.response.body.includes(sessionCookiePair)), "alliance route matrix does not echo private cookie"),
      check(gameAllianceWithoutCookie.status === 401, "alliance route rejects missing private cookie", { status: gameAllianceWithoutCookie.status }),
      check(gameAllianceWithoutCookieBody.authenticated === false, "alliance route missing private cookie is unauthenticated", gameAllianceWithoutCookieBody)
    ]
  }));

  const root = await request("/");
  cases.push(finalize({
    case: "go_react_shell",
    checks: [
      check(root.status === 200, "root returns HTTP 200", { status: root.status }),
      check(root.body.includes('<div id="root">'), "root renders React mount node"),
      check(root.body.includes("/assets/main.js"), "root references React JS bundle"),
      check(root.body.includes("/assets/main.css"), "root references React CSS bundle"),
      check(!root.body.includes("Master Database Settings"), "root does not render legacy installer form"),
      check(noLoopbackAsset(root.body), "root does not emit loopback absolute asset URLs"),
      check(hasHeader(root, "x-frame-options", "SAMEORIGIN"), "root has security headers")
    ]
  }));

  const publicStartBackground = await request("/public-assets/img/startseite_bg.jpg");
  const publicLoginButton = await request("/public-assets/img/login_button.jpg");
  const publicRegisterPanel = await request("/public-assets/img/part_register2.jpg");
  const publicBigPanel = await request("/public-assets/img/part_big.jpg");
  const publicAboutImage = await request("/public-assets/img/ogame_admiral.jpg");
  const publicStoryImage = await request("/public-assets/img/legorians.jpg");
  const publicFightImage = await request("/public-assets/img/fight.gif");
  const publicScreenshotThumb = await request("/public-assets/img/overview_t.jpg");
  const publicWallpaperThumb = await request("/public-assets/img/battleship_t.jpg");
  cases.push(finalize({
    case: "go_public_legacy_assets",
    checks: [
      check(publicStartBackground.status === 200, "legacy public start background returns HTTP 200", { status: publicStartBackground.status }),
      check(hasHeader(publicStartBackground, "content-type", "image/jpeg"), "legacy public start background has JPEG content type"),
      check(publicLoginButton.status === 200, "legacy public login button returns HTTP 200", { status: publicLoginButton.status }),
      check(hasHeader(publicLoginButton, "content-type", "image/jpeg"), "legacy public login button has JPEG content type"),
      check(publicRegisterPanel.status === 200, "legacy public registration panel returns HTTP 200", { status: publicRegisterPanel.status }),
      check(hasHeader(publicRegisterPanel, "content-type", "image/jpeg"), "legacy public registration panel has JPEG content type"),
      check(publicBigPanel.status === 200, "legacy public big panel returns HTTP 200", { status: publicBigPanel.status }),
      check(hasHeader(publicBigPanel, "content-type", "image/jpeg"), "legacy public big panel has JPEG content type"),
      check(publicAboutImage.status === 200, "legacy public about image returns HTTP 200", { status: publicAboutImage.status }),
      check(hasHeader(publicAboutImage, "content-type", "image/jpeg"), "legacy public about image has JPEG content type"),
      check(publicStoryImage.status === 200, "legacy public story image returns HTTP 200", { status: publicStoryImage.status }),
      check(hasHeader(publicStoryImage, "content-type", "image/jpeg"), "legacy public story image has JPEG content type"),
      check(publicFightImage.status === 200, "legacy public story gif returns HTTP 200", { status: publicFightImage.status }),
      check(hasHeader(publicFightImage, "content-type", "image/gif"), "legacy public story gif has GIF content type"),
      check(publicScreenshotThumb.status === 200, "legacy public screenshot thumbnail returns HTTP 200", { status: publicScreenshotThumb.status }),
      check(hasHeader(publicScreenshotThumb, "content-type", "image/jpeg"), "legacy public screenshot thumbnail has JPEG content type"),
      check(publicWallpaperThumb.status === 200, "legacy public wallpaper thumbnail returns HTTP 200", { status: publicWallpaperThumb.status }),
      check(hasHeader(publicWallpaperThumb, "content-type", "image/jpeg"), "legacy public wallpaper thumbnail has JPEG content type")
    ]
  }));

  const fallback = await request("/game/overview");
  cases.push(finalize({
    case: "go_spa_fallback",
    checks: [
      check(fallback.status === 200, "game route falls back to React shell", { status: fallback.status }),
      check(fallback.body.includes('<div id="root">'), "fallback response renders React mount node")
    ]
  }));

  const naturalPublicPaths = publicRoutes.map((route) => route.path);
  const naturalPublicChecks = [];
  for (const path of naturalPublicPaths) {
    const response = await request(path);
    naturalPublicChecks.push(
      check(response.status === 200, `${path} returns React shell`, { status: response.status }),
      check(response.body.includes('<div id="root">'), `${path} renders React mount node`),
      check(!response.body.includes("Master Database Settings"), `${path} does not render installer form`)
    );
  }
  cases.push(finalize({
    case: "go_natural_public_routes",
    checks: naturalPublicChecks
  }));

  const legacyPublicPaths = Array.from(publicRouteAliases.keys());
  const legacyPublicChecks = [];
  for (const path of legacyPublicPaths) {
    const response = await request(path);
    legacyPublicChecks.push(
      check(response.status === 200, `${path} returns React shell`, { status: response.status }),
      check(response.body.includes('<div id="root">'), `${path} renders React mount node`),
      check(!response.body.includes("Master Database Settings"), `${path} does not render installer form`)
    );
  }
  cases.push(finalize({
    case: "go_legacy_public_routes",
    checks: legacyPublicChecks
  }));

  const js = await request("/assets/main.js");
  const css = await request("/assets/main.css");
  const legacyGameOverviewSource = await readFile(new URL("../../frontend/src/LegacyGameOverview.tsx", import.meta.url), "utf8");
  const statisticsTooltipSource = legacyGameOverviewSource.match(/legacy-statistics-tooltip[\s\S]{0,500}/)?.[0] ?? "";
  cases.push(finalize({
    case: "go_react_assets",
    checks: [
      check(js.status === 200, "React JS bundle returns HTTP 200", { status: js.status }),
      check(css.status === 200, "React CSS bundle returns HTTP 200", { status: css.status }),
      check(hasHeader(js, "cache-control", "immutable"), "React JS bundle is immutable-cacheable"),
      check(hasHeader(css, "cache-control", "immutable"), "React CSS bundle is immutable-cacheable"),
      check(hasHeader(js, "content-type", "javascript"), "React JS bundle has JavaScript content type"),
      check(hasHeader(css, "content-type", "text/css"), "React CSS bundle has CSS content type"),
      check(js.body.includes("/register") && js.body.includes("/universes"), "React bundle contains natural public route model"),
      check(js.body.includes("/api/public/universes"), "React bundle consumes universe catalog API"),
      check(js.body.includes("/api/public/registration"), "React bundle consumes registration creation API"),
      check(js.body.includes("/api/public/login"), "React bundle consumes login submit API"),
      check(js.body.includes("/api/game/overview"), "React bundle consumes game overview API"),
      check(js.body.includes("/api/game/buildings"), "React bundle consumes game buildings API"),
      check(js.body.includes("/api/game/empire"), "React bundle consumes game empire API"),
      check(js.body.includes("sandybrown") && js.body.includes("magenta"), "React bundle contains legacy empire queue marker colors"),
      check(js.body.includes("/api/game/resources"), "React bundle consumes game resources API"),
      check(js.body.includes("/api/game/merchant"), "React bundle consumes game merchant API"),
      check(js.body.includes("/api/game/officers"), "React bundle consumes game officers API"),
      check(js.body.includes("/api/game/admin"), "React bundle consumes game admin API"),
      check(js.body.includes("/api/game/research"), "React bundle consumes game research API"),
      check(js.body.includes("/api/game/shipyard"), "React bundle consumes game shipyard API"),
      check(js.body.includes("/api/game/fleet"), "React bundle consumes game fleet API"),
      check(js.body.includes("/api/game/fleet-templates"), "React bundle consumes game fleet templates API"),
      check(js.body.includes("/api/game/galaxy"), "React bundle consumes game galaxy API"),
      check(js.body.includes("/api/game/defense"), "React bundle consumes game defense API"),
      check(js.body.includes("/api/game/technology"), "React bundle consumes game technology API"),
      check(js.body.includes("/api/game/statistics"), "React bundle consumes game statistics API"),
      check(js.body.includes("/api/game/search"), "React bundle consumes game search API"),
      check(js.body.includes("/api/game/buddy"), "React bundle consumes game buddy API"),
      check(js.body.includes("/api/game/notes"), "React bundle consumes game notes API"),
      check(js.body.includes("/api/game/messages"), "React bundle consumes game messages API"),
      check(js.body.includes("/api/game/report"), "React bundle consumes game report API"),
      check(js.body.includes("/api/game/options"), "React bundle consumes game options API"),
      check(js.body.includes("/api/game/logout"), "React bundle consumes game logout API"),
      check(js.body.includes("legacy-public-main"), "React bundle contains legacy public home layout"),
      check(js.body.includes("legacy-public-register-panel"), "React bundle contains legacy public registration layout"),
      check(js.body.includes("legacy-public-about-panel"), "React bundle contains legacy public about layout"),
      check(js.body.includes("legacy-public-story-panel"), "React bundle contains legacy public story layout"),
      check(js.body.includes("legacy-public-screenshots-panel"), "React bundle contains legacy public screenshots layout"),
      check(js.body.includes("legacy-public-rules-panel"), "React bundle contains legacy public rules layout"),
      check(js.body.includes("legacy-legal-page"), "React bundle contains legacy legal layout"),
      check(js.body.includes("legacy-public-universes-panel"), "React bundle contains legacy public universes layout"),
      check(js.body.includes("legacy-game-shell"), "React bundle contains legacy game overview layout"),
      check(js.body.includes("legacy-buildings-table"), "React bundle contains legacy game buildings layout"),
      check(js.body.includes("legacy-resources-table"), "React bundle contains legacy game resources layout"),
      check(js.body.includes("legacy-merchant-call-table"), "React bundle contains legacy game merchant layout"),
      check(js.body.includes("legacy-officers-table"), "React bundle contains legacy game officers layout"),
      check(js.body.includes("legacy-admin-home-table"), "React bundle contains legacy game admin layout"),
      check(js.body.includes("legacy-admin-bans-table"), "React bundle contains legacy game admin bans layout"),
      check(js.body.includes("legacy-admin-broadcast-table"), "React bundle contains legacy game admin broadcast layout"),
      check(js.body.includes("legacy-admin-reports-table"), "React bundle contains legacy game admin reports layout"),
      check(js.body.includes("legacy-admin-bots-table"), "React bundle contains legacy game admin bots layout"),
      check(js.body.includes("legacy-admin-coupons-table"), "React bundle contains legacy game admin coupons layout"),
      check(js.body.includes("legacy-admin-colony-settings-table"), "React bundle contains legacy game admin colony settings layout"),
      check(js.body.includes("legacy-admin-debug-table"), "React bundle contains legacy game admin debug layout"),
      check(js.body.includes("legacy-admin-errors-table"), "React bundle contains legacy game admin errors layout"),
      check(js.body.includes("legacy-admin-logins-table"), "React bundle contains legacy game admin logins layout"),
      check(js.body.includes("legacy-admin-userlogs-table"), "React bundle contains legacy game admin user logs layout"),
      check(js.body.includes("legacy-admin-browse-table"), "React bundle contains legacy game admin browse layout"),
      check(js.body.includes("legacy-admin-fleetlogs-table"), "React bundle contains legacy game admin fleetlogs layout"),
      check(js.body.includes("legacy-admin-queue-table"), "React bundle contains legacy game admin queue layout"),
      check(js.body.includes("legacy-admin-users-table"), "React bundle contains legacy game admin users layout"),
      check(js.body.includes("legacy-admin-planets-table"), "React bundle contains legacy game admin planets layout"),
      check(js.body.includes("legacy-admin-universe-table"), "React bundle contains legacy game admin universe layout"),
      check(js.body.includes("legacy-admin-checksum-table"), "React bundle contains legacy game admin checksum layout"),
      check(js.body.includes("legacy-admin-db-table"), "React bundle contains legacy game admin database layout"),
      check(js.body.includes("legacy-admin-battlesim-table"), "React bundle contains legacy game admin battle simulator layout"),
      check(js.body.includes("legacy-admin-expedition-table"), "React bundle contains legacy game admin expedition layout"),
      check(js.body.includes("legacy-admin-battle-report-table"), "React bundle contains legacy game admin battle report layout"),
      check(js.body.includes("legacy-admin-botedit-table"), "React bundle contains legacy game admin bot editor layout"),
      check(js.body.includes("legacy-admin-raksim-table"), "React bundle contains legacy game admin missile simulator layout"),
      check(js.body.includes("legacy-admin-loca-table"), "React bundle contains legacy game admin localization layout"),
      check(js.body.includes("legacy-admin-mods-table"), "React bundle contains legacy game admin mods layout"),
      check(js.body.includes("legacy-buddy-table"), "React bundle contains legacy game buddy layout"),
      check(js.body.includes("legacy-research-table"), "React bundle contains legacy game research layout"),
      check(js.body.includes("legacy-shipyard-table"), "React bundle contains legacy game shipyard layout"),
      check(js.body.includes("legacy-fleet-table"), "React bundle contains legacy game fleet active missions layout"),
      check(js.body.includes("legacy-fleet-select-table"), "React bundle contains legacy game fleet ship selection layout"),
      check(js.body.includes("legacy-fleet-dispatch-table"), "React bundle contains legacy game fleet dispatch preview layout"),
      check(js.body.includes("legacy-fleet-dispatch-form") && js.body.includes("remainingresources"), "React bundle contains legacy fleet mission/resource draft layout"),
      check(legacyGameOverviewSource.includes("legacyFleetFlightTime(") && legacyGameOverviewSource.includes("legacyFleetDisplayConsumption("), "React source contains legacy fleet flight math draft layout"),
      check(js.body.includes("launch-dispatch"), "React bundle contains legacy fleet final launch action"),
      check(js.body.includes("legacy-fleet-templates-table"), "React bundle contains legacy game standard fleets layout"),
      check(js.body.includes("legacy-galaxy-table"), "React bundle contains legacy game galaxy layout"),
      check(js.body.includes("target_galaxy") && js.body.includes("target_mission"), "React bundle preserves legacy fleet target prefill fields"),
      check(js.body.includes("data-galaxy-action") && js.body.includes("/game/buddy"), "React bundle contains migrated galaxy action links"),
      check(js.body.includes("legacy-galaxy-tooltip") && js.body.includes("data-galaxy-instant"), "React bundle contains legacy galaxy hover action menus"),
      check(js.body.includes("legacy-defense-table"), "React bundle contains legacy game defense layout"),
      check(js.body.includes("legacy-technology-table"), "React bundle contains legacy game technology layout"),
      check(js.body.includes("legacy-technology-details-table"), "React bundle contains legacy game technology details layout"),
      check(js.body.includes("legacy-statistics-table"), "React bundle contains legacy game statistics layout"),
      check(statisticsTooltipSource.includes("legacy-statistics-tooltip") && !statisticsTooltipSource.includes("overlib("), "React source scopes statistics tooltip without global overlib handlers"),
      check(js.body.includes("legacy-search-results-table"), "React bundle contains legacy game search layout"),
      check(js.body.includes("legacy-messages-table"), "React bundle contains legacy game messages layout"),
      check(js.body.includes("legacy-messages-compose-table"), "React bundle contains legacy game message compose layout"),
      check(js.body.includes("legacy-report-table"), "React bundle contains legacy game report layout"),
      check(js.body.includes("legacy-options-table"), "React bundle contains legacy game options layout"),
      check(js.body.includes("legacy-notes-table"), "React bundle contains legacy game notes layout"),
      check(js.body.includes("legacy-notes-form-table"), "React bundle contains legacy game notes form layout"),
      check(js.body.includes("legacy-logout-table"), "React bundle contains legacy game logout layout")
    ]
  }));

  const legacyImage = await request("/legacy-assets/use/uV/planeten/small/s_normaltempplanet01.jpg");
  const legacyDir = await request("/legacy-assets/");
  cases.push(finalize({
    case: "go_legacy_assets",
    checks: [
      check(legacyImage.status === 200, "legacy planet image returns HTTP 200", { status: legacyImage.status }),
      check(hasHeader(legacyImage, "content-type", "image/jpeg"), "legacy planet image has JPEG content type"),
      check(legacyDir.status === 404, "legacy asset directory listing is disabled", { status: legacyDir.status })
    ]
  }));

  const postHealth = await request("/api/healthz", { method: "POST" });
  const getRegistrationValidation = await request("/api/public/registration/validate");
  const getRegistration = await request("/api/public/registration");
  const postActivation = await request("/game/validate.php?ack=missing", { method: "POST" });
  const getLoginValidation = await request("/api/public/login/validate");
  const getLoginSubmit = await request("/api/public/login");
  const postGameSession = await request("/api/game/session", { method: "POST" });
  const putGameOverview = await request("/api/game/overview", { method: "PUT" });
  const putGameBuildings = await request("/api/game/buildings", { method: "PUT" });
  const postGameEmpire = await request("/api/game/empire", { method: "POST" });
  const putGameResearch = await request("/api/game/research", { method: "PUT" });
  const putGameShipyard = await request("/api/game/shipyard", { method: "PUT" });
  const putGameFleet = await request("/api/game/fleet", { method: "PUT" });
  const putGameFleetTemplates = await request("/api/game/fleet-templates", { method: "PUT" });
  const putGameGalaxy = await request("/api/game/galaxy", { method: "PUT" });
  const putGameDefense = await request("/api/game/defense", { method: "PUT" });
  const postGameTechnology = await request("/api/game/technology", { method: "POST" });
  const postGameStatistics = await request("/api/game/statistics", { method: "POST" });
  const postGameSearch = await request("/api/game/search", { method: "POST" });
  const putGameBuddy = await request("/api/game/buddy", { method: "PUT" });
  const putGameNotes = await request("/api/game/notes", { method: "PUT" });
  const putGameMessages = await request("/api/game/messages", { method: "PUT" });
  const putGameReport = await request("/api/game/report", { method: "PUT" });
  const putGameOptions = await request("/api/game/options", { method: "PUT" });
  const putGameMerchant = await request("/api/game/merchant", { method: "PUT" });
  const putGameOfficers = await request("/api/game/officers", { method: "PUT" });
  const putGameAdmin = await request("/api/game/admin", { method: "PUT" });
  const getGameLogout = await request("/api/game/logout");
  const putGameResources = await request("/api/game/resources", { method: "PUT" });
  cases.push(finalize({
    case: "go_method_guards",
    checks: [
      check(postHealth.status === 405, "POST health endpoint is rejected", { status: postHealth.status }),
      check(hasHeader(postHealth, "allow", "GET, HEAD"), "method rejection returns Allow header"),
      check(getRegistrationValidation.status === 405, "GET registration validation endpoint is rejected", { status: getRegistrationValidation.status }),
      check(hasHeader(getRegistrationValidation, "allow", "POST"), "registration validation method rejection returns Allow header"),
      check(getRegistration.status === 405, "GET registration creation endpoint is rejected", { status: getRegistration.status }),
      check(hasHeader(getRegistration, "allow", "POST"), "registration creation method rejection returns Allow header"),
      check(postActivation.status === 405, "POST registration activation endpoint is rejected", { status: postActivation.status }),
      check(hasHeader(postActivation, "allow", "GET, HEAD"), "registration activation method rejection returns Allow header"),
      check(getLoginValidation.status === 405, "GET login validation endpoint is rejected", { status: getLoginValidation.status }),
      check(hasHeader(getLoginValidation, "allow", "POST"), "login validation method rejection returns Allow header"),
      check(getLoginSubmit.status === 405, "GET login submit endpoint is rejected", { status: getLoginSubmit.status }),
      check(hasHeader(getLoginSubmit, "allow", "POST"), "login submit method rejection returns Allow header"),
      check(postGameSession.status === 405, "POST game session endpoint is rejected", { status: postGameSession.status }),
      check(hasHeader(postGameSession, "allow", "GET, HEAD"), "game session method rejection returns Allow header"),
      check(putGameOverview.status === 405, "PUT game overview endpoint is rejected", { status: putGameOverview.status }),
      check(hasHeader(putGameOverview, "allow", "GET, HEAD, POST"), "game overview method rejection returns Allow header"),
      check(putGameBuildings.status === 405, "PUT game buildings endpoint is rejected", { status: putGameBuildings.status }),
      check(hasHeader(putGameBuildings, "allow", "GET, HEAD, POST"), "game buildings method rejection returns Allow header"),
      check(postGameEmpire.status === 405, "POST game empire endpoint is rejected", { status: postGameEmpire.status }),
      check(hasHeader(postGameEmpire, "allow", "GET, HEAD"), "game empire method rejection returns Allow header"),
      check(putGameResearch.status === 405, "PUT game research endpoint is rejected", { status: putGameResearch.status }),
      check(hasHeader(putGameResearch, "allow", "GET, HEAD, POST"), "game research method rejection returns Allow header"),
      check(putGameShipyard.status === 405, "PUT game shipyard endpoint is rejected", { status: putGameShipyard.status }),
      check(hasHeader(putGameShipyard, "allow", "GET, HEAD, POST"), "game shipyard method rejection returns Allow header"),
      check(putGameFleet.status === 405, "PUT game fleet endpoint is rejected", { status: putGameFleet.status }),
      check(hasHeader(putGameFleet, "allow", "GET, HEAD, POST"), "game fleet method rejection returns Allow header"),
      check(putGameFleetTemplates.status === 405, "PUT game fleet templates endpoint is rejected", { status: putGameFleetTemplates.status }),
      check(hasHeader(putGameFleetTemplates, "allow", "GET, HEAD, POST"), "game fleet templates method rejection returns Allow header"),
      check(putGameGalaxy.status === 405, "PUT game galaxy endpoint is rejected", { status: putGameGalaxy.status }),
      check(hasHeader(putGameGalaxy, "allow", "GET, HEAD, POST"), "game galaxy method rejection returns Allow header"),
      check(putGameDefense.status === 405, "PUT game defense endpoint is rejected", { status: putGameDefense.status }),
      check(hasHeader(putGameDefense, "allow", "GET, HEAD, POST"), "game defense method rejection returns Allow header"),
      check(postGameTechnology.status === 405, "POST game technology endpoint is rejected", { status: postGameTechnology.status }),
      check(hasHeader(postGameTechnology, "allow", "GET, HEAD"), "game technology method rejection returns Allow header"),
      check(postGameStatistics.status === 405, "POST game statistics endpoint is rejected", { status: postGameStatistics.status }),
      check(hasHeader(postGameStatistics, "allow", "GET, HEAD"), "game statistics method rejection returns Allow header"),
      check(postGameSearch.status === 405, "POST game search endpoint is rejected", { status: postGameSearch.status }),
      check(hasHeader(postGameSearch, "allow", "GET, HEAD"), "game search method rejection returns Allow header"),
      check(putGameBuddy.status === 405, "PUT game buddy endpoint is rejected", { status: putGameBuddy.status }),
      check(hasHeader(putGameBuddy, "allow", "GET, HEAD, POST"), "game buddy method rejection returns Allow header"),
      check(putGameNotes.status === 405, "PUT game notes endpoint is rejected", { status: putGameNotes.status }),
      check(hasHeader(putGameNotes, "allow", "GET, HEAD, POST"), "game notes method rejection returns Allow header"),
      check(putGameMessages.status === 405, "PUT game messages endpoint is rejected", { status: putGameMessages.status }),
      check(hasHeader(putGameMessages, "allow", "GET, HEAD, POST"), "game messages method rejection returns Allow header"),
      check(putGameReport.status === 405, "PUT game report endpoint is rejected", { status: putGameReport.status }),
      check(hasHeader(putGameReport, "allow", "GET, HEAD"), "game report method rejection returns Allow header"),
      check(putGameOptions.status === 405, "PUT game options endpoint is rejected", { status: putGameOptions.status }),
      check(hasHeader(putGameOptions, "allow", "GET, HEAD, POST"), "game options method rejection returns Allow header"),
      check(putGameMerchant.status === 405, "PUT game merchant endpoint is rejected", { status: putGameMerchant.status }),
      check(hasHeader(putGameMerchant, "allow", "GET, HEAD, POST"), "game merchant method rejection returns Allow header"),
      check(putGameOfficers.status === 405, "PUT game officers endpoint is rejected", { status: putGameOfficers.status }),
      check(hasHeader(putGameOfficers, "allow", "GET, HEAD, POST"), "game officers method rejection returns Allow header"),
      check(putGameAdmin.status === 405, "PUT game admin endpoint is rejected", { status: putGameAdmin.status }),
      check(hasHeader(putGameAdmin, "allow", "GET, HEAD, POST"), "game admin method rejection returns Allow header"),
      check(getGameLogout.status === 405, "GET game logout endpoint is rejected", { status: getGameLogout.status }),
      check(hasHeader(getGameLogout, "allow", "POST"), "game logout method rejection returns Allow header"),
      check(putGameResources.status === 405, "PUT game resources endpoint is rejected", { status: putGameResources.status }),
      check(hasHeader(putGameResources, "allow", "GET, HEAD, POST"), "game resources method rejection returns Allow header")
    ]
  }));

  const missingActivation = await request("/game/validate.php");
  const naturalMissingActivation = await request("/activation?ack=missing");
  cases.push(finalize({
    case: "go_registration_activation_route",
    checks: [
      check(missingActivation.status === 302, "legacy activation without ack redirects", { status: missingActivation.status }),
      check(missingActivation.headers.location === "/home", "legacy activation without ack returns home location", { location: missingActivation.headers.location }),
      check(naturalMissingActivation.status === 302, "natural activation with missing ack redirects", { status: naturalMissingActivation.status }),
      check(naturalMissingActivation.headers.location === "/home", "natural activation missing account returns home location", { location: naturalMissingActivation.headers.location })
    ]
  }));
} catch (error) {
  cases.push(finalize({
    case: "go_compat_smoke_runtime",
    checks: [
      check(false, "Go compatibility smoke did not complete", {
        error: error instanceof Error ? error.message : String(error),
        stack: error instanceof Error ? error.stack : undefined
      })
    ]
  }));
}

const result = {
  case_group: "golang_compat_smoke",
  base_url: baseUrl,
  cases,
  all_pass: cases.every((item) => item.pass === true)
};

const output = process.env.OGAME_SMOKE_COMPACT === "1"
  ? {
      case_group: result.case_group,
      base_url: result.base_url,
      all_pass: result.all_pass,
      failed: result.cases
        .filter((testCase) => testCase.pass !== true)
        .map((testCase) => ({
          case: testCase.case,
          checks: testCase.checks.filter((item) => item.pass !== true)
        }))
    }
  : result;

process.stdout.write(`${JSON.stringify(output, null, 2)}\n`);
if (!result.all_pass) {
  process.exitCode = 1;
}
