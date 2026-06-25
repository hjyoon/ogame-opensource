function envURL(value, fallback) {
  return String(value ?? fallback).replace(/\/+$/, "");
}

function argValue(name, fallback) {
  const index = Bun.argv.indexOf(name);
  if (index === -1 || index + 1 >= Bun.argv.length) {
    return fallback;
  }
  return Bun.argv[index + 1];
}

const publicRoutes = [
  { path: "/home" },
  { path: "/register" },
  { path: "/universes" },
  { path: "/about" },
  { path: "/story" },
  { path: "/screenshots" },
  { path: "/rules" },
  { path: "/legal" },
  { path: "/migration" }
];

const publicRouteAliases = new Map([
  ["/home.php", "/home"],
  ["/index.php", "/home"],
  ["/install.php", "/home"],
  ["/register.php", "/register"],
  ["/unis.php", "/universes"],
  ["/about.php", "/about"],
  ["/story.php", "/story"],
  ["/screenshots.php", "/screenshots"],
  ["/regeln.php", "/rules"],
  ["/impressum.php", "/legal"]
]);

const baseUrl = envURL(argValue("--go-base-url", process.env.OGAME_GO_BASE_URL), "http://127.0.0.1:8890");
const mailhogBaseUrl = envURL(argValue("--mailhog-base-url", process.env.OGAME_MAILHOG_BASE_URL), "http://127.0.0.1:8026");
const loginSmokeUser = argValue("--login-user", process.env.OGAME_GO_LOGIN_SMOKE_USER ?? "legor");
const loginSmokePassword = argValue("--login-pass", process.env.OGAME_GO_LOGIN_SMOKE_PASS ?? "admin");
const smokeFixtureFile = argValue("--fixture", process.env.OGAME_GO_SMOKE_FIXTURE_FILE ?? "");

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
    return JSON.parse(await Bun.file(path).text());
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

function legacyOptionsForm(values = {}) {
  const form = new URLSearchParams({
    lang: "en",
    dpath: "/evolution/",
    design: "on",
    settings_sort: "0",
    settings_order: "0",
    spio_anz: "1",
    settings_fleetactions: "3"
  });
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null) {
      form.set(key, String(value));
    }
  }
  return form.toString();
}

function noLoopbackAsset(body) {
  return !/(?:src|href|background)=["']https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?\//i.test(body);
}

function sameOriginAssetPath(documentPath, assetURL) {
  const raw = String(assetURL ?? "").trim();
  if (
    raw === "" ||
    raw.startsWith("#") ||
    /^(?:javascript|data|mailto):/i.test(raw)
  ) {
    return "";
  }
  try {
    const resolved = new URL(raw, new URL(documentPath, baseUrl));
    if (resolved.origin !== new URL(baseUrl).origin) {
      return "";
    }
    return `${resolved.pathname}${resolved.search}`;
  } catch {
    return "";
  }
}

function extractSameOriginAssets(documentPath, body) {
  const assets = new Set();
  const attrPattern = /\b(?:src|href)=["']([^"']+)["']/gi;
  const cssURLPattern = /url\(["']?([^)"']+)["']?\)/gi;
  for (const pattern of [attrPattern, cssURLPattern]) {
    let match;
    while ((match = pattern.exec(body)) !== null) {
      const path = sameOriginAssetPath(documentPath, match[1]);
      if (path) {
        assets.add(path);
      }
    }
  }
  return Array.from(assets).sort();
}

function looksLikeHTML(response) {
  const contentType = String(response.headers["content-type"] ?? "").toLowerCase();
  const bodyStart = response.body.trimStart().slice(0, 128).toLowerCase();
  return contentType.includes("text/html") || bodyStart.startsWith("<!doctype") || bodyStart.startsWith("<html");
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

function extractRecoveryPassword(body) {
  const match = /your password for .*? is:\s+([a-z0-9]+)\s+You may log in/is.exec(String(body ?? ""));
  return match?.[1] ?? "";
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

async function loginGameUser(login, pass, universe) {
  const response = await request("/api/public/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ login, pass, universe })
  });
  const body = parseJSON(response);
  const cookie = response.headers["set-cookie"] ?? "";
  const cookiePair = cookie.split(";")[0] ?? "";
  const cookieName = cookiePair.split("=")[0] ?? "";
  const playerId = Number(/^prsess_(\d+)_/.exec(cookieName)?.[1] ?? 0);
  const search = typeof body.session?.redirectTo === "string"
    ? new URL(body.session.redirectTo, baseUrl).search
    : "?session=";
  return { response, body, cookie, cookiePair, playerId, search };
}

const cases = [];

try {
  const runId = Date.now().toString(36);
  const smokeFixture = await readOptionalJSON(smokeFixtureFile);
  const phalanxFixture = smokeFixture?.phalanx ?? {};
  const adminQueueFixture = smokeFixture?.admin_queue ?? {};
  const adminQueueTaskId = Number(adminQueueFixture.task_id ?? 0);
  const adminQueueFixtureReady = adminQueueTaskId > 0;
  const adminFleetlogsFixture = smokeFixture?.admin_fleetlogs ?? {};
  const adminFleetlogsTaskId = Number(adminFleetlogsFixture.task_id ?? 0);
  const adminFleetlogsFixtureReady = adminFleetlogsTaskId > 0;
  const adminFleetlogsRecallTaskId = Number(adminFleetlogsFixture.recall_task_id ?? 0);
  const adminFleetlogsRecallFleetId = Number(adminFleetlogsFixture.recall_fleet_id ?? 0);
  const adminFleetlogsRecallFixtureReady = adminFleetlogsRecallTaskId > 0 && adminFleetlogsRecallFleetId > 0;
  const adminOperationsFixture = smokeFixture?.admin_operations ?? {};
  const adminOperationsReady =
    Number(adminOperationsFixture.report_id ?? 0) > 0 &&
    String(adminOperationsFixture.token ?? "") !== "" &&
    Number(adminOperationsFixture.operator_player_id ?? 0) > 0;
  const feedFixture = smokeFixture?.feed ?? {};
  const feedFixtureReady =
    typeof feedFixture.rss_feed_id === "string" &&
    typeof feedFixture.atom_feed_id === "string" &&
    Number(feedFixture.owner_message_id ?? 0) > 0 &&
    Number(feedFixture.foreign_message_id ?? 0) > 0 &&
    String(feedFixture.owner_secret ?? "") !== "";
  const passwordRecoveryFixture = smokeFixture?.password_recovery ?? {};
  const passwordRecoveryFixtureReady =
    typeof passwordRecoveryFixture.password === "string" &&
    typeof passwordRecoveryFixture.permanent?.name === "string" &&
    typeof passwordRecoveryFixture.permanent?.email === "string" &&
    typeof passwordRecoveryFixture.temporary?.name === "string" &&
    typeof passwordRecoveryFixture.temporary?.email === "string" &&
    typeof passwordRecoveryFixture.temporary?.temporary_email === "string";
  const fleetRestrictionsFixture = smokeFixture?.fleet_restrictions ?? {};
  const fleetRestrictionsReady = Boolean(
    typeof fleetRestrictionsFixture.attacker?.login === "string" &&
    typeof fleetRestrictionsFixture.weak_attacker?.login === "string" &&
    typeof fleetRestrictionsFixture.blocked_attacker?.login === "string" &&
    Number(fleetRestrictionsFixture.attacker?.home_planet_id ?? 0) > 0 &&
    Number(fleetRestrictionsFixture.weak_attacker?.home_planet_id ?? 0) > 0 &&
    Number(fleetRestrictionsFixture.blocked_attacker?.home_planet_id ?? 0) > 0 &&
    fleetRestrictionsFixture.noob?.coordinates &&
    fleetRestrictionsFixture.strong?.coordinates &&
    fleetRestrictionsFixture.vacation?.coordinates &&
    fleetRestrictionsFixture.operator?.coordinates &&
    fleetRestrictionsFixture.comparable?.coordinates
  );
  const legacyTransportReturnMission = 103;
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

  const unsafeDirectURLs = [
    "javascript:alert(1)",
    "data:text/html,<script>go-direct</script>",
    "file:///etc/passwd",
    "http://127.0.0.1:8888/game/index.php",
    "http://localhost:8888/game/index.php",
    "http://[::1]/game/index.php",
    "http://[::ffff:127.0.0.1]/game/index.php",
    "http://169.254.169.254/latest/meta-data/",
    "http://example.com@127.0.0.1/image.png"
  ];
  const unsafeRedirectResponses = [];
  for (const unsafeURL of unsafeDirectURLs) {
    unsafeRedirectResponses.push({
      unsafeURL,
      response: await request(`/game/redir.php?url=${encodeURIComponent(unsafeURL)}`)
    });
  }
  const unsafeImageURLs = [
    "file:///etc/passwd",
    "javascript:alert(1)",
    "http://127.0.0.1:8888/game/img/preload.gif",
    "http://[::1]/game/img/preload.gif",
    "http://[::ffff:127.0.0.1]/game/img/preload.gif",
    "http://169.254.169.254/latest/meta-data/iam/security-credentials.png",
    "http://example.com/image.svg"
  ];
  const unsafeImageResponses = [];
  for (const unsafeURL of unsafeImageURLs) {
    unsafeImageResponses.push({
      unsafeURL,
      response: await request(`/game/pic.php?url=${encodeURIComponent(unsafeURL)}`)
    });
  }
  const safeRedirect = await request(`/game/redir.php?url=${encodeURIComponent("https://example.com/ogame")}`);
  cases.push(finalize({
    case: "go_legacy_direct_entry_url_proxy_security",
    checks: [
      ...unsafeRedirectResponses.flatMap(({ unsafeURL, response }) => [
        check(response.status === 400, "legacy redir rejects unsafe direct URL with HTTP 400", { unsafeURL, status: response.status }),
        check(!hasHeader(response, "location"), "legacy redir does not issue a Location header for unsafe direct URL", { unsafeURL, location: response.headers.location }),
        check(!response.body.includes(unsafeURL), "legacy redir does not echo unsafe direct URL", { unsafeURL, body: response.body })
      ]),
      ...unsafeImageResponses.flatMap(({ unsafeURL, response }) => [
        check(response.status === 200, "legacy pic returns a legacy unavailable page for unsafe direct URL", { unsafeURL, status: response.status }),
        check(!String(response.headers["content-type"] ?? "").toLowerCase().startsWith("image/"), "legacy pic does not return image content for unsafe direct URL", { unsafeURL, contentType: response.headers["content-type"] }),
        check(response.body.includes("Графика недоступна"), "legacy pic renders the unavailable image text for unsafe direct URL", { unsafeURL, body: response.body })
      ]),
      check(safeRedirect.status === 200, "legacy redir accepts a safe public HTTP URL", { status: safeRedirect.status }),
      check(safeRedirect.body.includes("Page has moved") && safeRedirect.body.includes("https://example.com/ogame"), "legacy redir renders the meta refresh shell for safe URLs", { body: safeRedirect.body })
    ]
  }));

  const feedRSS = feedFixtureReady
    ? await request(`/game/feed/show.php?feedid=${encodeURIComponent(feedFixture.rss_feed_id)}`)
    : null;
  const feedAtom = feedFixtureReady
    ? await request(`/game/feed/show.php?feedid=${encodeURIComponent(feedFixture.atom_feed_id)}`)
    : null;
  const feedItem = feedFixtureReady
    ? await request(`/game/feed/viewitem.php?feedid=${encodeURIComponent(feedFixture.rss_feed_id)}&mid=${Number(feedFixture.owner_message_id)}&type=i`)
    : null;
  const feedForeignItem = feedFixtureReady
    ? await request(`/game/feed/viewitem.php?feedid=${encodeURIComponent(feedFixture.rss_feed_id)}&mid=${Number(feedFixture.foreign_message_id)}&type=i`)
    : null;
  const feedBadID = feedFixtureReady
    ? await request(`/game/feed/show.php?feedid=${encodeURIComponent(`${feedFixture.rss_feed_id}x<script>`)}`)
    : null;
  const feedBadMID = feedFixtureReady
    ? await request(`/game/feed/viewitem.php?feedid=${encodeURIComponent(feedFixture.rss_feed_id)}&mid=abc`)
    : null;
  const feedMissingID = await request("/game/feed/show.php");
  const unsafeFeedMarkup = /<script|<img|<\/textarea/i;
  cases.push(finalize({
    case: "go_legacy_feed_direct_entry_security",
    checks: [
      check(!smokeFixtureFile || feedFixtureReady, "go smoke fixture exposes feed tokens and message ids", { feedFixture }),
      check(!feedFixtureReady || feedRSS?.status === 200, "RSS feed returns HTTP 200", { status: feedRSS?.status }),
      check(!feedFixtureReady || hasHeader(feedRSS, "content-type", "xml"), "RSS feed returns XML content type", feedRSS?.headers ?? {}),
      check(!feedFixtureReady || feedRSS.body.includes("<rss version=\"2.0\">"), "RSS feed uses RSS envelope", { body: feedRSS?.body.slice(0, 120) }),
      check(!feedFixtureReady || feedRSS.body.includes(String(feedFixture.owner_secret)), "RSS feed includes owner message", { ownerSecret: feedFixture.owner_secret }),
      check(!feedFixtureReady || !feedRSS.body.includes(String(feedFixture.foreign_secret)), "RSS feed does not include foreign owner message"),
      check(!feedFixtureReady || !unsafeFeedMarkup.test(feedRSS.body), "RSS feed strips unsafe raw markup"),
      check(!feedFixtureReady || feedAtom?.status === 200, "Atom feed returns HTTP 200", { status: feedAtom?.status }),
      check(!feedFixtureReady || feedAtom.body.includes("<feed xmlns=\"http://www.w3.org/2005/Atom\">"), "Atom feed uses Atom envelope"),
      check(!feedFixtureReady || feedAtom.body.includes(String(feedFixture.atom_secret)), "Atom feed includes atom owner message", { atomSecret: feedFixture.atom_secret }),
      check(!feedFixtureReady || !unsafeFeedMarkup.test(feedAtom.body), "Atom feed strips unsafe raw markup"),
      check(!feedFixtureReady || feedItem?.status === 200, "feed item returns HTTP 200", { status: feedItem?.status }),
      check(!feedFixtureReady || feedItem.body.includes(String(feedFixture.owner_secret)), "feed item includes owner message"),
      check(!feedFixtureReady || !unsafeFeedMarkup.test(feedItem.body), "feed item strips unsafe raw markup"),
      check(!feedFixtureReady || feedForeignItem?.status === 200, "foreign feed item request returns HTTP 200", { status: feedForeignItem?.status }),
      check(!feedFixtureReady || !feedForeignItem.body.includes(String(feedFixture.foreign_secret)), "foreign feed item does not leak another user's message"),
      check(!feedFixtureReady || feedBadID?.body.includes("Error validating request parameters: feedid"), "invalid feedid returns legacy validation text", { body: feedBadID?.body }),
      check(!feedFixtureReady || !feedBadID.body.includes(String(feedFixture.owner_secret)), "invalid feedid does not leak owner feed"),
      check(!feedFixtureReady || feedBadMID?.body.includes("Error validating request parameters: mid"), "invalid message id returns legacy validation text", { body: feedBadMID?.body }),
      check(feedMissingID.body === "No feed specified", "missing feed id returns legacy text", { body: feedMissingID.body })
    ]
  }));

  const recoveryForm = await request("/game/reg/mail.php");
  const recoveryMissingMailClear = await clearMailhog();
  const recoveryMissing = await request("/game/reg/fa_pass.php", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: ""
  });
  const recoveryMissingMessages = await readMailhogMessages();
  const recoveryUnknownMailClear = await clearMailhog();
  const recoveryUnknown = await request("/api/public/password-recovery", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email: `missing-${runId}@example.local` })
  });
  const recoveryUnknownBody = parseJSON(recoveryUnknown);
  const recoveryUnknownMessages = await readMailhogMessages();
  const recoveryPermanentMailClear = await clearMailhog();
  const recoveryPermanent = passwordRecoveryFixtureReady
    ? await request("/game/reg/fa_pass.php", {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams({ email: passwordRecoveryFixture.permanent.email }).toString()
      })
    : null;
  const recoveryPermanentMail = passwordRecoveryFixtureReady
    ? await waitForMailhogMessage(passwordRecoveryFixture.permanent.email, "your password for")
    : { message: null };
  const recoveryPermanentMailBody = mailhogBody(recoveryPermanentMail.message);
  const recoveryPermanentPassword = extractRecoveryPassword(recoveryPermanentMailBody);
  const recoveryUniverse = universes[0]?.baseUrl ?? "http://localhost:8888";
  const recoveryPermanentOldLogin = passwordRecoveryFixtureReady
    ? await loginGameUser(passwordRecoveryFixture.permanent.name, passwordRecoveryFixture.password, recoveryUniverse)
    : null;
  const recoveryPermanentNewLogin = passwordRecoveryFixtureReady && recoveryPermanentPassword
    ? await loginGameUser(passwordRecoveryFixture.permanent.name, recoveryPermanentPassword, recoveryUniverse)
    : null;
  const recoveryTemporaryMailClear = await clearMailhog();
  const recoveryTemporary = passwordRecoveryFixtureReady
    ? await request("/game/reg/fa_pass.php", {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams({ email: passwordRecoveryFixture.temporary.temporary_email }).toString()
      })
    : null;
  const recoveryTemporaryMail = passwordRecoveryFixtureReady
    ? await waitForMailhogMessage(passwordRecoveryFixture.temporary.email, "your password for")
    : { message: null };
  const recoveryTemporaryMailBody = mailhogBody(recoveryTemporaryMail.message);
  const recoveryTemporaryPassword = extractRecoveryPassword(recoveryTemporaryMailBody);
  const recoveryTemporaryNewLogin = passwordRecoveryFixtureReady && recoveryTemporaryPassword
    ? await loginGameUser(passwordRecoveryFixture.temporary.name, recoveryTemporaryPassword, recoveryUniverse)
    : null;
  cases.push(finalize({
    case: "go_legacy_password_recovery_flow",
    checks: [
      check(!smokeFixtureFile || passwordRecoveryFixtureReady, "go smoke fixture exposes password recovery users", { passwordRecoveryFixture }),
      check(recoveryForm.status === 200, "legacy password recovery form returns HTTP 200", { status: recoveryForm.status }),
      check(recoveryForm.body.includes("Send Password") && recoveryForm.body.includes('name="email"') && recoveryForm.body.includes("fa_pass.php"), "legacy password recovery form keeps title, email field, and post target"),
      check(recoveryMissingMailClear.ok, "MailHog can be cleared before missing-email recovery", recoveryMissingMailClear),
      check(recoveryMissing.status === 200, "missing-email recovery POST returns HTTP 200", { status: recoveryMissing.status }),
      check(recoveryMissing.body.includes("doesn't exist"), "missing-email recovery renders legacy generic error"),
      check(recoveryMissingMessages.messages.length === 0, "missing-email recovery sends no mail", { count: recoveryMissingMessages.messages.length }),
      check(recoveryUnknownMailClear.ok, "MailHog can be cleared before unknown-email API recovery", recoveryUnknownMailClear),
      check(recoveryUnknown.status === 200, "unknown-email natural recovery API returns HTTP 200", { status: recoveryUnknown.status }),
      check(recoveryUnknownBody.submitted === true && recoveryUnknownBody.sent === false, "unknown-email natural recovery API is a silent no-op", recoveryUnknownBody),
      check(recoveryUnknownMessages.messages.length === 0, "unknown-email natural recovery API sends no mail", { count: recoveryUnknownMessages.messages.length }),
      check(!passwordRecoveryFixtureReady || recoveryPermanentMailClear.ok, "MailHog can be cleared before permanent-email recovery", recoveryPermanentMailClear),
      check(!passwordRecoveryFixtureReady || recoveryPermanent?.status === 200, "permanent-email legacy recovery returns HTTP 200", { status: recoveryPermanent?.status }),
      check(!passwordRecoveryFixtureReady || recoveryPermanent.body.includes(`Your password has been sent to ${passwordRecoveryFixture.permanent.name}`), "permanent-email recovery renders legacy success message", { body: recoveryPermanent?.body }),
      check(!passwordRecoveryFixtureReady || !recoveryPermanent.body.includes(recoveryPermanentPassword), "permanent-email recovery response does not expose the new password"),
      check(!passwordRecoveryFixtureReady || recoveryPermanentMail.message !== null, "permanent-email recovery sends mail to permanent address", {
        recipients: recoveryPermanentMail.message ? mailhogRecipients(recoveryPermanentMail.message) : []
      }),
      check(!passwordRecoveryFixtureReady || recoveryPermanentMailBody.includes(String(passwordRecoveryFixture.permanent.name)) && recoveryPermanentMailBody.includes("Universe 1"), "permanent recovery email includes player and universe", {
        body: recoveryPermanentMailBody.slice(0, 200)
      }),
      check(!passwordRecoveryFixtureReady || /^[a-z0-9]{8}$/.test(recoveryPermanentPassword), "permanent recovery email contains an 8-character generated password", { recoveryPermanentPassword }),
      check(!passwordRecoveryFixtureReady || recoveryPermanentOldLogin?.body.valid === false, "old password is rejected after permanent recovery", recoveryPermanentOldLogin?.body ?? {}),
      check(!passwordRecoveryFixtureReady || recoveryPermanentNewLogin?.response.status === 200, "new permanent recovery password login returns HTTP 200", { status: recoveryPermanentNewLogin?.response.status }),
      check(!passwordRecoveryFixtureReady || typeof recoveryPermanentNewLogin?.body.session?.redirectTo === "string" && recoveryPermanentNewLogin.body.session.redirectTo.includes("/game/overview"), "new permanent recovery password logs into overview", recoveryPermanentNewLogin?.body ?? {}),
      check(!passwordRecoveryFixtureReady || recoveryTemporaryMailClear.ok, "MailHog can be cleared before temporary-email recovery", recoveryTemporaryMailClear),
      check(!passwordRecoveryFixtureReady || recoveryTemporary?.status === 200, "temporary-email legacy recovery returns HTTP 200", { status: recoveryTemporary?.status }),
      check(!passwordRecoveryFixtureReady || recoveryTemporaryMail.message !== null, "temporary-email recovery sends mail to permanent address", {
        requested: passwordRecoveryFixture.temporary.temporary_email,
        recipients: recoveryTemporaryMail.message ? mailhogRecipients(recoveryTemporaryMail.message) : []
      }),
      check(!passwordRecoveryFixtureReady || mailhogRecipients(recoveryTemporaryMail.message).some((recipient) => recipient.includes(passwordRecoveryFixture.temporary.email)), "temporary-email recovery targets the permanent email only", {
        recipients: recoveryTemporaryMail.message ? mailhogRecipients(recoveryTemporaryMail.message) : []
      }),
      check(!passwordRecoveryFixtureReady || /^[a-z0-9]{8}$/.test(recoveryTemporaryPassword), "temporary recovery email contains an 8-character generated password", { recoveryTemporaryPassword }),
      check(!passwordRecoveryFixtureReady || recoveryTemporaryNewLogin?.response.status === 200, "temporary recovery password login returns HTTP 200", { status: recoveryTemporaryNewLogin?.response.status }),
      check(!passwordRecoveryFixtureReady || typeof recoveryTemporaryNewLogin?.body.session?.redirectTo === "string" && recoveryTemporaryNewLogin.body.session.redirectTo.includes("/game/overview"), "temporary recovery password logs into overview", recoveryTemporaryNewLogin?.body ?? {})
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
  const registrationCharacter = `NewPilot${runId}`;
  const registrationEmail = `new-pilot-${runId}@example.local`;
  const mailhogClear = await clearMailhog();
  const createdRegistration = await request("/api/public/registration", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: registrationCharacter,
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
  const duplicateNameValidation = await request("/api/public/registration/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: registrationCharacter,
      password: registrationPassword,
      email: `dup-name-${runId}@example.local`,
      universe: universes[0]?.baseUrl ?? "http://localhost:8888",
      agb: true
    })
  });
  const duplicateNameValidationBody = parseJSON(duplicateNameValidation);
  const duplicateNameIssues = Array.isArray(duplicateNameValidationBody.issues) ? duplicateNameValidationBody.issues : [];
  const duplicateEmailValidation = await request("/api/public/registration/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: `DupEmail${runId}`.slice(0, 20),
      password: registrationPassword,
      email: registrationEmail,
      universe: universes[0]?.baseUrl ?? "http://localhost:8888",
      agb: true
    })
  });
  const duplicateEmailValidationBody = parseJSON(duplicateEmailValidation);
  const duplicateEmailIssues = Array.isArray(duplicateEmailValidationBody.issues) ? duplicateEmailValidationBody.issues : [];
  const duplicateNameCreation = await request("/api/public/registration", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: registrationCharacter,
      password: registrationPassword,
      email: `dup-create-${runId}@example.local`,
      universe: universes[0]?.baseUrl ?? "http://localhost:8888",
      agb: true
    })
  });
  const duplicateNameCreationBody = parseJSON(duplicateNameCreation);
  const duplicateNameCreationIssues = Array.isArray(duplicateNameCreationBody.issues) ? duplicateNameCreationBody.issues : [];
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

  cases.push(finalize({
    case: "go_registration_duplicate_edges_api",
    checks: [
      check(duplicateNameValidation.status === 200, "duplicate-name registration validation returns HTTP 200", { status: duplicateNameValidation.status }),
      check(duplicateNameValidationBody.valid === false, "duplicate-name registration validation is rejected", duplicateNameValidationBody),
      check(duplicateNameIssues.some((issue) => issue.code === "character_exists" && issue.legacyErrorCode === 101), "duplicate username maps to legacy error 101", duplicateNameValidationBody),
      check(duplicateEmailValidation.status === 200, "duplicate-email registration validation returns HTTP 200", { status: duplicateEmailValidation.status }),
      check(duplicateEmailValidationBody.valid === false, "duplicate-email registration validation is rejected", duplicateEmailValidationBody),
      check(duplicateEmailIssues.some((issue) => issue.code === "email_exists" && issue.legacyErrorCode === 102), "duplicate email maps to legacy error 102", duplicateEmailValidationBody),
      check(duplicateNameCreation.status === 200, "duplicate-name registration creation returns HTTP 200", { status: duplicateNameCreation.status }),
      check(duplicateNameCreationBody.valid === false && duplicateNameCreationBody.created !== true, "duplicate-name creation does not create another account", duplicateNameCreationBody),
      check(duplicateNameCreationIssues.some((issue) => issue.code === "character_exists" && issue.legacyErrorCode === 101), "duplicate-name creation returns the legacy duplicate issue", duplicateNameCreationBody),
      check(!hasHeader(duplicateNameCreation, "set-cookie", "prsess_"), "duplicate-name creation does not set a private session cookie", duplicateNameCreation.headers)
    ]
  }));

  const rotationUniverse = universes[0]?.baseUrl ?? "http://localhost:8888";
  const rotationCharacter = `RotatePilot${runId}`;
  const rotationPassword = "E2E_http123";
  const rotationEmail = `rotate-pilot-${runId}@example.local`;
  const rotationMailClear = await clearMailhog();
  const rotationRegistration = await request("/api/public/registration", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: rotationCharacter,
      password: rotationPassword,
      email: rotationEmail,
      universe: rotationUniverse,
      agb: true
    })
  });
  const rotationRegistrationBody = parseJSON(rotationRegistration);
  const rotationWelcomeMail = await waitForMailhogMessage(rotationEmail, "activate your account");
  const rotationActivationLink = mailhogBody(rotationWelcomeMail.message).match(activationLinkPattern)?.[0] ?? "";
  const rotationActivationPath = pathFromURL(rotationActivationLink);
  const rotationActivation = rotationActivationPath
    ? await request(rotationActivationPath)
    : { status: 0, headers: {}, body: "" };
  const rotationFirstLogin = await loginGameUser(rotationCharacter, rotationPassword, rotationUniverse);
  const rotationFirstSession = await request(`/api/game/session${rotationFirstLogin.search}`, {
    headers: { Cookie: rotationFirstLogin.cookiePair }
  });
  const rotationSecondLogin = await loginGameUser(rotationCharacter, rotationPassword, rotationUniverse);
  const rotationOldPublicCurrentCookie = await request(`/api/game/session${rotationFirstLogin.search}`, {
    headers: { Cookie: rotationSecondLogin.cookiePair }
  });
  const rotationNewPublicOldCookie = await request(`/api/game/session${rotationSecondLogin.search}`, {
    headers: { Cookie: rotationFirstLogin.cookiePair }
  });
  const rotationSecondSession = await request(`/api/game/session${rotationSecondLogin.search}`, {
    headers: { Cookie: rotationSecondLogin.cookiePair }
  });
  const rotationFirstSessionBody = parseJSON(rotationFirstSession);
  const rotationOldPublicCurrentCookieBody = parseJSON(rotationOldPublicCurrentCookie);
  const rotationNewPublicOldCookieBody = parseJSON(rotationNewPublicOldCookie);
  const rotationSecondSessionBody = parseJSON(rotationSecondSession);
  cases.push(finalize({
    case: "go_session_rotation_security_api",
    checks: [
      check(rotationMailClear.ok, "MailHog can be cleared before session rotation registration", rotationMailClear),
      check(rotationRegistration.status === 200, "session rotation fixture registration returns HTTP 200", { status: rotationRegistration.status }),
      check(rotationRegistrationBody.valid === true && rotationRegistrationBody.created === true, "session rotation fixture account is created", rotationRegistrationBody),
      check(rotationWelcomeMail.message !== null, "session rotation fixture receives activation mail", {
        recipients: rotationWelcomeMail.message ? mailhogRecipients(rotationWelcomeMail.message) : []
      }),
      check(rotationActivation.status === 302, "session rotation fixture activation redirects after activation", {
        status: rotationActivation.status,
        location: rotationActivation.headers.location ?? ""
      }),
      check(rotationFirstLogin.response.status === 200, "first login for rotation fixture returns HTTP 200", { status: rotationFirstLogin.response.status }),
      check(rotationFirstLogin.body.valid === true, "first login for rotation fixture creates a session", rotationFirstLogin.body),
      check(rotationFirstSession.status === 200, "first session is valid before rotation", { status: rotationFirstSession.status }),
      check(rotationFirstSessionBody.authenticated === true, "first session authenticates before rotation", rotationFirstSessionBody),
      check(rotationSecondLogin.response.status === 200, "second login for rotation fixture returns HTTP 200", { status: rotationSecondLogin.response.status }),
      check(rotationSecondLogin.body.valid === true, "second login for rotation fixture creates a session", rotationSecondLogin.body),
      check(rotationFirstLogin.search !== rotationSecondLogin.search, "second login rotates the public session token", {
        first: rotationFirstLogin.search,
        second: rotationSecondLogin.search
      }),
      check(rotationFirstLogin.cookiePair !== rotationSecondLogin.cookiePair, "second login rotates the private session cookie", {
        firstCookie: rotationFirstLogin.cookiePair,
        secondCookie: rotationSecondLogin.cookiePair
      }),
      check(rotationOldPublicCurrentCookie.status === 401, "old public session is rejected with the current private cookie", {
        status: rotationOldPublicCurrentCookie.status,
        body: rotationOldPublicCurrentCookieBody
      }),
      check(rotationOldPublicCurrentCookieBody.authenticated === false, "old public/current private pairing is unauthenticated", rotationOldPublicCurrentCookieBody),
      check(rotationNewPublicOldCookie.status === 401, "new public session is rejected with the old private cookie", {
        status: rotationNewPublicOldCookie.status,
        body: rotationNewPublicOldCookieBody
      }),
      check(rotationNewPublicOldCookieBody.authenticated === false, "new public/old private pairing is unauthenticated", rotationNewPublicOldCookieBody),
      check(rotationSecondSession.status === 200, "new public and private session pair remains valid", { status: rotationSecondSession.status }),
      check(rotationSecondSessionBody.authenticated === true, "new public/private pair authenticates after rotation", rotationSecondSessionBody),
      check(!rotationSecondSession.body.includes(rotationSecondLogin.cookiePair), "rotated session lookup does not echo the private cookie")
    ]
  }));

  const accountSecurityUniverse = universes[0]?.baseUrl ?? "http://localhost:8888";
  const accountSecurityCharacter = `Sec${runId}`;
  const accountSecurityPassword = "E2E_http123";
  const accountSecurityNewPassword = "Changed_123";
  const accountSecurityEmail = `security-pilot-${runId}@example.local`;
  const accountSecurityNewEmail = `security-pilot-updated-${runId}@example.local`;
  const accountSecurityMailClear = await clearMailhog();
  const accountSecurityRegistration = await request("/api/public/registration", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      character: accountSecurityCharacter,
      password: accountSecurityPassword,
      email: accountSecurityEmail,
      universe: accountSecurityUniverse,
      agb: true
    })
  });
  const accountSecurityRegistrationBody = parseJSON(accountSecurityRegistration);
  const accountSecurityWelcomeMail = await waitForMailhogMessage(accountSecurityEmail, "activate your account");
  const accountSecurityActivationLink = mailhogBody(accountSecurityWelcomeMail.message).match(activationLinkPattern)?.[0] ?? "";
  const accountSecurityActivationPath = pathFromURL(accountSecurityActivationLink);
  const accountSecurityActivation = accountSecurityActivationPath
    ? await request(accountSecurityActivationPath)
    : { status: 0, headers: {}, body: "" };
  const accountSecurityLogin = await loginGameUser(accountSecurityCharacter, accountSecurityPassword, accountSecurityUniverse);
  const accountSecurityReady = accountSecurityLogin.body.valid === true && accountSecurityLogin.cookiePair !== "";
  const accountSecurityHeaders = {
    "Content-Type": "application/x-www-form-urlencoded",
    Cookie: accountSecurityLogin.cookiePair
  };
  const accountSecurityOptions = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        headers: { Cookie: accountSecurityLogin.cookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountSecurityOptionsBody = parseJSON(accountSecurityOptions);
  const accountDeletionQueued = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_deaktjava: "on"
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountDeletionQueuedBody = parseJSON(accountDeletionQueued);
  const accountDeletionCleared = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm()
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountDeletionClearedBody = parseJSON(accountDeletionCleared);
  const accountVacationEnabled = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          urlaubs_modus: "on"
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountVacationEnabledBody = parseJSON(accountVacationEnabled);
  const accountVacationLocked = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          urlaub_aus: "on"
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountVacationLockedBody = parseJSON(accountVacationLocked);
  const accountPasswordMismatch = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_password: accountSecurityPassword,
          newpass1: "Mismatch_123",
          newpass2: "Mismatch_124"
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountPasswordMismatchBody = parseJSON(accountPasswordMismatch);
  const accountPasswordSpecial = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_password: accountSecurityPassword,
          newpass1: "invalid!!",
          newpass2: "invalid!!"
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountPasswordSpecialBody = parseJSON(accountPasswordSpecial);
  const accountPasswordShort = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_password: accountSecurityPassword,
          newpass1: "short7",
          newpass2: "short7"
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountPasswordShortBody = parseJSON(accountPasswordShort);
  const accountPasswordWrongOld = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_password: "wrongpass",
          newpass1: accountSecurityNewPassword,
          newpass2: accountSecurityNewPassword
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountPasswordWrongOldBody = parseJSON(accountPasswordWrongOld);
  const accountEmailMissingPassword = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_email: accountSecurityNewEmail
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountEmailMissingPasswordBody = parseJSON(accountEmailMissingPassword);
  const accountEmailInvalid = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_password: accountSecurityPassword,
          db_email: "bad address"
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountEmailInvalidBody = parseJSON(accountEmailInvalid);
  const accountEmailUsed = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_password: accountSecurityPassword,
          db_email: registrationEmail
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountEmailUsedBody = parseJSON(accountEmailUsed);
  const accountEmailChanged = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_password: accountSecurityPassword,
          db_email: accountSecurityNewEmail
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountEmailChangedBody = parseJSON(accountEmailChanged);
  const accountPasswordChanged = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        method: "POST",
        headers: accountSecurityHeaders,
        body: legacyOptionsForm({
          db_password: accountSecurityPassword,
          newpass1: accountSecurityNewPassword,
          newpass2: accountSecurityNewPassword
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountPasswordChangedBody = parseJSON(accountPasswordChanged);
  const accountOldSessionAfterPasswordChange = accountSecurityReady
    ? await request(`/api/game/options${accountSecurityLogin.search}`, {
        headers: { Cookie: accountSecurityLogin.cookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const accountOldSessionAfterPasswordChangeBody = parseJSON(accountOldSessionAfterPasswordChange);
  const accountOldPasswordLogin = accountSecurityReady
    ? await loginGameUser(accountSecurityCharacter, accountSecurityPassword, accountSecurityUniverse)
    : { response: { status: 0 }, body: {}, cookiePair: "", search: "" };
  const accountNewPasswordLogin = accountSecurityReady
    ? await loginGameUser(accountSecurityCharacter, accountSecurityNewPassword, accountSecurityUniverse)
    : { response: { status: 0 }, body: {}, cookiePair: "", search: "" };
  cases.push(finalize({
    case: "go_account_security_options_legacy_form",
    checks: [
      check(accountSecurityMailClear.ok, "MailHog can be cleared before account-security registration", accountSecurityMailClear),
      check(accountSecurityRegistration.status === 200, "account-security fixture registration returns HTTP 200", { status: accountSecurityRegistration.status }),
      check(accountSecurityRegistrationBody.valid === true && accountSecurityRegistrationBody.created === true, "account-security fixture account is created", accountSecurityRegistrationBody),
      check(accountSecurityWelcomeMail.message !== null, "account-security fixture receives activation mail", {
        recipients: accountSecurityWelcomeMail.message ? mailhogRecipients(accountSecurityWelcomeMail.message) : []
      }),
      check(accountSecurityActivation.status === 302, "account-security fixture activation redirects after activation", {
        status: accountSecurityActivation.status,
        location: accountSecurityActivation.headers.location ?? ""
      }),
      check(accountSecurityLogin.response.status === 200, "account-security login returns HTTP 200", { status: accountSecurityLogin.response.status }),
      check(accountSecurityLogin.body.valid === true, "account-security login creates a session", accountSecurityLogin.body),
      check(accountSecurityOptions.status === 200, "account-security options page returns HTTP 200", { status: accountSecurityOptions.status }),
      check(accountSecurityOptionsBody.authenticated === true, "account-security options page authenticates", accountSecurityOptionsBody),
      check(accountDeletionQueuedBody.actionIssue?.code === "account_deletion_queued", "legacy options account deletion can be queued", accountDeletionQueuedBody.actionIssue ?? {}),
      check(accountDeletionQueuedBody.options?.account?.deletionQueued === true && Number(accountDeletionQueuedBody.options?.account?.deletionAt ?? 0) > 0, "legacy options account deletion stores a future deadline", accountDeletionQueuedBody.options?.account ?? {}),
      check(accountDeletionClearedBody.actionIssue?.code === "account_deletion_cleared", "legacy options account deletion can be cancelled", accountDeletionClearedBody.actionIssue ?? {}),
      check(accountDeletionClearedBody.options?.account?.deletionQueued === false, "legacy options account deletion cancel clears the flag", accountDeletionClearedBody.options?.account ?? {}),
      check(accountVacationEnabledBody.actionIssue?.code === "vacation_enabled", "legacy options vacation mode can be enabled", accountVacationEnabledBody.actionIssue ?? {}),
      check(accountVacationEnabledBody.options?.account?.vacation === true && Number(accountVacationEnabledBody.options?.account?.vacationUntil ?? 0) > 0, "legacy options vacation mode stores a minimum deadline", accountVacationEnabledBody.options?.account ?? {}),
      check(accountVacationLockedBody.actionIssue?.code === "vacation_locked", "legacy options vacation mode cannot be disabled before the minimum", accountVacationLockedBody.actionIssue ?? {}),
      check(accountVacationLockedBody.options?.account?.vacation === true, "legacy options locked vacation mode remains active", accountVacationLockedBody.options?.account ?? {}),
      check(accountPasswordMismatchBody.actionIssue?.code === "password_mismatch", "legacy options password mismatch is rejected", accountPasswordMismatchBody.actionIssue ?? {}),
      check(accountPasswordSpecialBody.actionIssue?.code === "password_special", "legacy options password special characters are rejected", accountPasswordSpecialBody.actionIssue ?? {}),
      check(accountPasswordShortBody.actionIssue?.code === "password_too_short", "legacy options short password is rejected", accountPasswordShortBody.actionIssue ?? {}),
      check(accountPasswordWrongOldBody.actionIssue?.code === "password_wrong_old", "legacy options wrong old password is rejected", accountPasswordWrongOldBody.actionIssue ?? {}),
      check(accountEmailMissingPasswordBody.actionIssue?.code === "email_need_password", "legacy options email change requires the current password", accountEmailMissingPasswordBody.actionIssue ?? {}),
      check(accountEmailInvalidBody.actionIssue?.code === "email_invalid", "legacy options invalid email is rejected", accountEmailInvalidBody.actionIssue ?? {}),
      check(accountEmailUsedBody.actionIssue?.code === "email_used", "legacy options duplicate email is rejected", accountEmailUsedBody.actionIssue ?? {}),
      check(accountEmailChangedBody.actionIssue?.code === "email_changed", "legacy options email change queues the email update", accountEmailChangedBody.actionIssue ?? {}),
      check(accountEmailChangedBody.options?.user?.email === accountSecurityNewEmail, "legacy options email change stores the pending email", accountEmailChangedBody.options?.user ?? {}),
      check(accountEmailChangedBody.options?.user?.validated === false, "legacy options email change marks the account unvalidated", accountEmailChangedBody.options?.user ?? {}),
      check(accountPasswordChangedBody.actionIssue?.code === "password_changed", "legacy options valid password change succeeds", accountPasswordChangedBody.actionIssue ?? {}),
      check(accountOldSessionAfterPasswordChange.status === 401, "legacy options password change invalidates the old session", {
        status: accountOldSessionAfterPasswordChange.status,
        body: accountOldSessionAfterPasswordChangeBody
      }),
      check(accountOldPasswordLogin.body.valid === false, "old account-security password cannot log in after change", accountOldPasswordLogin.body),
      check(accountNewPasswordLogin.response.status === 200, "new account-security password login returns HTTP 200", { status: accountNewPasswordLogin.response.status }),
      check(accountNewPasswordLogin.body.valid === true, "new account-security password can log in after change", accountNewPasswordLogin.body),
      check(!accountPasswordChanged.body.includes(accountSecurityNewPassword), "account-security options response does not echo the new password"),
      check(!accountNewPasswordLogin.body?.session?.redirectTo?.includes(accountNewPasswordLogin.cookiePair), "account-security login redirect does not echo the private cookie")
    ]
  }));

  const legacyRegistrationUniverse = universes[0]?.baseUrl ?? "http://localhost:8888";
  const legacyRegistrationGet = await request("/game/reg/newredirect.php");
  const legacyRegistrationMissingPassword = await request("/game/reg/newredirect.php", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      character: `LegacyBad${runId}`,
      email: `legacy-bad-${runId}@example.local`,
      universe: legacyRegistrationUniverse,
      agb: "on"
    }).toString()
  });
  const legacyRegistrationTermsOnly = await request("/game/reg/newredirect.php", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      character: `LegacyTerms${runId}`,
      password: "E2E_http123",
      email: `legacy-terms-${runId}@example.local`,
      universe: legacyRegistrationUniverse
    }).toString()
  });
  const legacyRegistrationPassword = "E2E_http123";
  const legacyRegistrationEmail = `legacy-form-${runId}@example.local`;
  const legacyRegistrationMailClear = await clearMailhog();
  const legacyRegistrationCreate = await request("/game/reg/newredirect.php", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      character: `LegacyForm${runId}`,
      password: legacyRegistrationPassword,
      email: legacyRegistrationEmail,
      universe: legacyRegistrationUniverse,
      agb: "on"
    }).toString()
  });
  const legacyRegistrationCookie = legacyRegistrationCreate.headers["set-cookie"] ?? "";
  const legacyRegistrationCookiePair = legacyRegistrationCookie.split(";")[0] ?? "";
  let legacyRegistrationSession = "";
  try {
    legacyRegistrationSession = new URL(legacyRegistrationCreate.headers.location ?? "", baseUrl).searchParams.get("session") ?? "";
  } catch {
    legacyRegistrationSession = "";
  }
  const legacyRegistrationOverview = legacyRegistrationSession
    ? await request(`/api/game/overview?session=${encodeURIComponent(legacyRegistrationSession)}`, {
        headers: { Cookie: legacyRegistrationCookiePair }
      })
    : { status: 0, headers: {}, body: "" };
  const legacyRegistrationOverviewBody = parseJSON(legacyRegistrationOverview);
  const legacyRegistrationWelcomeMail = await waitForMailhogMessage(legacyRegistrationEmail, "activate your account");
  const legacyRegistrationWelcomeBody = mailhogBody(legacyRegistrationWelcomeMail.message);
  cases.push(finalize({
    case: "go_legacy_registration_newredirect_route",
    checks: [
      check(legacyRegistrationGet.status === 200, "legacy newredirect GET returns HTTP 200", { status: legacyRegistrationGet.status }),
      check(legacyRegistrationGet.body.includes("url=new.php"), "legacy newredirect GET opens the legacy registration form"),
      check(legacyRegistrationMissingPassword.status === 200, "legacy newredirect missing password returns HTTP 200", { status: legacyRegistrationMissingPassword.status }),
      check(legacyRegistrationMissingPassword.body.includes("register.php?") && legacyRegistrationMissingPassword.body.includes("errorCode=107"), "legacy newredirect missing password maps to error 107", {
        body: legacyRegistrationMissingPassword.body
      }),
      check(legacyRegistrationMissingPassword.body.includes("agb=1"), "legacy newredirect preserves accepted terms on validation redirect", {
        body: legacyRegistrationMissingPassword.body
      }),
      check(legacyRegistrationTermsOnly.status === 200, "legacy newredirect missing terms returns HTTP 200", { status: legacyRegistrationTermsOnly.status }),
      check(legacyRegistrationTermsOnly.body.includes("errorCode=0") && legacyRegistrationTermsOnly.body.includes("agb=0"), "legacy newredirect preserves PHP terms-only redirect semantics", {
        body: legacyRegistrationTermsOnly.body
      }),
      check(legacyRegistrationMailClear.ok, "MailHog can be cleared before legacy newredirect registration", legacyRegistrationMailClear),
      check(legacyRegistrationCreate.status === 302, "legacy newredirect valid registration redirects after login", {
        status: legacyRegistrationCreate.status,
        location: legacyRegistrationCreate.headers.location ?? ""
      }),
      check(typeof legacyRegistrationCreate.headers.location === "string" && legacyRegistrationCreate.headers.location.includes("/game/overview?"), "legacy newredirect registration redirects to overview", {
        location: legacyRegistrationCreate.headers.location ?? ""
      }),
      check(/^prsess_\d+_1=/.test(legacyRegistrationCookiePair), "legacy newredirect registration sets a private session cookie", {
        cookie: legacyRegistrationCookiePair
      }),
      check(!legacyRegistrationCreate.body.includes(legacyRegistrationPassword), "legacy newredirect registration response does not echo password"),
      check(legacyRegistrationOverview.status === 200, "legacy newredirect registration session can read game overview", {
        status: legacyRegistrationOverview.status
      }),
      check(legacyRegistrationOverviewBody.authenticated === true, "legacy newredirect registration overview is authenticated", legacyRegistrationOverviewBody),
      check(legacyRegistrationWelcomeMail.message !== null, "legacy newredirect registration sends welcome mail", {
        recipients: legacyRegistrationWelcomeMail.message ? mailhogRecipients(legacyRegistrationWelcomeMail.message) : []
      }),
      check(legacyRegistrationWelcomeBody.includes(`Password: ${legacyRegistrationPassword}`), "legacy newredirect welcome mail keeps the legacy password line")
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
  const securityPublicHome = await request("/home.php");
  const securityHTTPSHome = await request("/home.php", {
    headers: { "X-Forwarded-Proto": "https" }
  });
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
  const sessionCookiePrivateValue = sessionCookiePair.includes("=")
    ? sessionCookiePair.slice(sessionCookiePair.indexOf("=") + 1)
    : "";
  const fakeUniverseCookiePair = loginPlayerId > 0
    ? `prsess_${loginPlayerId}_9901=${sessionCookiePrivateValue}`
    : sessionCookiePair;
  const gameSessionFakeUniverseCookie = await request(`/api/game/session${sessionSearch}`, {
    headers: { Cookie: fakeUniverseCookiePair }
  });
  const gameSessionFakeUniverseCookieBody = parseJSON(gameSessionFakeUniverseCookie);

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

  const fleetRestrictionSmallCargo = 202;
  const fleetRestrictionProbe = 210;
  const fleetRestrictionAttackerLogin = fleetRestrictionsReady
    ? await loginGameUser(fleetRestrictionsFixture.attacker.login, loginSmokePassword, universes[0]?.baseUrl ?? "http://localhost:8888")
    : null;
  const fleetRestrictionWeakLogin = fleetRestrictionsReady
    ? await loginGameUser(fleetRestrictionsFixture.weak_attacker.login, loginSmokePassword, universes[0]?.baseUrl ?? "http://localhost:8888")
    : null;
  const fleetRestrictionBlockedLogin = fleetRestrictionsReady
    ? await loginGameUser(fleetRestrictionsFixture.blocked_attacker.login, loginSmokePassword, universes[0]?.baseUrl ?? "http://localhost:8888")
    : null;
  const fleetRestrictionSearch = (login, actor) => withQueryParams(login?.search ?? "?session=", {
    cp: Number(actor?.home_planet_id ?? 0)
  });
  const fleetRestrictionAttackerSearch = fleetRestrictionsReady
    ? fleetRestrictionSearch(fleetRestrictionAttackerLogin, fleetRestrictionsFixture.attacker)
    : "?session=";
  const fleetRestrictionWeakSearch = fleetRestrictionsReady
    ? fleetRestrictionSearch(fleetRestrictionWeakLogin, fleetRestrictionsFixture.weak_attacker)
    : "?session=";
  const fleetRestrictionBlockedSearch = fleetRestrictionsReady
    ? fleetRestrictionSearch(fleetRestrictionBlockedLogin, fleetRestrictionsFixture.blocked_attacker)
    : "?session=";
  const fleetRestrictionAttackerBefore = fleetRestrictionsReady
    ? await request(`/api/game/fleet${fleetRestrictionAttackerSearch}`, {
        headers: { Cookie: fleetRestrictionAttackerLogin?.cookiePair ?? "" }
      })
    : null;
  const fleetRestrictionBlockedBefore = fleetRestrictionsReady
    ? await request(`/api/game/fleet${fleetRestrictionBlockedSearch}`, {
        headers: { Cookie: fleetRestrictionBlockedLogin?.cookiePair ?? "" }
      })
    : null;
  const fleetRestrictionWeakBefore = fleetRestrictionsReady
    ? await request(`/api/game/fleet${fleetRestrictionWeakSearch}`, {
        headers: { Cookie: fleetRestrictionWeakLogin?.cookiePair ?? "" }
      })
    : null;
  const fleetRestrictionAttackerBeforeBody = fleetRestrictionAttackerBefore ? parseJSON(fleetRestrictionAttackerBefore) : {};
  const fleetRestrictionBlockedBeforeBody = fleetRestrictionBlockedBefore ? parseJSON(fleetRestrictionBlockedBefore) : {};
  const fleetRestrictionWeakBeforeBody = fleetRestrictionWeakBefore ? parseJSON(fleetRestrictionWeakBefore) : {};
  async function launchFleetRestriction(login, search, target, mission, ships) {
    if (!fleetRestrictionsReady) {
      return { status: 0, body: "", headers: {} };
    }
    return request(`/api/game/fleet${search}`, {
      method: "POST",
      headers: { Cookie: login?.cookiePair ?? "", "Content-Type": "application/json" },
      body: JSON.stringify({
        action: "launch-dispatch",
        ships,
        resources: {},
        target: target.coordinates,
        targetType: 1,
        mission,
        speed: 10
      })
    });
  }
  const fleetRestrictionNoobAttack = await launchFleetRestriction(
    fleetRestrictionAttackerLogin,
    fleetRestrictionAttackerSearch,
    fleetRestrictionsFixture.noob,
    1,
    { [String(fleetRestrictionSmallCargo)]: 1 }
  );
  const fleetRestrictionStrongAttack = await launchFleetRestriction(
    fleetRestrictionWeakLogin,
    fleetRestrictionWeakSearch,
    fleetRestrictionsFixture.strong,
    1,
    { [String(fleetRestrictionSmallCargo)]: 1 }
  );
  const fleetRestrictionVacationAttack = await launchFleetRestriction(
    fleetRestrictionAttackerLogin,
    fleetRestrictionAttackerSearch,
    fleetRestrictionsFixture.vacation,
    1,
    { [String(fleetRestrictionSmallCargo)]: 1 }
  );
  const fleetRestrictionOperatorSpy = await launchFleetRestriction(
    fleetRestrictionAttackerLogin,
    fleetRestrictionAttackerSearch,
    fleetRestrictionsFixture.operator,
    6,
    { [String(fleetRestrictionProbe)]: 1 }
  );
  const fleetRestrictionAttackBan = await launchFleetRestriction(
    fleetRestrictionBlockedLogin,
    fleetRestrictionBlockedSearch,
    fleetRestrictionsFixture.comparable,
    1,
    { [String(fleetRestrictionSmallCargo)]: 1 }
  );
  const fleetRestrictionNoobAttackBody = parseJSON(fleetRestrictionNoobAttack);
  const fleetRestrictionStrongAttackBody = parseJSON(fleetRestrictionStrongAttack);
  const fleetRestrictionVacationAttackBody = parseJSON(fleetRestrictionVacationAttack);
  const fleetRestrictionOperatorSpyBody = parseJSON(fleetRestrictionOperatorSpy);
  const fleetRestrictionAttackBanBody = parseJSON(fleetRestrictionAttackBan);
  const fleetRestrictionAttackerAfter = fleetRestrictionsReady
    ? await request(`/api/game/fleet${fleetRestrictionAttackerSearch}`, {
        headers: { Cookie: fleetRestrictionAttackerLogin?.cookiePair ?? "" }
      })
    : null;
  const fleetRestrictionBlockedAfter = fleetRestrictionsReady
    ? await request(`/api/game/fleet${fleetRestrictionBlockedSearch}`, {
        headers: { Cookie: fleetRestrictionBlockedLogin?.cookiePair ?? "" }
      })
    : null;
  const fleetRestrictionWeakAfter = fleetRestrictionsReady
    ? await request(`/api/game/fleet${fleetRestrictionWeakSearch}`, {
        headers: { Cookie: fleetRestrictionWeakLogin?.cookiePair ?? "" }
      })
    : null;
  const fleetRestrictionAttackerAfterBody = fleetRestrictionAttackerAfter ? parseJSON(fleetRestrictionAttackerAfter) : {};
  const fleetRestrictionBlockedAfterBody = fleetRestrictionBlockedAfter ? parseJSON(fleetRestrictionBlockedAfter) : {};
  const fleetRestrictionWeakAfterBody = fleetRestrictionWeakAfter ? parseJSON(fleetRestrictionWeakAfter) : {};

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

  const targetLogin = await loginGameUser("gophalaxtarget", loginSmokePassword, universes[0]?.baseUrl ?? "http://localhost:8888");

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
  const gameReportForeignAccess = sentReportID > 0 && targetLogin.cookiePair
    ? await request(`/api/game/report${targetLogin.search}&bericht=${sentReportID}`, {
        headers: { Cookie: targetLogin.cookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const gameReportForeignAccessBody = parseJSON(gameReportForeignAccess);
  const gameMessagesForeignDelete = sentReportID > 0 && targetLogin.cookiePair
    ? await request(`/api/game/messages${targetLogin.search}`, {
        method: "POST",
        headers: { Cookie: targetLogin.cookiePair, "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "delete",
          deleteMode: "deletemarked",
          messageIds: [sentReportID]
        })
      })
    : { status: 0, headers: {}, body: "{}" };
  const gameMessagesForeignDeleteBody = parseJSON(gameMessagesForeignDelete);
  const gameReportAfterForeignDelete = sentReportID > 0
    ? await request(`/api/game/report${sessionSearch}&bericht=${sentReportID}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const gameReportAfterForeignDeleteBody = parseJSON(gameReportAfterForeignDelete);
  const legacyGetMessageDelete = sentReportID > 0
    ? await request(`/game/index.php?page=messages${sessionSearch.replace("?", "&")}&messages=1&deletemessages=deleteall&delmes${sentReportID}=on`, {
        headers: { Cookie: sessionCookiePair }
      })
    : { status: 0, headers: {}, body: "" };
  const gameReportAfterLegacyGetDelete = sentReportID > 0
    ? await request(`/api/game/report${sessionSearch}&bericht=${sentReportID}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const gameReportAfterLegacyGetDeleteBody = parseJSON(gameReportAfterLegacyGetDelete);

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
  const regularAdminOperationModes = ["Broadcast", "Reports", "BattleSim", "RakSim", "Expedition"];
  const regularAdminOperationDenials = await Promise.all(regularAdminOperationModes.map(async (mode) => {
    const response = await request(`/api/game/admin${withQueryParam(targetLogin.search, "mode", mode)}`, {
      headers: { Cookie: targetLogin.cookiePair }
    });
    return { mode, response, body: parseJSON(response) };
  }));
  const operatorLogin = adminQueueFixtureReady || adminFleetlogsFixtureReady || adminOperationsReady
    ? await loginGameUser("gooperator", loginSmokePassword, universes[0]?.baseUrl ?? "http://localhost:8888")
    : null;
  const adminExpeditionBeforeSettings = operatorLogin
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Expedition")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminExpeditionBeforeSettingsBody = adminExpeditionBeforeSettings ? parseJSON(adminExpeditionBeforeSettings) : {};
  const originalExpeditionChance = Number(adminExpeditionBeforeSettingsBody.admin?.expedition?.chance_success ?? Number.NaN);
  const adminExpeditionSettingsReady = Number.isFinite(originalExpeditionChance);
  const operatorExpeditionChance = originalExpeditionChance === 99 ? 98 : originalExpeditionChance + 1;
  const adminExpeditionChance = operatorExpeditionChance === 99 ? 97 : operatorExpeditionChance + 1;
  const operatorExpeditionSettings = adminExpeditionSettingsReady && operatorLogin
    ? await request(`/api/game/admin${withQueryParam(operatorLogin.search, "mode", "Expedition")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: operatorLogin.cookiePair },
        body: JSON.stringify({
          action: "settings",
          values: { chance_success: operatorExpeditionChance }
        })
      })
    : null;
  const operatorExpeditionSettingsBody = operatorExpeditionSettings ? parseJSON(operatorExpeditionSettings) : {};
  const adminExpeditionAfterOperatorSettings = adminExpeditionSettingsReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Expedition")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminExpeditionAfterOperatorSettingsBody = adminExpeditionAfterOperatorSettings ? parseJSON(adminExpeditionAfterOperatorSettings) : {};
  const adminExpeditionSettings = adminExpeditionSettingsReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Expedition")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
        body: JSON.stringify({
          action: "settings",
          values: { chance_success: adminExpeditionChance }
        })
      })
    : null;
  const adminExpeditionSettingsBody = adminExpeditionSettings ? parseJSON(adminExpeditionSettings) : {};
  const adminExpeditionAfterAdminSettings = adminExpeditionSettingsReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Expedition")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminExpeditionAfterAdminSettingsBody = adminExpeditionAfterAdminSettings ? parseJSON(adminExpeditionAfterAdminSettings) : {};
  const adminExpeditionRestore = adminExpeditionSettingsReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Expedition")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
        body: JSON.stringify({
          action: "settings",
          values: { chance_success: originalExpeditionChance }
        })
      })
    : null;
  const adminExpeditionRestoreBody = adminExpeditionRestore ? parseJSON(adminExpeditionRestore) : {};
  const adminExpeditionAfterRestore = adminExpeditionSettingsReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Expedition")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminExpeditionAfterRestoreBody = adminExpeditionAfterRestore ? parseJSON(adminExpeditionAfterRestore) : {};
  const adminReportsBeforeDelete = adminOperationsReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Reports")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminReportsBeforeDeleteBody = adminReportsBeforeDelete ? parseJSON(adminReportsBeforeDelete) : {};
  const adminReportSeedRow = Array.isArray(adminReportsBeforeDeleteBody.admin?.reportRows)
    ? adminReportsBeforeDeleteBody.admin.reportRows.find((row) => Number(row.id) === Number(adminOperationsFixture.report_id))
    : undefined;
  const operatorReportsDelete = adminOperationsReady && operatorLogin
    ? await request(`/api/game/admin${withQueryParam(operatorLogin.search, "mode", "Reports")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: operatorLogin.cookiePair },
        body: JSON.stringify({
          action: "reports_delete",
          reportIds: [Number(adminOperationsFixture.report_id)],
          deleteMode: "deletemarked"
        })
      })
    : null;
  const operatorReportsDeleteBody = operatorReportsDelete ? parseJSON(operatorReportsDelete) : {};
  const adminReportsAfterDelete = adminOperationsReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Reports")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminReportsAfterDeleteBody = adminReportsAfterDelete ? parseJSON(adminReportsAfterDelete) : {};
  const adminReportDeletedRow = Array.isArray(adminReportsAfterDeleteBody.admin?.reportRows)
    ? adminReportsAfterDeleteBody.admin.reportRows.find((row) => Number(row.id) === Number(adminOperationsFixture.report_id))
    : undefined;
  const adminBroadcast = adminOperationsReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Broadcast")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
        body: JSON.stringify({
          action: "broadcast_send",
          category: 3,
          subject: `${adminOperationsFixture.token} broadcast subject`,
          text: `${adminOperationsFixture.token} broadcast body`
        })
      })
    : null;
  const adminBroadcastBody = adminBroadcast ? parseJSON(adminBroadcast) : {};
  const operatorMessagesAfterBroadcast = adminOperationsReady && operatorLogin
    ? await request(`/api/game/messages${operatorLogin.search}`, {
        headers: { Cookie: operatorLogin.cookiePair }
      })
    : null;
  const operatorMessagesAfterBroadcastBody = operatorMessagesAfterBroadcast ? parseJSON(operatorMessagesAfterBroadcast) : {};
  const operatorQueueFreeze = adminQueueFixtureReady && operatorLogin
    ? await request(`/api/game/admin${withQueryParam(operatorLogin.search, "mode", "Queue")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: operatorLogin.cookiePair },
        body: JSON.stringify({ action: "queue_freeze", taskId: adminQueueTaskId })
      })
    : null;
  const operatorQueueFreezeBody = operatorQueueFreeze ? parseJSON(operatorQueueFreeze) : {};
  const adminQueueFreeze = adminQueueFixtureReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Queue")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
        body: JSON.stringify({ action: "queue_freeze", taskId: adminQueueTaskId })
      })
    : null;
  const adminQueueFreezeBody = adminQueueFreeze ? parseJSON(adminQueueFreeze) : {};
  const adminQueueAfterFreeze = adminQueueFixtureReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Queue")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminQueueAfterFreezeBody = adminQueueAfterFreeze ? parseJSON(adminQueueAfterFreeze) : {};
  const operatorFleetlogsTwoMinute = adminFleetlogsFixtureReady && operatorLogin
    ? await request(`/api/game/admin${withQueryParam(operatorLogin.search, "mode", "Fleetlogs")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: operatorLogin.cookiePair },
        body: JSON.stringify({ action: "fleetlogs_2min", taskId: adminFleetlogsTaskId })
      })
    : null;
  const operatorFleetlogsTwoMinuteBody = operatorFleetlogsTwoMinute ? parseJSON(operatorFleetlogsTwoMinute) : {};
  const adminFleetlogsTwoMinuteStartedAt = Math.floor(Date.now() / 1000);
  const adminFleetlogsTwoMinute = adminFleetlogsFixtureReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Fleetlogs")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
        body: JSON.stringify({ action: "fleetlogs_2min", taskId: adminFleetlogsTaskId })
      })
    : null;
  const adminFleetlogsTwoMinuteBody = adminFleetlogsTwoMinute ? parseJSON(adminFleetlogsTwoMinute) : {};
  const adminFleetlogsAfterTwoMinute = adminFleetlogsFixtureReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Fleetlogs")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminFleetlogsAfterTwoMinuteBody = adminFleetlogsAfterTwoMinute ? parseJSON(adminFleetlogsAfterTwoMinute) : {};
  const adminFleetlogsReturn = adminFleetlogsRecallFixtureReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Fleetlogs")}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
        body: JSON.stringify({ action: "fleetlogs_return", taskId: adminFleetlogsRecallTaskId })
      })
    : null;
  const adminFleetlogsReturnBody = adminFleetlogsReturn ? parseJSON(adminFleetlogsReturn) : {};
  const adminFleetlogsAfterReturn = adminFleetlogsRecallFixtureReady
    ? await request(`/api/game/admin${withQueryParam(sessionSearch, "mode", "Fleetlogs")}`, {
        headers: { Cookie: sessionCookiePair }
      })
    : null;
  const adminFleetlogsAfterReturnBody = adminFleetlogsAfterReturn ? parseJSON(adminFleetlogsAfterReturn) : {};
  const adminSubmodeSpecs = [
    { name: "Users", mode: "Users", arrayKey: "userRows" },
    { name: "Planets", mode: "Planets", arrayKey: "planetRows" },
    { name: "Reports", mode: "Reports", arrayKey: "reportRows" },
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
  const allianceFounderLogin = operatorLogin ?? {
    response: sessionLogin,
    search: sessionSearch,
    cookiePair: sessionCookiePair,
    playerId: loginPlayerId
  };
  const allianceTag = `GOSM${runId}`.replace(/[^A-Za-z0-9]/g, "").slice(0, 8);
  const allianceName = `Go smoke alliance ${runId}`.slice(0, 30);
  const allianceCreate = await request(`/api/game/alliance${withQueryParams(allianceFounderLogin.search, { a: "1", weiter: "1" })}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: allianceFounderLogin.cookiePair },
    body: JSON.stringify({ action: "create", tag: allianceTag, name: allianceName })
  });
  const allianceCreateBody = parseJSON(allianceCreate);
  const createdAllianceId = Number(allianceCreateBody.alliance?.own?.id ?? 0);
  const allianceApply = createdAllianceId > 0
    ? await request(`/api/game/alliance${withQueryParams(targetLogin.search, { page: "bewerben", allyid: createdAllianceId })}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: targetLogin.cookiePair },
        body: JSON.stringify({ action: "apply", allianceId: createdAllianceId, text: `Go smoke application ${runId}` })
      })
    : { status: 0, body: "", headers: {} };
  const allianceApplyBody = parseJSON(allianceApply);
  const applicationId = Number(allianceApplyBody.alliance?.pending?.id ?? 0);
  const allianceAccept = applicationId > 0
    ? await request(`/api/game/alliance${withQueryParams(allianceFounderLogin.search, { page: "bewerbungen", show: applicationId, sort: "1" })}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: allianceFounderLogin.cookiePair },
        body: JSON.stringify({ action: "accept", applicationId })
      })
    : { status: 0, body: "", headers: {} };
  const allianceAcceptBody = parseJSON(allianceAccept);
  const rankName = `GoSmoke${runId}`.replace(/[^A-Za-z0-9._ -]/g, "").slice(0, 30);
  const allianceRankCreate = createdAllianceId > 0
    ? await request(`/api/game/alliance${withQueryParams(allianceFounderLogin.search, { a: "15" })}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: allianceFounderLogin.cookiePair },
        body: JSON.stringify({ action: "add_rank", rankName })
      })
    : { status: 0, body: "", headers: {} };
  const allianceRankCreateBody = parseJSON(allianceRankCreate);
  const createdRankId = Number((allianceRankCreateBody.alliance?.ranks ?? []).find((rank) => rank.name === rankName)?.id ?? 0);
  const rankRights = 0x008 | 0x020 | 0x080;
  const allianceRankRights = createdRankId > 0
    ? await request(`/api/game/alliance${withQueryParams(allianceFounderLogin.search, { a: "15" })}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: allianceFounderLogin.cookiePair },
        body: JSON.stringify({ action: "save_ranks", rankRights: [{ id: createdRankId, rights: rankRights }] })
      })
    : { status: 0, body: "", headers: {} };
  const allianceRankRightsBody = parseJSON(allianceRankRights);
  const rankAfterRights = (allianceRankRightsBody.alliance?.ranks ?? []).find((rank) => rank.id === createdRankId);
  const allianceAssignRank = createdRankId > 0 && targetLogin.playerId > 0
    ? await request(`/api/game/alliance${withQueryParams(allianceFounderLogin.search, { a: "16", u: targetLogin.playerId })}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: allianceFounderLogin.cookiePair },
        body: JSON.stringify({ action: "assign_rank", targetPlayerId: targetLogin.playerId, targetRankId: createdRankId })
      })
    : { status: 0, body: "", headers: {} };
  const allianceAssignRankBody = parseJSON(allianceAssignRank);
  const assignedMember = (allianceAssignRankBody.alliance?.members ?? []).find((member) => member.playerId === targetLogin.playerId);
  const circularText = `Go smoke circular ${runId}`;
  const allianceCircular = createdRankId > 0
    ? await request(`/api/game/alliance${withQueryParams(targetLogin.search, { a: "17", sendmail: "1" })}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: targetLogin.cookiePair },
        body: JSON.stringify({ action: "send_circular", circularRankId: createdRankId, text: circularText })
      })
    : { status: 0, body: "", headers: {} };
  const allianceCircularBody = parseJSON(allianceCircular);
  const targetMessagesAfterCircular = await request(`/api/game/messages${targetLogin.search}`, {
    headers: { Cookie: targetLogin.cookiePair }
  });
  const targetMessagesAfterCircularBody = parseJSON(targetMessagesAfterCircular);
  const allianceSettingsBeforeLegacyGet = createdAllianceId > 0
    ? await request(`/api/game/alliance${withQueryParams(allianceFounderLogin.search, { a: "11", d: "2" })}`, {
        headers: { Cookie: allianceFounderLogin.cookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const allianceSettingsBeforeLegacyGetBody = parseJSON(allianceSettingsBeforeLegacyGet);
  const legacyGetAllianceSettings = createdAllianceId > 0
    ? await request(`/game/index.php?page=allianzen${allianceFounderLogin.search.replace("?", "&")}&a=11&d=2&hp=${encodeURIComponent(`https://example.com/${runId}`)}&logo=${encodeURIComponent(`https://example.com/${runId}.png`)}&bew=0&fname=${encodeURIComponent(`Founder ${runId}`)}`, {
        headers: { Cookie: allianceFounderLogin.cookiePair }
      })
    : { status: 0, headers: {}, body: "" };
  const allianceSettingsAfterLegacyGet = createdAllianceId > 0
    ? await request(`/api/game/alliance${withQueryParams(allianceFounderLogin.search, { a: "11", d: "2" })}`, {
        headers: { Cookie: allianceFounderLogin.cookiePair }
      })
    : { status: 0, headers: {}, body: "{}" };
  const allianceSettingsAfterLegacyGetBody = parseJSON(allianceSettingsAfterLegacyGet);

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
  const legacyGetOptionsDeletion = await request(`/game/index.php?page=options${sessionSearch.replace("?", "&")}&mode=change&db_deaktjava=on`, {
    headers: { Cookie: sessionCookiePair }
  });
  const gameOptionsAfterLegacyGet = await request(`/api/game/options${sessionSearch}`, {
    headers: { Cookie: sessionCookiePair }
  });
  const gameOptionsAfterLegacyGetBody = parseJSON(gameOptionsAfterLegacyGet);

  const hardeningInvalidOverviewCP = await request(`/api/game/overview${withQueryParam(sessionSearch, "cp", "abc")}`, {
    headers: { Cookie: sessionCookiePair }
  });
  const hardeningInvalidOptionsCP = await request(`/api/game/options${withQueryParam(sessionSearch, "cp", "abc")}`, {
    headers: { Cookie: sessionCookiePair }
  });
  const hardeningInvalidReportID = await request(`/api/game/report${sessionSearch}&bericht=abc`, {
    headers: { Cookie: sessionCookiePair }
  });
  const hardeningInvalidMessageTarget = await request(`/api/game/messages${sessionSearch}&messageziel=abc`, {
    headers: { Cookie: sessionCookiePair }
  });
  const hardeningMalformedResources = await request(`/api/game/resources${sessionSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
    body: "{"
  });
  const hardeningMalformedOptions = await request(`/api/game/options${sessionSearch}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: sessionCookiePair },
    body: "{"
  });
  const hardeningUnknownAPI = await request("/api/does-not-exist");

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
      check(gameSessionFakeUniverseCookie.status === 401, "game session lookup rejects a private cookie from another universe suffix", {
        status: gameSessionFakeUniverseCookie.status,
        cookie: fakeUniverseCookiePair,
        body: gameSessionFakeUniverseCookieBody
      }),
      check(gameSessionFakeUniverseCookieBody.authenticated === false, "fake-universe private cookie is unauthenticated", gameSessionFakeUniverseCookieBody),
      check(gameSessionFakeUniverseCookieBody.issues?.some((issue) => issue.code === "private_session_invalid"), "fake-universe private cookie reports a private session issue", gameSessionFakeUniverseCookieBody),
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
      check(gameReportForeignAccess.status === 200, "foreign user report lookup returns HTTP 200 without leaking content", {
        status: gameReportForeignAccess.status
      }),
      check(gameReportForeignAccessBody.authenticated === true, "foreign report lookup still authenticates the requester", gameReportForeignAccessBody),
      check(gameReportForeignAccessBody.report?.id === sentReportID, "foreign report lookup maps the requested bericht id", gameReportForeignAccessBody),
      check(gameReportForeignAccessBody.report?.allowed === false, "foreign user cannot access another player's report body", gameReportForeignAccessBody.report ?? {}),
      check(String(gameReportForeignAccessBody.report?.text ?? "") === "", "foreign report lookup strips protected text", gameReportForeignAccessBody.report ?? {}),
      check(gameMessagesForeignDelete.status === 200, "foreign message delete attempt returns HTTP 200 as a scoped no-op", {
        status: gameMessagesForeignDelete.status
      }),
      check(gameMessagesForeignDeleteBody.authenticated === true, "foreign message delete attempt authenticates only the requester", gameMessagesForeignDeleteBody),
      check(gameReportAfterForeignDelete.status === 200, "owner can reload report after foreign delete attempt", {
        status: gameReportAfterForeignDelete.status
      }),
      check(gameReportAfterForeignDeleteBody.report?.allowed === true, "foreign delete attempt does not remove owner report access", gameReportAfterForeignDeleteBody.report ?? {}),
      check(String(gameReportAfterForeignDeleteBody.report?.text ?? "").includes("Go migration message smoke"), "foreign delete attempt does not delete owner message text", gameReportAfterForeignDeleteBody.report ?? {}),
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

  cases.push(finalize({
    case: "go_fleet_target_restrictions_api",
    checks: [
      check(!smokeFixtureFile || fleetRestrictionsReady, "go smoke fixture exposes fleet target restriction users", { fleetRestrictionsFixture }),
      check(!fleetRestrictionsReady || fleetRestrictionAttackerLogin?.response.status === 200, "fleet restriction attacker can log in", {
        status: fleetRestrictionAttackerLogin?.response.status
      }),
      check(!fleetRestrictionsReady || fleetRestrictionWeakLogin?.response.status === 200, "fleet restriction weak attacker can log in", {
        status: fleetRestrictionWeakLogin?.response.status
      }),
      check(!fleetRestrictionsReady || fleetRestrictionBlockedLogin?.response.status === 200, "fleet restriction noattack attacker can log in", {
        status: fleetRestrictionBlockedLogin?.response.status
      }),
      check(!fleetRestrictionsReady || fleetRestrictionAttackerBefore?.status === 200, "fleet restriction attacker fleet screen loads before blocked launches", {
        status: fleetRestrictionAttackerBefore?.status
      }),
      check(!fleetRestrictionsReady || fleetRestrictionBlockedBefore?.status === 200, "fleet restriction blocked attacker fleet screen loads before blocked launches", {
        status: fleetRestrictionBlockedBefore?.status
      }),
      check(!fleetRestrictionsReady || fleetRestrictionWeakBefore?.status === 200, "fleet restriction weak attacker fleet screen loads before blocked launches", {
        status: fleetRestrictionWeakBefore?.status
      }),
      check(!fleetRestrictionsReady || fleetRestrictionNoobAttack.status === 200, "newbie-protected target launch returns HTTP 200", { status: fleetRestrictionNoobAttack.status }),
      check(!fleetRestrictionsReady || fleetRestrictionNoobAttackBody.actionIssue?.code === "target_noob", "newbie-protected target returns legacy noob issue", fleetRestrictionNoobAttackBody.actionIssue ?? {}),
      check(!fleetRestrictionsReady || fleetRestrictionStrongAttack.status === 200, "strong-protected target launch returns HTTP 200", { status: fleetRestrictionStrongAttack.status }),
      check(!fleetRestrictionsReady || fleetRestrictionStrongAttackBody.actionIssue?.code === "target_noob", "strong-protected target shares the legacy noob issue", fleetRestrictionStrongAttackBody.actionIssue ?? {}),
      check(!fleetRestrictionsReady || fleetRestrictionVacationAttack.status === 200, "vacation target launch returns HTTP 200", { status: fleetRestrictionVacationAttack.status }),
      check(!fleetRestrictionsReady || fleetRestrictionVacationAttackBody.actionIssue?.code === "vacation_other", "vacation target returns legacy vacation issue", fleetRestrictionVacationAttackBody.actionIssue ?? {}),
      check(!fleetRestrictionsReady || fleetRestrictionOperatorSpy.status === 200, "operator target spy launch returns HTTP 200", { status: fleetRestrictionOperatorSpy.status }),
      check(!fleetRestrictionsReady || fleetRestrictionOperatorSpyBody.actionIssue?.code === "target_admin", "operator target returns legacy admin issue", fleetRestrictionOperatorSpyBody.actionIssue ?? {}),
      check(!fleetRestrictionsReady || fleetRestrictionAttackBan.status === 200, "noattack player launch returns HTTP 200", { status: fleetRestrictionAttackBan.status }),
      check(!fleetRestrictionsReady || fleetRestrictionAttackBanBody.actionIssue?.code === "attack_ban", "noattack player returns legacy attack-ban issue", fleetRestrictionAttackBanBody.actionIssue ?? {}),
      check(
        !fleetRestrictionsReady ||
          (fleetRestrictionAttackerAfterBody.fleet?.missions?.length ?? -1) === (fleetRestrictionAttackerBeforeBody.fleet?.missions?.length ?? -2),
        "blocked target restriction launches do not create attacker fleet rows",
        {
          before: fleetRestrictionAttackerBeforeBody.fleet?.missions?.length,
          after: fleetRestrictionAttackerAfterBody.fleet?.missions?.length
        }
      ),
      check(
        !fleetRestrictionsReady ||
          (fleetRestrictionBlockedAfterBody.fleet?.missions?.length ?? -1) === (fleetRestrictionBlockedBeforeBody.fleet?.missions?.length ?? -2),
        "blocked attack-ban launch does not create noattack player fleet rows",
        {
          before: fleetRestrictionBlockedBeforeBody.fleet?.missions?.length,
          after: fleetRestrictionBlockedAfterBody.fleet?.missions?.length
        }
      ),
      check(
        !fleetRestrictionsReady ||
          (fleetRestrictionWeakAfterBody.fleet?.missions?.length ?? -1) === (fleetRestrictionWeakBeforeBody.fleet?.missions?.length ?? -2),
        "blocked strong-target launch does not create weak attacker fleet rows",
        {
          before: fleetRestrictionWeakBeforeBody.fleet?.missions?.length,
          after: fleetRestrictionWeakAfterBody.fleet?.missions?.length
        }
      ),
      check(!fleetRestrictionsReady || !fleetRestrictionNoobAttack.body.includes(fleetRestrictionAttackerLogin?.cookiePair ?? "missing-cookie"), "fleet restriction response does not echo attacker cookie"),
      check(!fleetRestrictionsReady || !fleetRestrictionAttackBan.body.includes(fleetRestrictionBlockedLogin?.cookiePair ?? "missing-cookie"), "fleet restriction response does not echo noattack attacker cookie")
    ]
  }));

  cases.push(finalize({
    case: "go_security_headers_cookie_flags",
    checks: [
      check(securityPublicHome.status === 200, "public home returns HTTP 200 for security header checks", { status: securityPublicHome.status }),
      check(!/Fatal error|Parse error|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i.test(securityPublicHome.body), "public home has no runtime error marker"),
      check(hasHeader(securityPublicHome, "x-frame-options", "SAMEORIGIN"), "public home sends SAMEORIGIN frame protection", securityPublicHome.headers),
      check(hasHeader(securityPublicHome, "x-content-type-options", "nosniff"), "public home sends nosniff header", securityPublicHome.headers),
      check(hasHeader(securityPublicHome, "referrer-policy", "same-origin"), "public home sends same-origin referrer policy", securityPublicHome.headers),
      check(hasHeader(securityPublicHome, "content-security-policy", "frame-ancestors 'self'"), "public home sends frame-ancestor CSP", securityPublicHome.headers),
      check(hasHeader(sessionLogin, "x-frame-options", "SAMEORIGIN"), "login response sends SAMEORIGIN frame protection", sessionLogin.headers),
      check(hasHeader(sessionLogin, "x-content-type-options", "nosniff"), "login response sends nosniff header", sessionLogin.headers),
      check(hasHeader(sessionLogin, "referrer-policy", "same-origin"), "login response sends same-origin referrer policy", sessionLogin.headers),
      check(hasHeader(sessionLogin, "content-security-policy", "frame-ancestors 'self'"), "login response sends frame-ancestor CSP", sessionLogin.headers),
      check(hasHeader(securityHTTPSHome, "strict-transport-security", "max-age=31536000"), "HTTPS-forwarded home sends HSTS", securityHTTPSHome.headers),
      check(/^prsess_\d+_1=/.test(sessionCookie), "login response names the private session cookie by player and universe", { setCookie: sessionCookie }),
      check(sessionCookie.includes("HttpOnly"), "private session cookie is HttpOnly", { setCookie: sessionCookie }),
      check(sessionCookie.includes("SameSite=Lax"), "private session cookie uses SameSite=Lax", { setCookie: sessionCookie }),
      check(sessionCookie.includes("Max-Age=86400"), "private session cookie keeps the 24h legacy lifetime", { setCookie: sessionCookie })
    ]
  }));

  cases.push(finalize({
    case: "go_legacy_get_mutation_noop",
    checks: [
      check(sentReportID > 0, "message no-op fixture exposes a report id", { sentReportID }),
      check(legacyGetMessageDelete.status === 200, "legacy GET message delete URL returns HTTP 200", { status: legacyGetMessageDelete.status }),
      check(legacyGetMessageDelete.body.includes('<div id="root">'), "legacy GET message delete URL is served by the React shell"),
      check(gameReportAfterLegacyGetDelete.status === 200, "owner can reload report after legacy GET delete URL", { status: gameReportAfterLegacyGetDelete.status }),
      check(gameReportAfterLegacyGetDeleteBody.report?.allowed === true, "legacy GET message delete URL does not remove owner report access", gameReportAfterLegacyGetDeleteBody.report ?? {}),
      check(String(gameReportAfterLegacyGetDeleteBody.report?.text ?? "").includes("Go migration message smoke"), "legacy GET message delete URL keeps message text", gameReportAfterLegacyGetDeleteBody.report ?? {}),
      check(legacyGetOptionsDeletion.status === 200, "legacy GET account deletion URL returns HTTP 200", { status: legacyGetOptionsDeletion.status }),
      check(legacyGetOptionsDeletion.body.includes('<div id="root">'), "legacy GET account deletion URL is served by the React shell"),
      check(gameOptionsAfterLegacyGet.status === 200, "options reloads after legacy GET account deletion URL", { status: gameOptionsAfterLegacyGet.status }),
      check(
        gameOptionsAfterLegacyGetBody.options?.account?.deletionQueued === gameOptionsBody.options?.account?.deletionQueued,
        "legacy GET account deletion URL does not change deletion state",
        {
          before: gameOptionsBody.options?.account ?? {},
          after: gameOptionsAfterLegacyGetBody.options?.account ?? {}
        }
      ),
      check(!createdAllianceId || legacyGetAllianceSettings.status === 200, "legacy GET alliance settings URL returns HTTP 200", { status: legacyGetAllianceSettings.status }),
      check(!createdAllianceId || legacyGetAllianceSettings.body.includes('<div id="root">'), "legacy GET alliance settings URL is served by the React shell"),
      check(!createdAllianceId || allianceSettingsAfterLegacyGet.status === 200, "alliance settings reloads after legacy GET settings URL", { status: allianceSettingsAfterLegacyGet.status }),
      check(
        !createdAllianceId ||
          (
            allianceSettingsBeforeLegacyGetBody.alliance?.own?.homepage === allianceSettingsAfterLegacyGetBody.alliance?.own?.homepage &&
            allianceSettingsBeforeLegacyGetBody.alliance?.own?.imageLogo === allianceSettingsAfterLegacyGetBody.alliance?.own?.imageLogo &&
            allianceSettingsBeforeLegacyGetBody.alliance?.own?.open === allianceSettingsAfterLegacyGetBody.alliance?.own?.open
          ),
        "legacy GET alliance settings URL does not update persisted settings",
        {
          before: allianceSettingsBeforeLegacyGetBody.alliance?.own ?? {},
          after: allianceSettingsAfterLegacyGetBody.alliance?.own ?? {}
        }
      )
    ]
  }));

  cases.push(finalize({
    case: "go_input_hardening_api",
    checks: [
      check(hardeningInvalidOverviewCP.status === 400, "overview rejects non-numeric selected planet", { status: hardeningInvalidOverviewCP.status, body: hardeningInvalidOverviewCP.body }),
      check(hardeningInvalidOverviewCP.body.includes("invalid selected planet"), "overview invalid planet response is explicit", { body: hardeningInvalidOverviewCP.body }),
      check(hardeningInvalidOptionsCP.status === 400, "options rejects non-numeric selected planet", { status: hardeningInvalidOptionsCP.status, body: hardeningInvalidOptionsCP.body }),
      check(hardeningInvalidOptionsCP.body.includes("invalid selected planet"), "options invalid planet response is explicit", { body: hardeningInvalidOptionsCP.body }),
      check(hardeningInvalidReportID.status === 400, "report rejects non-numeric report id", { status: hardeningInvalidReportID.status, body: hardeningInvalidReportID.body }),
      check(hardeningInvalidReportID.body.includes("invalid report id"), "report invalid id response is explicit", { body: hardeningInvalidReportID.body }),
      check(hardeningInvalidMessageTarget.status === 400, "messages rejects non-numeric compose target", { status: hardeningInvalidMessageTarget.status, body: hardeningInvalidMessageTarget.body }),
      check(hardeningInvalidMessageTarget.body.includes("invalid message target"), "message target response is explicit", { body: hardeningInvalidMessageTarget.body }),
      check(hardeningMalformedResources.status === 400, "resources rejects malformed JSON payload", { status: hardeningMalformedResources.status, body: hardeningMalformedResources.body }),
      check(hardeningMalformedResources.body.includes("invalid resource production request"), "resources malformed payload response is explicit", { body: hardeningMalformedResources.body }),
      check(hardeningMalformedOptions.status === 400, "options rejects malformed JSON payload", { status: hardeningMalformedOptions.status, body: hardeningMalformedOptions.body }),
      check(hardeningMalformedOptions.body.includes("invalid options request"), "options malformed payload response is explicit", { body: hardeningMalformedOptions.body }),
      check(hardeningUnknownAPI.status === 404, "unknown API route returns HTTP 404", { status: hardeningUnknownAPI.status }),
      check(!hardeningUnknownAPI.body.includes('id="root"'), "unknown API route is not swallowed by the React shell", { body: hardeningUnknownAPI.body })
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

  const operatorBroadcastMessage = Array.isArray(operatorMessagesAfterBroadcastBody.messages?.rows)
    ? operatorMessagesAfterBroadcastBody.messages.rows.find((row) =>
        String(row.subject ?? "").includes(String(adminOperationsFixture.token ?? "")) ||
        String(row.text ?? "").includes(String(adminOperationsFixture.token ?? ""))
      )
    : undefined;
  cases.push(finalize({
    case: "go_admin_operations_broadcast_reports_api",
    checks: [
      check(!smokeFixtureFile || adminOperationsReady, "go smoke fixture exposes admin operations report and token", {
        smokeFixtureFile,
        adminOperationsFixture
      }),
      check(!adminOperationsReady || operatorLogin?.response.status === 200, "operator smoke user can log in for admin operation checks", {
        status: operatorLogin?.response.status
      }),
      check(!adminOperationsReady || adminReportsBeforeDelete?.status === 200, "admin Reports GET returns HTTP 200", {
        status: adminReportsBeforeDelete?.status
      }),
      check(
        !adminOperationsReady ||
          adminReportSeedRow?.subject?.includes(String(adminOperationsFixture.token)) ||
          adminReportSeedRow?.text?.includes(String(adminOperationsFixture.token)),
        "admin Reports GET renders the seeded report marker",
        { adminReportSeedRow }
      ),
      check(!adminOperationsReady || operatorReportsDelete?.status === 200, "operator Reports delete mutation returns HTTP 200", {
        status: operatorReportsDelete?.status
      }),
      check(
        !adminOperationsReady || operatorReportsDeleteBody.actionIssue?.code === "action_saved",
        "operator Reports delete mutation saves like legacy",
        operatorReportsDeleteBody.actionIssue ?? {}
      ),
      check(!adminOperationsReady || adminReportsAfterDelete?.status === 200, "admin Reports reload after delete returns HTTP 200", {
        status: adminReportsAfterDelete?.status
      }),
      check(!adminOperationsReady || adminReportDeletedRow === undefined, "operator Reports delete removes the seeded report", {
        adminReportDeletedRow
      }),
      check(!adminOperationsReady || adminBroadcast?.status === 200, "admin Broadcast mutation returns HTTP 200", {
        status: adminBroadcast?.status
      }),
      check(
        !adminOperationsReady || adminBroadcastBody.actionIssue?.code === "action_saved",
        "admin Broadcast mutation saves like legacy",
        adminBroadcastBody.actionIssue ?? {}
      ),
      check(!adminOperationsReady || operatorMessagesAfterBroadcast?.status === 200, "operator inbox reload after broadcast returns HTTP 200", {
        status: operatorMessagesAfterBroadcast?.status
      }),
      check(!adminOperationsReady || operatorBroadcastMessage !== undefined, "operator category broadcast creates a marker message for the operator", {
        operatorBroadcastMessage
      }),
      check(!adminOperationsReady || !String(operatorMessagesAfterBroadcast?.body ?? "").includes(operatorLogin?.cookiePair ?? "missing-cookie"), "admin operations responses do not echo private cookie")
    ]
  }));

  cases.push(finalize({
    case: "go_admin_operations_regular_denial_api",
    checks: regularAdminOperationDenials.flatMap((item) => [
      check(item.response.status === 200, `regular user ${item.mode} admin request returns HTTP 200`, {
        status: item.response.status
      }),
      check(item.body.authenticated === true, `regular user ${item.mode} admin request authenticates session`, item.body),
      check(item.body.admin?.mode === item.mode, `regular user ${item.mode} admin request resolves legacy mode`, item.body.admin ?? {}),
      check(item.body.actionIssue?.code === "access_denied", `regular user ${item.mode} admin request is denied like legacy`, item.body.actionIssue ?? {})
    ])
  }));

  cases.push(finalize({
    case: "go_admin_expedition_settings_permission_api",
    checks: [
      check(!smokeFixtureFile || operatorLogin?.response.status === 200, "operator smoke user can log in for expedition settings permission check", {
        status: operatorLogin?.response.status
      }),
      check(!smokeFixtureFile || adminExpeditionBeforeSettings?.status === 200, "admin Expedition settings GET returns HTTP 200", {
        status: adminExpeditionBeforeSettings?.status
      }),
      check(!smokeFixtureFile || adminExpeditionSettingsReady, "admin Expedition settings expose chance_success", {
        expedition: adminExpeditionBeforeSettingsBody.admin?.expedition
      }),
      check(
        !smokeFixtureFile || !adminExpeditionSettingsReady || operatorExpeditionSettings?.status === 200,
        "operator Expedition settings mutation returns HTTP 200",
        { status: operatorExpeditionSettings?.status }
      ),
      check(
        !smokeFixtureFile || !adminExpeditionSettingsReady || operatorExpeditionSettingsBody.actionIssue?.code === "access_denied",
        "operator Expedition settings mutation is denied like legacy",
        operatorExpeditionSettingsBody.actionIssue ?? {}
      ),
      check(
        !smokeFixtureFile ||
          !adminExpeditionSettingsReady ||
          Number(adminExpeditionAfterOperatorSettingsBody.admin?.expedition?.chance_success) === originalExpeditionChance,
        "operator Expedition settings mutation does not alter chance_success",
        {
          originalExpeditionChance,
          afterOperator: adminExpeditionAfterOperatorSettingsBody.admin?.expedition?.chance_success
        }
      ),
      check(!smokeFixtureFile || !adminExpeditionSettingsReady || adminExpeditionSettings?.status === 200, "admin Expedition settings mutation returns HTTP 200", {
        status: adminExpeditionSettings?.status
      }),
      check(
        !smokeFixtureFile || !adminExpeditionSettingsReady || adminExpeditionSettingsBody.actionIssue?.code === "action_saved",
        "admin Expedition settings mutation saves like legacy",
        adminExpeditionSettingsBody.actionIssue ?? {}
      ),
      check(
        !smokeFixtureFile ||
          !adminExpeditionSettingsReady ||
          Number(adminExpeditionAfterAdminSettingsBody.admin?.expedition?.chance_success) === adminExpeditionChance,
        "admin Expedition settings mutation updates chance_success",
        {
          adminExpeditionChance,
          afterAdmin: adminExpeditionAfterAdminSettingsBody.admin?.expedition?.chance_success
        }
      ),
      check(
        !smokeFixtureFile || !adminExpeditionSettingsReady || adminExpeditionRestoreBody.actionIssue?.code === "action_saved",
        "admin Expedition settings restore saves the original value",
        adminExpeditionRestoreBody.actionIssue ?? {}
      ),
      check(
        !smokeFixtureFile ||
          !adminExpeditionSettingsReady ||
          Number(adminExpeditionAfterRestoreBody.admin?.expedition?.chance_success) === originalExpeditionChance,
        "admin Expedition settings restore returns chance_success to the original value",
        {
          originalExpeditionChance,
          afterRestore: adminExpeditionAfterRestoreBody.admin?.expedition?.chance_success
        }
      )
    ]
  }));

  const frozenQueueRow = Array.isArray(adminQueueAfterFreezeBody.admin?.queueRows)
    ? adminQueueAfterFreezeBody.admin.queueRows.find((row) => Number(row.id) === adminQueueTaskId)
    : undefined;
  cases.push(finalize({
    case: "go_admin_queue_permission_mutation_api",
    checks: [
      check(!smokeFixtureFile || adminQueueFixtureReady, "go smoke fixture exposes admin queue task id", {
        smokeFixtureFile,
        adminQueueFixture
      }),
      check(!adminQueueFixtureReady || operatorLogin?.response.status === 200, "operator smoke user can log in for admin permission check", {
        status: operatorLogin?.response.status
      }),
      check(!adminQueueFixtureReady || operatorQueueFreeze?.status === 200, "operator queue mutation returns HTTP 200", {
        status: operatorQueueFreeze?.status
      }),
      check(
        !adminQueueFixtureReady || operatorQueueFreezeBody.actionIssue?.code === "access_denied",
        "operator queue mutation is denied like legacy",
        operatorQueueFreezeBody
      ),
      check(!adminQueueFixtureReady || adminQueueFreeze?.status === 200, "admin queue mutation returns HTTP 200", {
        status: adminQueueFreeze?.status
      }),
      check(
        !adminQueueFixtureReady || adminQueueFreezeBody.actionIssue?.code === "action_saved",
        "admin queue mutation saves like legacy",
        adminQueueFreezeBody.actionIssue ?? {}
      ),
      check(!adminQueueFixtureReady || adminQueueAfterFreeze?.status === 200, "admin queue reload returns HTTP 200", {
        status: adminQueueAfterFreeze?.status
      }),
      check(
        !adminQueueFixtureReady || frozenQueueRow?.freeze === true,
        "admin queue freeze actually updates the target task",
        { taskId: adminQueueTaskId, frozenQueueRow }
      )
    ]
  }));

  const fleetlogControlRow = Array.isArray(adminFleetlogsAfterTwoMinuteBody.admin?.fleetLogRows)
    ? adminFleetlogsAfterTwoMinuteBody.admin.fleetLogRows.find((row) => Number(row.taskId) === adminFleetlogsTaskId)
    : undefined;
  const fleetlogRowsAfterReturn = Array.isArray(adminFleetlogsAfterReturnBody.admin?.fleetLogRows)
    ? adminFleetlogsAfterReturnBody.admin.fleetLogRows
    : [];
  const recalledFleetlogTaskRow = fleetlogRowsAfterReturn.find((row) => Number(row.taskId) === adminFleetlogsRecallTaskId);
  const returnFleetlogRow = fleetlogRowsAfterReturn.find((row) => Number(row.mission) === legacyTransportReturnMission && Number(row.origin?.ownerId ?? 0) === Number(smokeFixture?.phalanx?.target_player_id ?? 0));
  cases.push(finalize({
    case: "go_admin_fleetlogs_permission_mutation_api",
    checks: [
      check(!smokeFixtureFile || adminFleetlogsFixtureReady, "go smoke fixture exposes admin fleetlogs task id", {
        smokeFixtureFile,
        adminFleetlogsFixture
      }),
      check(!adminFleetlogsFixtureReady || operatorLogin?.response.status === 200, "operator smoke user is available for fleetlogs permission check", {
        status: operatorLogin?.response.status
      }),
      check(!adminFleetlogsFixtureReady || operatorFleetlogsTwoMinute?.status === 200, "operator fleetlogs mutation returns HTTP 200", {
        status: operatorFleetlogsTwoMinute?.status
      }),
      check(
        !adminFleetlogsFixtureReady || operatorFleetlogsTwoMinuteBody.actionIssue?.code === "access_denied",
        "operator fleetlogs mutation is denied like legacy",
        operatorFleetlogsTwoMinuteBody
      ),
      check(!adminFleetlogsFixtureReady || adminFleetlogsTwoMinute?.status === 200, "admin fleetlogs mutation returns HTTP 200", {
        status: adminFleetlogsTwoMinute?.status
      }),
      check(
        !adminFleetlogsFixtureReady || adminFleetlogsTwoMinuteBody.actionIssue?.code === "action_saved",
        "admin fleetlogs mutation saves like legacy",
        adminFleetlogsTwoMinuteBody.actionIssue ?? {}
      ),
      check(!adminFleetlogsFixtureReady || adminFleetlogsAfterTwoMinute?.status === 200, "admin fleetlogs reload returns HTTP 200", {
        status: adminFleetlogsAfterTwoMinute?.status
      }),
      check(
        !adminFleetlogsFixtureReady ||
          (Number(fleetlogControlRow?.end ?? 0) >= adminFleetlogsTwoMinuteStartedAt + 110 &&
            Number(fleetlogControlRow?.end ?? 0) <= adminFleetlogsTwoMinuteStartedAt + 180),
        "admin fleetlogs 2m action updates the target task end time",
        { taskId: adminFleetlogsTaskId, startedAt: adminFleetlogsTwoMinuteStartedAt, fleetlogControlRow }
      )
    ]
  }));

  cases.push(finalize({
    case: "go_admin_fleetlogs_return_api",
    checks: [
      check(!smokeFixtureFile || adminFleetlogsRecallFixtureReady, "go smoke fixture exposes admin fleetlogs recall task id", {
        smokeFixtureFile,
        adminFleetlogsFixture
      }),
      check(!adminFleetlogsRecallFixtureReady || adminFleetlogsReturn?.status === 200, "admin fleetlogs return mutation returns HTTP 200", {
        status: adminFleetlogsReturn?.status
      }),
      check(
        !adminFleetlogsRecallFixtureReady || adminFleetlogsReturnBody.actionIssue?.code === "action_saved",
        "admin fleetlogs return mutation saves like legacy",
        adminFleetlogsReturnBody.actionIssue ?? {}
      ),
      check(!adminFleetlogsRecallFixtureReady || adminFleetlogsAfterReturn?.status === 200, "admin fleetlogs return reload returns HTTP 200", {
        status: adminFleetlogsAfterReturn?.status
      }),
      check(
        !adminFleetlogsRecallFixtureReady || recalledFleetlogTaskRow === undefined,
        "admin fleetlogs return removes the original fleet queue task",
        { taskId: adminFleetlogsRecallTaskId, recalledFleetlogTaskRow }
      ),
      check(
        !adminFleetlogsRecallFixtureReady || returnFleetlogRow !== undefined,
        "admin fleetlogs return creates a legacy transport return fleet row",
        { mission: legacyTransportReturnMission, returnFleetlogRow }
      )
    ]
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
  cases.push(finalize({
    case: "go_alliance_management_lifecycle_api",
    checks: [
      check(allianceFounderLogin.response.status === 200, "founder smoke user can log in for alliance lifecycle", { status: allianceFounderLogin.response.status }),
      check(targetLogin.response.status === 200, "target smoke user can log in for alliance lifecycle", { status: targetLogin.response.status }),
      check(targetLogin.playerId > 0, "target smoke login exposes a player id", { playerId: targetLogin.playerId }),
      check(allianceCreate.status === 200, "founder creates an alliance through Go API", { status: allianceCreate.status }),
      check(allianceCreateBody.actionIssue?.code === "created", "alliance create returns created issue", allianceCreateBody),
      check(createdAllianceId > 0 && allianceCreateBody.alliance?.own?.tag === allianceTag, "created alliance is returned with the requested tag", allianceCreateBody.alliance?.own ?? {}),
      check(allianceApply.status === 200, "target applies to created alliance through Go API", { status: allianceApply.status }),
      check(allianceApplyBody.actionIssue?.code === "applied", "alliance application returns applied issue", allianceApplyBody),
      check(applicationId > 0, "alliance application exposes pending application id", allianceApplyBody.alliance?.pending ?? {}),
      check(allianceAccept.status === 200, "founder accepts target application through Go API", { status: allianceAccept.status }),
      check(allianceAcceptBody.actionIssue?.code === "accepted", "alliance accept returns accepted issue", allianceAcceptBody),
      check(allianceRankCreate.status === 200, "founder creates a custom rank through Go API", { status: allianceRankCreate.status }),
      check(createdRankId > 1, "custom rank is returned after creation", allianceRankCreateBody.alliance?.ranks ?? []),
      check(allianceRankRights.status === 200, "founder saves custom rank rights through Go API", { status: allianceRankRights.status }),
      check(rankAfterRights?.rights === rankRights, "custom rank receives member list, management, and circular rights", {
        expected: rankRights,
        actual: rankAfterRights
      }),
      check(allianceAssignRank.status === 200, "founder assigns custom rank to target member through Go API", { status: allianceAssignRank.status }),
      check(assignedMember?.rankId === createdRankId, "assigned member reloads with the custom rank", assignedMember ?? {}),
      check(allianceCircular.status === 200, "ranked member sends a rank-scoped circular message through Go API", { status: allianceCircular.status }),
      check(allianceCircularBody.actionIssue?.code === "sent", "circular send returns sent issue", allianceCircularBody),
      check(
        Array.isArray(allianceCircularBody.alliance?.circularResult?.recipients) &&
          allianceCircularBody.alliance.circularResult.recipients.length === 1,
        "rank-scoped circular lists exactly the selected-rank recipient",
        allianceCircularBody.alliance?.circularResult ?? {}
      ),
      check(targetMessagesAfterCircular.status === 200, "target messages reload after circular send", { status: targetMessagesAfterCircular.status }),
      check(
        (targetMessagesAfterCircularBody.messages?.rows ?? []).some((row) => String(row.text ?? "").includes(circularText)),
        "target inbox contains the circular alliance message",
        targetMessagesAfterCircularBody.messages?.rows ?? []
      ),
      check(!allianceCircular.body.includes(targetLogin.cookiePair), "alliance circular response does not echo target private cookie")
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

  const legacyGamePagePaths = [
    "/game",
    `/game/index.php?page=overview${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=renameplanet${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=b_building${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=buildings&mode=Forschung${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=buildings&mode=Flotte${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=buildings&mode=Verteidigung${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=resources${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=imperium${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=trader${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=micropayment${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=allianzen${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=admin${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=flotten1${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=fleet_templates${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=galaxy&galaxy=1&system=1${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=techtree${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=infos&gid=1${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=statistics${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=suche${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=buddy${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=messages${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=writemessages&messageziel=${loginPlayerId}${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=bericht&bericht=${sentReportID || 1}${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=notizen${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=options${sessionSearch.replace("?", "&")}`,
    `/game/index.php?page=phalanx&spid=${phalanxTargetPlanetID || basePlanetID || 1}${sessionSearch.replace("?", "&")}`
  ];
  const legacyGameRouteChecks = [];
  for (const path of legacyGamePagePaths) {
    const response = await request(path);
    legacyGameRouteChecks.push(
      check(response.status === 200, `${path} returns React shell`, { status: response.status }),
      check(response.body.includes('<div id="root">'), `${path} renders React mount node`),
      check(!/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i.test(response.body), `${path} has no legacy runtime error marker`),
      check(!response.body.includes("Master Database Settings"), `${path} does not render installer form`),
      check(noLoopbackAsset(response.body), `${path} does not emit localhost/127.0.0.1 absolute asset URLs`)
    );
  }
  cases.push(finalize({
    case: "go_legacy_game_route_matrix",
    checks: legacyGameRouteChecks
  }));

  const renderAssetDocuments = [
    { path: "/", response: root },
    { path: "/home", response: await request("/home") },
    { path: "/home.php", response: await request("/home.php") },
    { path: "/game/overview", response: fallback },
    {
      path: `/game/index.php?page=overview${sessionSearch.replace("?", "&")}`,
      response: await request(`/game/index.php?page=overview${sessionSearch.replace("?", "&")}`)
    }
  ];
  const renderAssetPaths = new Set();
  const renderAssetDocumentChecks = [];
  for (const { path, response } of renderAssetDocuments) {
    const assets = extractSameOriginAssets(path, response.body);
    for (const assetPath of assets) {
      renderAssetPaths.add(assetPath);
    }
    renderAssetDocumentChecks.push(
      check(response.status === 200, `${path} render asset source returns HTTP 200`, { status: response.status }),
      check(response.body.includes('<div id="root">'), `${path} render asset source is a React document`),
      check(!response.body.includes("Master Database Settings"), `${path} render asset source skips installer`),
      check(noLoopbackAsset(response.body), `${path} render asset source has no loopback absolute asset URLs`),
      check(assets.length > 0, `${path} exposes at least one same-origin asset`, { assets })
    );
  }
  const renderAssetChecks = [
    ...renderAssetDocumentChecks,
    check(renderAssetPaths.size > 0, "rendered shell documents expose same-origin assets", {
      assetCount: renderAssetPaths.size
    })
  ];
  for (const assetPath of Array.from(renderAssetPaths).slice(0, 80)) {
    const assetResponse = await request(assetPath);
    renderAssetChecks.push(
      check(assetResponse.status === 200, "referenced render asset returns HTTP 200", {
        assetPath,
        status: assetResponse.status,
        contentType: assetResponse.headers["content-type"]
      }),
      check(assetResponse.body.length > 0, "referenced render asset is non-empty", {
        assetPath,
        size: assetResponse.body.length
      }),
      check(!looksLikeHTML(assetResponse), "referenced render asset is not an HTML fallback", {
        assetPath,
        contentType: assetResponse.headers["content-type"],
        bodyStart: assetResponse.body.slice(0, 80)
      })
    );
  }
  cases.push(finalize({
    case: "go_render_asset_smoke",
    checks: renderAssetChecks
  }));

  const js = await request("/assets/main.js");
  const css = await request("/assets/main.css");
  const legacyGameOverviewSource = await Bun.file(new URL("../../frontend/src/LegacyGameOverview.tsx", import.meta.url)).text();
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
  const putLegacyRegistration = await request("/game/reg/newredirect.php", { method: "PUT" });
  const postActivation = await request("/game/validate.php?ack=missing", { method: "POST" });
  const getLoginValidation = await request("/api/public/login/validate");
  const getLoginSubmit = await request("/api/public/login");
  const postLegacyPasswordForm = await request("/game/reg/mail.php", { method: "POST" });
  const getLegacyPasswordSubmit = await request("/game/reg/fa_pass.php");
  const postLegacyRedirect = await request("/game/redir.php", { method: "POST" });
  const postLegacyPic = await request("/game/pic.php", { method: "POST" });
  const postFeedShow = await request("/game/feed/show.php", { method: "POST" });
  const postFeedItem = await request("/game/feed/viewitem.php", { method: "POST" });
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
      check(putLegacyRegistration.status === 405, "PUT legacy registration redirect endpoint is rejected", { status: putLegacyRegistration.status }),
      check(hasHeader(putLegacyRegistration, "allow", "GET, POST"), "legacy registration redirect method rejection returns Allow header"),
      check(postActivation.status === 405, "POST registration activation endpoint is rejected", { status: postActivation.status }),
      check(hasHeader(postActivation, "allow", "GET, HEAD"), "registration activation method rejection returns Allow header"),
      check(getLoginValidation.status === 405, "GET login validation endpoint is rejected", { status: getLoginValidation.status }),
      check(hasHeader(getLoginValidation, "allow", "POST"), "login validation method rejection returns Allow header"),
      check(getLoginSubmit.status === 405, "GET login submit endpoint is rejected", { status: getLoginSubmit.status }),
      check(hasHeader(getLoginSubmit, "allow", "POST"), "login submit method rejection returns Allow header"),
      check(postLegacyPasswordForm.status === 405, "POST legacy password recovery form endpoint is rejected", { status: postLegacyPasswordForm.status }),
      check(hasHeader(postLegacyPasswordForm, "allow", "GET, HEAD"), "legacy password recovery form method rejection returns Allow header"),
      check(getLegacyPasswordSubmit.status === 405, "GET legacy password recovery submit endpoint is rejected", { status: getLegacyPasswordSubmit.status }),
      check(hasHeader(getLegacyPasswordSubmit, "allow", "POST"), "legacy password recovery submit method rejection returns Allow header"),
      check(postLegacyRedirect.status === 405, "POST legacy redirect endpoint is rejected", { status: postLegacyRedirect.status }),
      check(hasHeader(postLegacyRedirect, "allow", "GET, HEAD"), "legacy redirect method rejection returns Allow header"),
      check(postLegacyPic.status === 405, "POST legacy image proxy endpoint is rejected", { status: postLegacyPic.status }),
      check(hasHeader(postLegacyPic, "allow", "GET, HEAD"), "legacy image proxy method rejection returns Allow header"),
      check(postFeedShow.status === 405, "POST legacy feed endpoint is rejected", { status: postFeedShow.status }),
      check(hasHeader(postFeedShow, "allow", "GET, HEAD"), "legacy feed method rejection returns Allow header"),
      check(postFeedItem.status === 405, "POST legacy feed item endpoint is rejected", { status: postFeedItem.status }),
      check(hasHeader(postFeedItem, "allow", "GET, HEAD"), "legacy feed item method rejection returns Allow header"),
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
