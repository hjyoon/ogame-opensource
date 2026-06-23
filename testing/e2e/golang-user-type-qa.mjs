import { readFileSync } from "node:fs";

const baseUrl = (process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890").replace(/\/+$/, "");
const fixturePath = process.env.OGAME_USER_TYPE_FIXTURE_FILE ?? ".tmp/golang-user-type-fixture.json";
const fixture = JSON.parse(process.env.OGAME_USER_TYPE_FIXTURE_JSON ?? readFileSync(fixturePath, "utf8"));
const password = fixture.password ?? "qa-type-pass";
const users = fixture.users ?? {};

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

function parseJSON(response) {
  try {
    return JSON.parse(response.body);
  } catch {
    return {};
  }
}

function issueCodes(payload) {
  return Array.isArray(payload.issues) ? payload.issues.map((issue) => issue.code) : [];
}

function menuHas(payload, mode) {
  return Array.isArray(payload.admin?.menu) && payload.admin.menu.some((item) => item.mode === mode);
}

async function login(role, universe) {
  const user = users[role];
  const response = await request("/api/public/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      login: user?.login ?? "",
      pass: password,
      universe
    })
  });
  const body = parseJSON(response);
  const cookie = response.headers["set-cookie"] ?? "";
  const cookiePair = cookie.split(";")[0] ?? "";
  const sessionSearch = typeof body.session?.redirectTo === "string"
    ? new URL(body.session.redirectTo, baseUrl).search
    : "?session=";
  return { user, response, body, cookiePair, sessionSearch };
}

async function authedJSON(auth, path, options = {}) {
  const response = await request(`${path}${auth.sessionSearch}`, {
    ...options,
    headers: {
      Cookie: auth.cookiePair,
      ...(options.headers ?? {})
    }
  });
  return { response, body: parseJSON(response) };
}

async function authedAdminJSON(auth, mode) {
  const separator = auth.sessionSearch.includes("?") ? "&" : "?";
  const response = await request(`/api/game/admin${auth.sessionSearch}${separator}mode=${encodeURIComponent(mode)}`, {
    headers: { Cookie: auth.cookiePair }
  });
  return { response, body: parseJSON(response) };
}

const cases = [];

try {
  const catalog = await request("/api/public/universes");
  const catalogBody = parseJSON(catalog);
  const universe = catalogBody.universes?.[0]?.baseUrl ?? "http://localhost:8888";

  const player = await login("player", universe);
  const playerSession = await authedJSON(player, "/api/game/session");
  const playerAdmin = await authedAdminJSON(player, "Users");
  const playerOptions = await authedJSON(player, "/api/game/options");
  const playerVacation = await authedJSON(player, "/api/game/options", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      lang: playerOptions.body.options?.settings?.language ?? "en",
      dpath: playerOptions.body.options?.settings?.skinPath ?? "/evolution/",
      design: "on",
      noipcheck: "on",
      settings_sort: String(playerOptions.body.options?.settings?.sortBy ?? 0),
      settings_order: String(playerOptions.body.options?.settings?.sortOrder ?? 0),
      spio_anz: String(playerOptions.body.options?.settings?.maxSpy ?? 1),
      settings_fleetactions: String(playerOptions.body.options?.settings?.maxFleetMessages ?? 3),
      urlaubs_modus: "on"
    }).toString()
  });
  cases.push(finalize({
    case: "regular_player_type",
    checks: [
      check(player.response.status === 200 && player.body.valid === true, "regular player can log in", player.body),
      check(player.cookiePair.startsWith(`prsess_${users.player?.player_id ?? ""}_`), "regular player receives private session cookie", { cookie: player.cookiePair }),
      check(playerSession.response.status === 200 && playerSession.body.authenticated === true, "regular player session authenticates", playerSession.body),
      check(playerAdmin.response.status === 200 && playerAdmin.body.authenticated === true, "regular player admin API request stays authenticated", playerAdmin.body),
      check(playerAdmin.body.admin?.viewer?.level === 0, "regular player keeps admin level 0", playerAdmin.body.admin?.viewer ?? {}),
      check(playerAdmin.body.actionIssue?.code === "access_denied", "regular player is denied admin mode access", playerAdmin.body.actionIssue ?? {}),
      check(!Array.isArray(playerAdmin.body.admin?.userRows) || playerAdmin.body.admin.userRows.length === 0, "regular player does not receive admin user rows", playerAdmin.body.admin ?? {}),
      check(playerVacation.response.status === 200 && playerVacation.body.authenticated === true, "regular player vacation options POST authenticates", playerVacation.body),
      check(playerVacation.body.actionIssue?.code === "vacation_enabled", "regular player can enable vacation mode from options", playerVacation.body.actionIssue ?? {}),
      check(playerVacation.body.options?.account?.vacation === true && playerVacation.body.options.account.vacationUntil > Math.floor(Date.now() / 1000), "vacation enable stores a future minimum vacation timestamp", playerVacation.body.options?.account ?? {})
    ]
  }));

  const operator = await login("operator", universe);
  const operatorHome = await authedAdminJSON(operator, "Home");
  const operatorUsers = await authedAdminJSON(operator, "Users");
  const operatorBotEdit = await authedAdminJSON(operator, "BotEdit");
  cases.push(finalize({
    case: "operator_type",
    checks: [
      check(operator.response.status === 200 && operator.body.valid === true, "operator can log in", operator.body),
      check(operatorHome.body.admin?.viewer?.level === 1, "operator keeps admin level 1", operatorHome.body.admin?.viewer ?? {}),
      check(operatorHome.body.actionIssue === undefined, "operator can open admin home", operatorHome.body.actionIssue ?? {}),
      check(operatorUsers.body.actionIssue === undefined, "operator can open standard Users admin mode", operatorUsers.body.actionIssue ?? {}),
      check(Array.isArray(operatorUsers.body.admin?.userRows), "operator receives Users mode rows for read-only review", operatorUsers.body.admin ?? {}),
      check(menuHas(operatorHome.body, "BotEdit"), "operator menu keeps legacy BotEdit icon even though mode is restricted", operatorHome.body.admin ?? {}),
      check(operatorBotEdit.body.actionIssue?.code === "access_denied", "operator is denied admin-only BotEdit data", operatorBotEdit.body.actionIssue ?? {}),
      check(!Array.isArray(operatorBotEdit.body.admin?.botStrategies) || operatorBotEdit.body.admin.botStrategies.length === 0, "operator does not receive bot strategy rows", operatorBotEdit.body.admin ?? {})
    ]
  }));

  const admin = await login("admin", universe);
  const adminUni = await authedAdminJSON(admin, "Uni");
  const adminBotEdit = await authedAdminJSON(admin, "BotEdit");
  cases.push(finalize({
    case: "administrator_type",
    checks: [
      check(admin.response.status === 200 && admin.body.valid === true, "administrator can log in", admin.body),
      check(adminUni.body.admin?.viewer?.level === 2, "administrator keeps admin level 2", adminUni.body.admin?.viewer ?? {}),
      check(adminUni.body.actionIssue === undefined, "administrator can open Universe Settings", adminUni.body.actionIssue ?? {}),
      check(typeof adminUni.body.admin?.universe?.number === "number", "administrator receives universe settings payload", adminUni.body.admin?.universe ?? {}),
      check(adminBotEdit.body.actionIssue === undefined, "administrator can open BotEdit mode", adminBotEdit.body.actionIssue ?? {}),
      check(menuHas(adminUni.body, "BotEdit") && menuHas(adminUni.body, "Uni"), "administrator receives full admin menu", adminUni.body.admin ?? {})
    ]
  }));

  const unvalidated = await login("unvalidated", universe);
  const unvalidatedOptions = await authedJSON(unvalidated, "/api/game/options");
  const unvalidatedNewEmail = `qa-type-unvalidated-${Date.now()}@example.local`;
  const unvalidatedEmailChange = await authedJSON(unvalidated, "/api/game/options", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      language: unvalidatedOptions.body.options?.settings?.language ?? "en",
      skinPath: unvalidatedOptions.body.options?.settings?.skinPath ?? "/evolution/",
      useSkin: unvalidatedOptions.body.options?.settings?.useSkin ?? true,
      deactivateIp: true,
      sortBy: unvalidatedOptions.body.options?.settings?.sortBy ?? 0,
      sortOrder: unvalidatedOptions.body.options?.settings?.sortOrder ?? 0,
      maxSpy: unvalidatedOptions.body.options?.settings?.maxSpy ?? 1,
      maxFleetMessages: unvalidatedOptions.body.options?.settings?.maxFleetMessages ?? 3,
      oldPassword: password,
      email: unvalidatedNewEmail,
      vacationMode: false,
      deleteAccount: false
    })
  });
  cases.push(finalize({
    case: "unvalidated_user_type",
    checks: [
      check(unvalidated.response.status === 200 && unvalidated.body.valid === true, "unvalidated user can still log in like legacy", unvalidated.body),
      check(unvalidatedOptions.response.status === 200 && unvalidatedOptions.body.authenticated === true, "unvalidated user options request authenticates", unvalidatedOptions.body),
      check(unvalidatedOptions.body.options?.user?.validated === false, "options exposes unvalidated account state", unvalidatedOptions.body.options?.user ?? {}),
      check(unvalidatedEmailChange.body.actionIssue?.code === "email_changed", "unvalidated user can update pending email from options", unvalidatedEmailChange.body.actionIssue ?? {}),
      check(unvalidatedEmailChange.body.options?.user?.email === unvalidatedNewEmail && unvalidatedEmailChange.body.options?.user?.validated === false, "pending email stays unvalidated after options email change", unvalidatedEmailChange.body.options?.user ?? {})
    ]
  }));

  const vacation = await login("vacation", universe);
  const vacationSession = await authedJSON(vacation, "/api/game/session");
  const vacationOptions = await authedJSON(vacation, "/api/game/options");
  const vacationBuild = await authedJSON(vacation, "/api/game/buildings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "add", techId: 1 })
  });
  cases.push(finalize({
    case: "vacation_user_type",
    checks: [
      check(vacation.response.status === 200 && vacation.body.valid === true, "vacation user can log in", vacation.body),
      check(vacationSession.body.session?.vacationMode === true, "session exposes vacation mode", vacationSession.body.session ?? {}),
      check(vacationOptions.body.options?.account?.vacation === true, "options exposes vacation mode", vacationOptions.body.options?.account ?? {}),
      check(vacationBuild.body.actionIssue?.code === "vacation", "vacation mode blocks building mutation", vacationBuild.body.actionIssue ?? {})
    ]
  }));

  const banned = await login("banned", universe);
  cases.push(finalize({
    case: "banned_user_type",
    checks: [
      check(banned.response.status === 200, "banned login returns legacy-compatible HTTP 200", { status: banned.response.status }),
      check(banned.body.valid === false, "banned user cannot create a login session", banned.body),
      check(issueCodes(banned.body).includes("user_banned"), "banned login returns user_banned issue", banned.body),
      check(!banned.cookiePair.startsWith("prsess_"), "banned login does not set private session cookie", { cookie: banned.cookiePair })
    ]
  }));

  const deletionQueued = await login("deletion_queued", universe);
  const deletionSession = await authedJSON(deletionQueued, "/api/game/session");
  const deletionOptions = await authedJSON(deletionQueued, "/api/game/options");
  cases.push(finalize({
    case: "deletion_queued_user_type",
    checks: [
      check(deletionQueued.response.status === 200 && deletionQueued.body.valid === true, "deletion-queued user can log in before cleanup", deletionQueued.body),
      check(deletionSession.body.session?.deletionQueued === true, "session exposes deletion queue state", deletionSession.body.session ?? {}),
      check(deletionOptions.body.options?.account?.deletionQueued === true, "options exposes deletion queue state", deletionOptions.body.options?.account ?? {})
    ]
  }));

  const credentials = await login("credentials", universe);
  const credentialsOptions = await authedJSON(credentials, "/api/game/options");
  const credentialsNextPassword = "qatypepass2";
  const credentialsPasswordChange = await authedJSON(credentials, "/api/game/options", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      language: credentialsOptions.body.options?.settings?.language ?? "en",
      skinPath: credentialsOptions.body.options?.settings?.skinPath ?? "/evolution/",
      useSkin: credentialsOptions.body.options?.settings?.useSkin ?? true,
      deactivateIp: true,
      sortBy: credentialsOptions.body.options?.settings?.sortBy ?? 0,
      sortOrder: credentialsOptions.body.options?.settings?.sortOrder ?? 0,
      maxSpy: credentialsOptions.body.options?.settings?.maxSpy ?? 1,
      maxFleetMessages: credentialsOptions.body.options?.settings?.maxFleetMessages ?? 3,
      oldPassword: password,
      newPassword: credentialsNextPassword,
      newPasswordRepeat: credentialsNextPassword,
      email: credentialsOptions.body.options?.user?.plainEmail ?? "",
      vacationMode: false,
      deleteAccount: false
    })
  });
  const credentialsNewLogin = await request("/api/public/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      login: credentials.user?.login ?? "",
      pass: credentialsNextPassword,
      universe
    })
  });
  const credentialsNewLoginBody = parseJSON(credentialsNewLogin);
  cases.push(finalize({
    case: "options_credentials",
    checks: [
      check(credentials.response.status === 200 && credentials.body.valid === true, "credentials fixture user can log in", credentials.body),
      check(credentialsPasswordChange.body.actionIssue?.code === "password_changed", "options password change succeeds with old password", credentialsPasswordChange.body.actionIssue ?? {}),
      check(credentialsNewLoginBody.valid === true, "new password can authenticate after options password change", credentialsNewLoginBody)
    ]
  }));
} catch (error) {
  cases.push({
    case: "golang_user_type_qa_exception",
    checks: [check(false, String(error), { stack: error?.stack ?? "" })],
    pass: false
  });
}

const allPass = cases.every((testCase) => testCase.pass === true);
console.log(JSON.stringify({
  case_group: "golang_user_type_qa",
  base: baseUrl,
  fixture: fixturePath,
  cases,
  all_pass: allPass
}, null, 2));
if (!allPass) {
  process.exitCode = 1;
}
