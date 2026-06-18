import { publicRouteAliases, publicRoutes } from "../../frontend/src/routes.ts";

const baseUrl = (process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890").replace(/\/+$/, "");
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

function hasHeader(response, name, expected) {
  const actual = response.headers[name.toLowerCase()] ?? "";
  return expected === undefined ? actual !== "" : actual.toLowerCase().includes(expected.toLowerCase());
}

function noLoopbackAsset(body) {
  return !/(?:src|href|background)=["']https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?\//i.test(body);
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
  const createdRegistration = await request("/api/public/registration", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: `NewPilot${runId}`,
      password: registrationPassword,
      email: `new-pilot-${runId}@example.local`,
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
  cases.push(finalize({
    case: "go_registration_creation_api",
    checks: [
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
      check(createdOverviewBody.overview?.currentPlanet?.id === createdRegistrationBody.account?.homePlanetId, "created overview uses home planet", createdOverviewBody.overview?.currentPlanet ?? {})
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
      check(js.body.includes("/api/game/defense"), "React bundle consumes game defense API"),
      check(js.body.includes("/api/game/technology"), "React bundle consumes game technology API"),
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
      check(js.body.includes("legacy-research-table"), "React bundle contains legacy game research layout"),
      check(js.body.includes("legacy-shipyard-table"), "React bundle contains legacy game shipyard layout"),
      check(js.body.includes("legacy-fleet-table"), "React bundle contains legacy game fleet active missions layout"),
      check(js.body.includes("legacy-fleet-select-table"), "React bundle contains legacy game fleet ship selection layout"),
      check(js.body.includes("legacy-defense-table"), "React bundle contains legacy game defense layout"),
      check(js.body.includes("legacy-technology-table"), "React bundle contains legacy game technology layout"),
      check(js.body.includes("legacy-technology-details-table"), "React bundle contains legacy game technology details layout")
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
  const getLoginValidation = await request("/api/public/login/validate");
  const getLoginSubmit = await request("/api/public/login");
  const postGameSession = await request("/api/game/session", { method: "POST" });
  const postGameOverview = await request("/api/game/overview", { method: "POST" });
  const postGameBuildings = await request("/api/game/buildings", { method: "POST" });
  const postGameResearch = await request("/api/game/research", { method: "POST" });
  const postGameShipyard = await request("/api/game/shipyard", { method: "POST" });
  const postGameFleet = await request("/api/game/fleet", { method: "POST" });
  const postGameDefense = await request("/api/game/defense", { method: "POST" });
  const postGameTechnology = await request("/api/game/technology", { method: "POST" });
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
      check(getLoginValidation.status === 405, "GET login validation endpoint is rejected", { status: getLoginValidation.status }),
      check(hasHeader(getLoginValidation, "allow", "POST"), "login validation method rejection returns Allow header"),
      check(getLoginSubmit.status === 405, "GET login submit endpoint is rejected", { status: getLoginSubmit.status }),
      check(hasHeader(getLoginSubmit, "allow", "POST"), "login submit method rejection returns Allow header"),
      check(postGameSession.status === 405, "POST game session endpoint is rejected", { status: postGameSession.status }),
      check(hasHeader(postGameSession, "allow", "GET, HEAD"), "game session method rejection returns Allow header"),
      check(postGameOverview.status === 405, "POST game overview endpoint is rejected", { status: postGameOverview.status }),
      check(hasHeader(postGameOverview, "allow", "GET, HEAD"), "game overview method rejection returns Allow header"),
      check(postGameBuildings.status === 405, "POST game buildings endpoint is rejected", { status: postGameBuildings.status }),
      check(hasHeader(postGameBuildings, "allow", "GET, HEAD"), "game buildings method rejection returns Allow header"),
      check(postGameResearch.status === 405, "POST game research endpoint is rejected", { status: postGameResearch.status }),
      check(hasHeader(postGameResearch, "allow", "GET, HEAD"), "game research method rejection returns Allow header"),
      check(postGameShipyard.status === 405, "POST game shipyard endpoint is rejected", { status: postGameShipyard.status }),
      check(hasHeader(postGameShipyard, "allow", "GET, HEAD"), "game shipyard method rejection returns Allow header"),
      check(postGameFleet.status === 405, "POST game fleet endpoint is rejected", { status: postGameFleet.status }),
      check(hasHeader(postGameFleet, "allow", "GET, HEAD"), "game fleet method rejection returns Allow header"),
      check(postGameDefense.status === 405, "POST game defense endpoint is rejected", { status: postGameDefense.status }),
      check(hasHeader(postGameDefense, "allow", "GET, HEAD"), "game defense method rejection returns Allow header"),
      check(postGameTechnology.status === 405, "POST game technology endpoint is rejected", { status: postGameTechnology.status }),
      check(hasHeader(postGameTechnology, "allow", "GET, HEAD"), "game technology method rejection returns Allow header"),
      check(putGameResources.status === 405, "PUT game resources endpoint is rejected", { status: putGameResources.status }),
      check(hasHeader(putGameResources, "allow", "GET, HEAD, POST"), "game resources method rejection returns Allow header")
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
