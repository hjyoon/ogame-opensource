import { publicRouteAliases, publicRoutes } from "../../frontend/src/routes.ts";

const baseUrl = (process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890").replace(/\/+$/, "");
const mailhogBaseUrl = (process.env.OGAME_MAILHOG_BASE_URL ?? "http://127.0.0.1:8026").replace(/\/+$/, "");
const loginSmokeUser = process.env.OGAME_GO_LOGIN_SMOKE_USER ?? "legor";
const loginSmokePassword = process.env.OGAME_GO_LOGIN_SMOKE_PASS ?? "admin";

function check(pass, message, context = {}) {
  return { pass, message, context };
}

function finalize(testCase) {
  testCase.pass = testCase.checks.every((item) => item.pass === true);
  return testCase;
}

async function request(path, options = {}) {
  const response = await fetch(`${baseUrl}${path}`, {
    redirect: "manual",
    ...options
  });
  const headers = Object.fromEntries(response.headers.entries());
  const body = await response.text();
  return { status: response.status, headers, body };
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
  const createdOverview = createdRegistrationSession
    ? await request(`/api/game/overview?session=${encodeURIComponent(createdRegistrationSession)}`, {
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
  const welcomeActivationOverview = welcomeActivationSession
    ? await request(`/api/game/overview?session=${encodeURIComponent(welcomeActivationSession)}`, {
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

  const gameFleetWithoutCookie = await request(`/api/game/fleet${sessionSearch}`);
  let gameFleetWithoutCookieBody = {};
  try {
    gameFleetWithoutCookieBody = JSON.parse(gameFleetWithoutCookie.body);
  } catch {
    gameFleetWithoutCookieBody = {};
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
      check(!gameFleet.body.includes(sessionCookiePair), "game fleet response does not echo private cookie"),
      check(gameFleetWithoutCookie.status === 401, "game fleet rejects missing private cookie", { status: gameFleetWithoutCookie.status }),
      check(gameFleetWithoutCookieBody.authenticated === false, "game fleet missing private cookie is unauthenticated", gameFleetWithoutCookieBody),
      check(gameGalaxy.status === 200, "game galaxy returns HTTP 200 with private cookie", { status: gameGalaxy.status }),
      check(gameGalaxyBody.authenticated === true, "game galaxy authenticates the login session", gameGalaxyBody),
      check(Array.isArray(gameGalaxyBody.galaxy?.rows) && gameGalaxyBody.galaxy.rows.length === 15, "game galaxy returns 15 visible system rows", gameGalaxyBody),
      check(Number.isFinite(gameGalaxyBody.galaxy?.coordinates?.galaxy), "game galaxy returns selected galaxy coordinate", gameGalaxyBody),
      check(Number.isFinite(gameGalaxyBody.galaxy?.coordinates?.system), "game galaxy returns selected system coordinate", gameGalaxyBody),
      check(Number.isFinite(gameGalaxyBody.galaxy?.slots?.max), "game galaxy returns fleet slot summary", gameGalaxyBody),
      check(typeof gameGalaxyBody.galaxy?.extra?.commander === "boolean", "game galaxy returns commander extra info state", gameGalaxyBody),
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
      check(js.body.includes("/api/game/resources"), "React bundle consumes game resources API"),
      check(js.body.includes("/api/game/research"), "React bundle consumes game research API"),
      check(js.body.includes("/api/game/shipyard"), "React bundle consumes game shipyard API"),
      check(js.body.includes("/api/game/fleet"), "React bundle consumes game fleet API"),
      check(js.body.includes("/api/game/galaxy"), "React bundle consumes game galaxy API"),
      check(js.body.includes("/api/game/defense"), "React bundle consumes game defense API"),
      check(js.body.includes("/api/game/technology"), "React bundle consumes game technology API"),
      check(js.body.includes("/api/game/statistics"), "React bundle consumes game statistics API"),
      check(js.body.includes("/api/game/search"), "React bundle consumes game search API"),
      check(js.body.includes("/api/game/buddy"), "React bundle consumes game buddy API"),
      check(js.body.includes("/api/game/notes"), "React bundle consumes game notes API"),
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
      check(js.body.includes("legacy-buddy-table"), "React bundle contains legacy game buddy layout"),
      check(js.body.includes("legacy-research-table"), "React bundle contains legacy game research layout"),
      check(js.body.includes("legacy-shipyard-table"), "React bundle contains legacy game shipyard layout"),
      check(js.body.includes("legacy-fleet-table"), "React bundle contains legacy game fleet active missions layout"),
      check(js.body.includes("legacy-fleet-select-table"), "React bundle contains legacy game fleet ship selection layout"),
      check(js.body.includes("legacy-galaxy-table"), "React bundle contains legacy game galaxy layout"),
      check(js.body.includes("legacy-defense-table"), "React bundle contains legacy game defense layout"),
      check(js.body.includes("legacy-technology-table"), "React bundle contains legacy game technology layout"),
      check(js.body.includes("legacy-technology-details-table"), "React bundle contains legacy game technology details layout"),
      check(js.body.includes("legacy-statistics-table"), "React bundle contains legacy game statistics layout"),
      check(js.body.includes("legacy-search-results-table"), "React bundle contains legacy game search layout"),
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
  const postGameBuildings = await request("/api/game/buildings", { method: "POST" });
  const postGameResearch = await request("/api/game/research", { method: "POST" });
  const postGameShipyard = await request("/api/game/shipyard", { method: "POST" });
  const postGameFleet = await request("/api/game/fleet", { method: "POST" });
  const postGameGalaxy = await request("/api/game/galaxy", { method: "POST" });
  const postGameDefense = await request("/api/game/defense", { method: "POST" });
  const postGameTechnology = await request("/api/game/technology", { method: "POST" });
  const postGameStatistics = await request("/api/game/statistics", { method: "POST" });
  const postGameSearch = await request("/api/game/search", { method: "POST" });
  const putGameBuddy = await request("/api/game/buddy", { method: "PUT" });
  const putGameNotes = await request("/api/game/notes", { method: "PUT" });
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
      check(postGameBuildings.status === 405, "POST game buildings endpoint is rejected", { status: postGameBuildings.status }),
      check(hasHeader(postGameBuildings, "allow", "GET, HEAD"), "game buildings method rejection returns Allow header"),
      check(postGameResearch.status === 405, "POST game research endpoint is rejected", { status: postGameResearch.status }),
      check(hasHeader(postGameResearch, "allow", "GET, HEAD"), "game research method rejection returns Allow header"),
      check(postGameShipyard.status === 405, "POST game shipyard endpoint is rejected", { status: postGameShipyard.status }),
      check(hasHeader(postGameShipyard, "allow", "GET, HEAD"), "game shipyard method rejection returns Allow header"),
      check(postGameFleet.status === 405, "POST game fleet endpoint is rejected", { status: postGameFleet.status }),
      check(hasHeader(postGameFleet, "allow", "GET, HEAD"), "game fleet method rejection returns Allow header"),
      check(postGameGalaxy.status === 405, "POST game galaxy endpoint is rejected", { status: postGameGalaxy.status }),
      check(hasHeader(postGameGalaxy, "allow", "GET, HEAD"), "game galaxy method rejection returns Allow header"),
      check(postGameDefense.status === 405, "POST game defense endpoint is rejected", { status: postGameDefense.status }),
      check(hasHeader(postGameDefense, "allow", "GET, HEAD"), "game defense method rejection returns Allow header"),
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
        error: error instanceof Error ? error.message : String(error)
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

process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
if (!result.all_pass) {
  process.exitCode = 1;
}
