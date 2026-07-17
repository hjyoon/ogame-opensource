import { createHash, randomBytes } from "node:crypto";
import { mcpGuideToolGroups } from "../../frontend/src/mcpGuide.ts";

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

const baseUrl = envURL(argValue("--go-base-url", process.env.OGAME_GO_BASE_URL), "http://127.0.0.1:8890");
const loginUser = argValue("--login-user", process.env.OGAME_GO_LOGIN_SMOKE_USER ?? "legor");
const loginPassword = argValue("--login-pass", process.env.OGAME_GO_LOGIN_SMOKE_PASS ?? "admin");

function check(pass, message, context = {}) {
  return { pass, message, context };
}

function finalize(testCase) {
  testCase.pass = testCase.checks.every((item) => item.pass === true);
  return testCase;
}

async function request(path, options = {}) {
  const targetURL = `${baseUrl}${path}`;
  let response;
  try {
    response = await fetch(targetURL, { redirect: "manual", ...options });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`request failed for ${path} (${targetURL}): ${message}`);
  }
  return {
    status: response.status,
    headers: Object.fromEntries(response.headers.entries()),
    body: await response.text()
  };
}

function parseJSON(response) {
  try {
    return JSON.parse(response.body);
  } catch {
    return {};
  }
}

function hasHeader(response, name, expected) {
  const actual = response.headers[name.toLowerCase()] ?? "";
  return expected === undefined ? actual !== "" : actual.toLowerCase().includes(expected.toLowerCase());
}

function hiddenInputValue(html, name) {
  const escapedName = String(name).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`<input[^>]+name="${escapedName}"[^>]+value="([^"]*)"`, "i").exec(String(html ?? ""));
  return match?.[1] ?? "";
}

function toolNames(body) {
  return Array.isArray(body.result?.tools)
    ? body.result.tools.map((tool) => String(tool.name ?? "")).filter((name) => name !== "")
    : [];
}

function pkceVerifier() {
  return randomBytes(32).toString("base64url");
}

function pkceChallenge(verifier) {
  return createHash("sha256").update(verifier).digest("base64url");
}

function decodeJWTClaims(token) {
  const parts = String(token ?? "").split(".");
  if (parts.length !== 3) {
    return {};
  }
  try {
    return JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8"));
  } catch {
    return {};
  }
}

async function mcpJSONRPC(method, params, options = {}) {
  const { id, headers: extraHeaders = {}, ...requestOptions } = options;
  const body = { jsonrpc: "2.0", id: id ?? 1, method };
  if (params !== undefined) {
    body.params = params;
  }
  return request("/mcp", {
    ...requestOptions,
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      "MCP-Protocol-Version": "2025-06-18",
      ...extraHeaders
    },
    body: JSON.stringify(body)
  });
}

async function loginGameUser(universe) {
  const response = await request("/api/public/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ login: loginUser, pass: loginPassword, universe })
  });
  const body = parseJSON(response);
  const cookie = response.headers["set-cookie"] ?? "";
  const cookiePair = cookie.split(";")[0] ?? "";
  const playerID = Number(/^prsess_(\d+)_/.exec(cookiePair.split("=")[0] ?? "")?.[1] ?? 0);
  const search = typeof body.session?.redirectTo === "string"
    ? new URL(body.session.redirectTo, baseUrl).search
    : "?session=";
  return { response, body, cookiePair, playerID, search };
}

const cases = [];

try {
  const health = await request("/api/healthz");
  const healthBody = parseJSON(health);
  const universesResponse = await request("/api/public/universes");
  const universesBody = parseJSON(universesResponse);
  const universe = universesBody.universes?.[0]?.baseUrl ?? "http://localhost:8888";
  cases.push(finalize({
    case: "go_mcp_smoke_prerequisites",
    checks: [
      check(health.status === 200 && healthBody.status === "ok", "Go health endpoint is available", {
        status: health.status,
        body: healthBody
      }),
      check(universesResponse.status === 200 && Array.isArray(universesBody.universes) && universesBody.universes.length > 0, "universe catalog is available for login", universesBody)
    ]
  }));

  const mcpGet = await request("/mcp");
  const mcpBadOrigin = await request("/mcp", {
    method: "POST",
    headers: { "Content-Type": "application/json", Origin: "http://evil.example" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "initialize" })
  });
  const mcpInvalidProtocol = await request("/mcp", {
    method: "POST",
    headers: { "Content-Type": "application/json", "MCP-Protocol-Version": "2024-01-01" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 2, method: "initialize" })
  });
  const mcpParseError = await request("/mcp", {
    method: "POST",
    headers: { "Content-Type": "application/json", "MCP-Protocol-Version": "2025-06-18" },
    body: "{"
  });
  const mcpInitialize = await mcpJSONRPC("initialize", {
    protocolVersion: "2025-06-18",
    capabilities: {},
    clientInfo: { name: "go-mcp-smoke", version: "1" }
  }, { id: 3 });
  const mcpPing = await mcpJSONRPC("ping", {}, { id: 4 });
  const mcpListAnon = await mcpJSONRPC("tools/list", {}, { id: 5 });
  const mcpHealthTool = await mcpJSONRPC("tools/call", { name: "get_server_health", arguments: {} }, { id: 6 });
  const mcpUnauthorizedAccess = await mcpJSONRPC("tools/call", { name: "get_mcp_access", arguments: {} }, { id: 7 });
  const oauthMetadata = await request("/.well-known/oauth-authorization-server");
  const protectedResourceMetadata = await request("/.well-known/oauth-protected-resource");
  const oauthJWKS = await request("/.well-known/jwks.json");
  const oauthTokenUnavailable = await request("/oauth/token", { method: "POST" });
  const oauthRevokeUnavailable = await request("/oauth/revoke", { method: "POST" });
  const oauthExternalRedirectReject = await request(`/oauth/authorize?response_type=code&client_id=external&redirect_uri=${encodeURIComponent("https://client.example/callback")}&resource=${encodeURIComponent(`${baseUrl}/mcp`)}&scope=mcp:read&code_challenge=${"a".repeat(43)}&code_challenge_method=S256`);
  const mcpParseErrorBody = parseJSON(mcpParseError);
  const mcpInitializeBody = parseJSON(mcpInitialize);
  const mcpPingBody = parseJSON(mcpPing);
  const mcpListAnonBody = parseJSON(mcpListAnon);
  const mcpHealthToolBody = parseJSON(mcpHealthTool);
  const mcpUnauthorizedAccessBody = parseJSON(mcpUnauthorizedAccess);
  const oauthMetadataBody = parseJSON(oauthMetadata);
  const protectedResourceMetadataBody = parseJSON(protectedResourceMetadata);
  const oauthJWKSBody = parseJSON(oauthJWKS);
  const oauthTokenUnavailableBody = parseJSON(oauthTokenUnavailable);
  const oauthRevokeUnavailableBody = parseJSON(oauthRevokeUnavailable);
  const oauthExternalRedirectRejectBody = parseJSON(oauthExternalRedirectReject);
  cases.push(finalize({
    case: "go_mcp_public_transport",
    checks: [
      check(mcpGet.status === 405, "GET /mcp rejects unavailable stream transport", { status: mcpGet.status }),
      check(hasHeader(mcpGet, "allow", "POST"), "GET /mcp returns POST Allow header"),
      check(mcpBadOrigin.status === 403, "MCP rejects cross-origin browser requests", { status: mcpBadOrigin.status }),
      check(mcpInvalidProtocol.status === 400, "MCP rejects unsupported protocol versions", { status: mcpInvalidProtocol.status }),
      check(mcpParseError.status === 400 && mcpParseErrorBody.error?.code === -32700, "MCP invalid JSON returns JSON-RPC parse error", mcpParseErrorBody),
      check(mcpInitialize.status === 200 && mcpInitializeBody.result?.protocolVersion === "2025-06-18", "MCP initialize succeeds", mcpInitializeBody.result ?? {}),
      check(mcpInitializeBody.result?.serverInfo?.name === "ogame-opensource", "MCP initialize reports server identity", mcpInitializeBody.result?.serverInfo ?? {}),
      check(mcpPing.status === 200 && mcpPingBody.error === undefined, "MCP ping succeeds", mcpPingBody),
      check(mcpListAnon.status === 200 && toolNames(mcpListAnonBody).join(",") === "get_server_health", "anonymous tools/list exposes only public health", {
        tools: toolNames(mcpListAnonBody)
      }),
      check(mcpHealthTool.status === 200 && mcpHealthToolBody.result?.structuredContent?.status === "ok", "public get_server_health tool returns structured status", mcpHealthToolBody.result ?? {}),
      check(mcpUnauthorizedAccess.status === 401 && mcpUnauthorizedAccessBody.error?.code === -32001, "protected tool without token returns Unauthorized", mcpUnauthorizedAccessBody),
      check(hasHeader(mcpUnauthorizedAccess, "www-authenticate", "Bearer"), "Unauthorized MCP response includes Bearer challenge"),
      check(hasHeader(mcpUnauthorizedAccess, "www-authenticate", `${baseUrl}/.well-known/oauth-protected-resource`), "Unauthorized MCP response advertises protected resource metadata"),
      check(oauthMetadata.status === 200 && oauthMetadataBody.issuer === baseUrl, "OAuth authorization server metadata uses current origin issuer", oauthMetadataBody),
      check(protectedResourceMetadata.status === 200 && protectedResourceMetadataBody.resource === `${baseUrl}/mcp` && (protectedResourceMetadataBody.authorization_servers ?? []).includes(baseUrl), "Protected resource metadata advertises MCP resource and issuer", protectedResourceMetadataBody),
      check(oauthMetadataBody.authorization_endpoint === `${baseUrl}/oauth/authorize`, "OAuth metadata exposes authorize endpoint", oauthMetadataBody),
      check(oauthMetadataBody.registration_endpoint === `${baseUrl}/oauth/register`, "OAuth metadata exposes dynamic client registration endpoint", oauthMetadataBody),
      check(oauthMetadataBody.revocation_endpoint === `${baseUrl}/oauth/revoke`, "OAuth metadata exposes revocation endpoint", oauthMetadataBody),
      check((oauthMetadataBody.code_challenge_methods_supported ?? []).includes("S256"), "OAuth metadata requires PKCE S256 support", oauthMetadataBody),
      check(oauthMetadataBody.jwks_uri === `${baseUrl}/.well-known/jwks.json` && (oauthMetadataBody.id_token_signing_alg_values_supported ?? []).includes("EdDSA"), "OAuth metadata exposes OIDC JWKS and EdDSA", oauthMetadataBody),
      check(oauthJWKS.status === 200 && (oauthJWKSBody.keys ?? []).some((key) => key.kty === "OKP" && key.crv === "Ed25519"), "OIDC JWKS exposes Ed25519 signing key", oauthJWKSBody),
      check(oauthTokenUnavailable.status === 400 && oauthTokenUnavailableBody.error === "invalid_request", "OAuth token endpoint rejects malformed exchange requests", oauthTokenUnavailableBody),
      check(oauthRevokeUnavailable.status === 400 && oauthRevokeUnavailableBody.error === "invalid_request", "OAuth revoke endpoint rejects missing token requests", oauthRevokeUnavailableBody),
      check(oauthExternalRedirectReject.status === 400 && oauthExternalRedirectRejectBody.error === "invalid_request", "OAuth authorize rejects non-loopback redirects without allow-list", oauthExternalRedirectRejectBody)
    ]
  }));

  const login = await loginGameUser(universe);
  const sessionID = new URLSearchParams(login.search.startsWith("?") ? login.search.slice(1) : login.search).get("session") ?? "";
  const oauthVerifier = pkceVerifier();
  const oauthRedirectURI = `${baseUrl}/oauth/callback`;
  const clientRegistration = await request("/oauth/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      redirect_uris: [oauthRedirectURI],
      client_name: "Go MCP smoke",
      scope: "openid profile mcp:read mcp:messages mcp:message_write mcp:notes_write mcp:buddy_write mcp:fleet mcp:fleet_write mcp:queue_write mcp:resources_write mcp:premium_write mcp:merchant_write mcp:planet_write mcp:alliance_write mcp:account_write mcp:payment_write"
    })
  });
  const clientRegistrationBody = parseJSON(clientRegistration);
  const oauthClientID = String(clientRegistrationBody.client_id ?? `go-mcp-smoke-${Date.now().toString(36)}`);
  const oauthParams = new URLSearchParams({
    response_type: "code",
    client_id: oauthClientID,
    redirect_uri: oauthRedirectURI,
    resource: `${baseUrl}/mcp`,
    scope: "openid profile mcp:read mcp:messages mcp:message_write mcp:notes_write mcp:buddy_write mcp:fleet mcp:fleet_write mcp:queue_write mcp:resources_write mcp:premium_write mcp:merchant_write mcp:planet_write mcp:alliance_write mcp:account_write mcp:payment_write",
    state: "go-mcp-smoke-state",
    code_challenge: pkceChallenge(oauthVerifier),
    code_challenge_method: "S256",
    session: sessionID
  });
  const oauthConsent = await request(`/oauth/authorize?${oauthParams}`, {
    headers: { Cookie: login.cookiePair }
  });
  const oauthApproveParams = new URLSearchParams(oauthParams);
  oauthApproveParams.set("consent", "approve");
  oauthApproveParams.set("consent_token", hiddenInputValue(oauthConsent.body, "consent_token"));
  const oauthApprove = await request(`/oauth/authorize?${oauthApproveParams}`, {
    headers: { Cookie: login.cookiePair }
  });
  const oauthApproveLocation = oauthApprove.headers.location ?? "";
  const oauthCallback = oauthApproveLocation !== "" ? new URL(oauthApproveLocation) : new URL(`${oauthRedirectURI}?code=`);
  const oauthCode = oauthCallback.searchParams.get("code") ?? "";
  const oauthToken = await request("/oauth/token", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code: oauthCode,
      redirect_uri: oauthRedirectURI,
      resource: `${baseUrl}/mcp`,
      client_id: oauthClientID,
      code_verifier: oauthVerifier
    }).toString()
  });
  const oauthTokenBody = parseJSON(oauthToken);
  const oauthSecret = String(oauthTokenBody.access_token ?? "");
  const oauthIDToken = String(oauthTokenBody.id_token ?? "");
  const oauthIDClaims = decodeJWTClaims(oauthIDToken);
  const oauthTools = await mcpJSONRPC("tools/list", {}, { id: 18, headers: { Authorization: `Bearer ${oauthSecret}` } });
  const oauthToolsBody = parseJSON(oauthTools);
  const oauthTokenList = await request(`/api/game/mcp-tokens${login.search}`, {
    headers: { Cookie: login.cookiePair }
  });
  const oauthTokenListBody = parseJSON(oauthTokenList);
  const oauthTokenRow = (oauthTokenListBody.tokens ?? []).find((token) =>
    String(token.name ?? "").startsWith("OAuth ") && (token.scopes ?? []).includes("openid")
  );
  const oauthRevoke = await request("/oauth/revoke", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      token: oauthSecret,
      token_type_hint: "access_token"
    }).toString()
  });
  const oauthRevokeBody = parseJSON(oauthRevoke);
  const oauthAccessAfterRevoke = await mcpJSONRPC("tools/call", { name: "get_mcp_access", arguments: {} }, { id: 19, headers: { Authorization: `Bearer ${oauthSecret}` } });
  const oauthAccessAfterRevokeBody = parseJSON(oauthAccessAfterRevoke);
  const tokenListBefore = await request(`/api/game/mcp-tokens${login.search}`, {
    headers: { Cookie: login.cookiePair }
  });
  const tokenListBeforeBody = parseJSON(tokenListBefore);
  const currentRole = String(tokenListBeforeBody.role ?? "player");
  const currentUserType = Number(tokenListBeforeBody.userType ?? 0);
  const availableScopes = Array.isArray(tokenListBeforeBody.availableScopes) ? tokenListBeforeBody.availableScopes : [];
  const staffScope = currentRole === "admin" ? "mcp:admin" : currentRole === "operator" ? "mcp:operator" : "";
  const deniedStaffScope = currentRole === "operator" ? "mcp:admin" : currentRole === "player" ? "mcp:operator" : "";
  const deniedStaffCreate = deniedStaffScope === "" ? undefined : await request(`/api/game/mcp-tokens${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({
      name: `go-mcp-denied-${Date.now().toString(36)}`,
      scopes: [deniedStaffScope]
    })
  });
  const staffTokenCreate = staffScope === "" ? undefined : await request(`/api/game/mcp-tokens${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({
      name: `go-mcp-${currentRole}-${Date.now().toString(36)}`,
      scopes: [staffScope]
    })
  });
  const staffTokenCreateBody = staffTokenCreate === undefined ? {} : parseJSON(staffTokenCreate);
  const staffSecret = String(staffTokenCreateBody.secret ?? "");
  const staffTokenID = Number(staffTokenCreateBody.token?.id ?? 0);
  const staffHeaders = { Authorization: `Bearer ${staffSecret}` };
  const staffTools = staffSecret === "" ? undefined : await mcpJSONRPC("tools/list", {}, { id: 77, headers: staffHeaders });
  const staffToolsBody = staffTools === undefined ? {} : parseJSON(staffTools);
  const staffAccess = staffSecret === "" ? undefined : await mcpJSONRPC("tools/call", { name: "get_admin_access", arguments: {} }, { id: 78, headers: staffHeaders });
  const staffAccessBody = staffAccess === undefined ? {} : parseJSON(staffAccess);
  const staffPanel = staffSecret === "" ? undefined : await mcpJSONRPC("tools/call", { name: "get_admin_panel", arguments: { mode: "Home" } }, { id: 79, headers: staffHeaders });
  const staffPanelBody = staffPanel === undefined ? {} : parseJSON(staffPanel);
  const staffMutation = staffSecret === "" ? undefined : await mcpJSONRPC("tools/call", { name: "mutate_admin_panel", arguments: { mode: "Bans", action: "ban" } }, { id: 80, headers: staffHeaders });
  const staffMutationBody = staffMutation === undefined ? {} : parseJSON(staffMutation);
  const staffAdminOnlyPanel = staffSecret === "" ? undefined : await mcpJSONRPC("tools/call", { name: "get_admin_panel", arguments: { mode: "Bots" } }, { id: 81, headers: staffHeaders });
  const staffAdminOnlyPanelBody = staffAdminOnlyPanel === undefined ? {} : parseJSON(staffAdminOnlyPanel);
  const limitedTokenCreate = currentRole !== "admin" ? undefined : await request(`/api/game/mcp-tokens${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({
      name: `go-mcp-operator-ceiling-${Date.now().toString(36)}`,
      scopes: ["mcp:operator"]
    })
  });
  const limitedTokenCreateBody = limitedTokenCreate === undefined ? {} : parseJSON(limitedTokenCreate);
  const limitedSecret = String(limitedTokenCreateBody.secret ?? "");
  const limitedTokenID = Number(limitedTokenCreateBody.token?.id ?? 0);
  const limitedHeaders = { Authorization: `Bearer ${limitedSecret}` };
  const limitedAccess = limitedSecret === "" ? undefined : await mcpJSONRPC("tools/call", { name: "get_admin_access", arguments: {} }, { id: 82, headers: limitedHeaders });
  const limitedAccessBody = limitedAccess === undefined ? {} : parseJSON(limitedAccess);
  const limitedAdminOnlyPanel = limitedSecret === "" ? undefined : await mcpJSONRPC("tools/call", { name: "get_admin_panel", arguments: { mode: "Bots" } }, { id: 83, headers: limitedHeaders });
  const limitedAdminOnlyPanelBody = limitedAdminOnlyPanel === undefined ? {} : parseJSON(limitedAdminOnlyPanel);
  const tokenCreate = await request(`/api/game/mcp-tokens${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({
      name: `go-mcp-smoke-${Date.now().toString(36)}`,
      scopes: ["mcp:read", "mcp:messages", "mcp:message_write", "mcp:notes_write", "mcp:buddy_write", "mcp:fleet", "mcp:fleet_write", "mcp:queue_write", "mcp:resources_write", "mcp:premium_write", "mcp:merchant_write", "mcp:planet_write", "mcp:alliance_write", "mcp:account_write", "mcp:payment_write"],
      expiresInSeconds: 604800
    })
  });
  const tokenCreateBody = parseJSON(tokenCreate);
  const secret = String(tokenCreateBody.secret ?? "");
  const tokenID = Number(tokenCreateBody.token?.id ?? 0);
  const authHeaders = { Authorization: `Bearer ${secret}` };
  const tokenListAfterCreate = await request(`/api/game/mcp-tokens${login.search}`, {
    headers: { Cookie: login.cookiePair }
  });
  const tokenListAfterCreateBody = parseJSON(tokenListAfterCreate);
  const tokenRowAfterCreate = (tokenListAfterCreateBody.tokens ?? []).find((token) => Number(token.id ?? 0) === tokenID);
  const authedTools = await mcpJSONRPC("tools/list", {}, { id: 20, headers: authHeaders });
  const authedToolsBody = parseJSON(authedTools);
  const accessTool = await mcpJSONRPC("tools/call", { name: "get_mcp_access", arguments: {} }, { id: 21, headers: authHeaders });
  const accessToolBody = parseJSON(accessTool);
  const planetsTool = await mcpJSONRPC("tools/call", { name: "list_planets", arguments: {} }, { id: 22, headers: authHeaders });
  const planetsToolBody = parseJSON(planetsTool);
  const overviewTool = await mcpJSONRPC("tools/call", { name: "get_account_overview", arguments: {} }, { id: 23, headers: authHeaders });
  const overviewToolBody = parseJSON(overviewTool);
  const resourcesTool = await mcpJSONRPC("tools/call", { name: "get_planet_resources", arguments: {} }, { id: 24, headers: authHeaders });
  const resourcesToolBody = parseJSON(resourcesTool);
  const resourceProductionTool = await mcpJSONRPC("tools/call", { name: "get_resource_production_options", arguments: {} }, { id: 52, headers: authHeaders });
  const resourceProductionToolBody = parseJSON(resourceProductionTool);
  const queueTool = await mcpJSONRPC("tools/call", { name: "get_building_queue", arguments: {} }, { id: 25, headers: authHeaders });
  const queueToolBody = parseJSON(queueTool);
  const fleetTool = await mcpJSONRPC("tools/call", { name: "get_fleet_movements", arguments: {} }, { id: 26, headers: authHeaders });
  const fleetToolBody = parseJSON(fleetTool);
  const fleetOptionsTool = await mcpJSONRPC("tools/call", { name: "get_fleet_options", arguments: {} }, { id: 51, headers: authHeaders });
  const fleetOptionsToolBody = parseJSON(fleetOptionsTool);
  const officerStatusTool = await mcpJSONRPC("tools/call", { name: "get_officer_status", arguments: {} }, { id: 41, headers: authHeaders });
  const officerStatusToolBody = parseJSON(officerStatusTool);
  const searchGameTool = await mcpJSONRPC("tools/call", { name: "search_game", arguments: { type: "playername", text: loginUser.slice(0, 3) || "leg" } }, { id: 42, headers: authHeaders });
  const searchGameToolBody = parseJSON(searchGameTool);
  const galaxySystemTool = await mcpJSONRPC("tools/call", { name: "get_galaxy_system", arguments: {} }, { id: 43, headers: authHeaders });
  const galaxySystemToolBody = parseJSON(galaxySystemTool);
  const statisticsTool = await mcpJSONRPC("tools/call", { name: "get_statistics", arguments: { who: "player", type: "resources" } }, { id: 44, headers: authHeaders });
  const statisticsToolBody = parseJSON(statisticsTool);
  const allianceStatusTool = await mcpJSONRPC("tools/call", { name: "get_alliance_status", arguments: {} }, { id: 53, headers: authHeaders });
  const allianceStatusToolBody = parseJSON(allianceStatusTool);
  const buddyStatusTool = await mcpJSONRPC("tools/call", { name: "get_buddy_status", arguments: {} }, { id: 54, headers: authHeaders });
  const buddyStatusToolBody = parseJSON(buddyStatusTool);
  const buddyMutationDryRun = await mcpJSONRPC("tools/call", { name: "mutate_buddy", arguments: { action: "add", buddyId: login.playerID, text: "MCP smoke dry-run" } }, { id: 62, headers: authHeaders });
  const buddyMutationDryRunBody = parseJSON(buddyMutationDryRun);
  const prangerTool = await mcpJSONRPC("tools/call", { name: "get_pranger", arguments: { from: 0 } }, { id: 63, headers: authHeaders });
  const prangerToolBody = parseJSON(prangerTool);
  const notesTool = await mcpJSONRPC("tools/call", { name: "get_notes", arguments: {} }, { id: 55, headers: authHeaders });
  const notesToolBody = parseJSON(notesTool);
  const createNoteDryRun = await mcpJSONRPC("tools/call", { name: "create_note", arguments: { subject: "MCP smoke dry-run", text: "This note is not written.", priority: 1 } }, { id: 61, headers: authHeaders });
  const createNoteDryRunBody = parseJSON(createNoteDryRun);
  const optionsTool = await mcpJSONRPC("tools/call", { name: "get_options", arguments: {} }, { id: 56, headers: authHeaders });
  const optionsToolBody = parseJSON(optionsTool);
  const maintenanceTool = await mcpJSONRPC("tools/call", { name: "get_maintenance", arguments: {} }, { id: 66, headers: authHeaders });
  const maintenanceToolBody = parseJSON(maintenanceTool);
  const merchantStatusTool = await mcpJSONRPC("tools/call", { name: "get_merchant_status", arguments: {} }, { id: 57, headers: authHeaders });
  const merchantStatusToolBody = parseJSON(merchantStatusTool);
  const merchantMutationDryRun = await mcpJSONRPC("tools/call", { name: "mutate_merchant", arguments: { action: "call", offerId: 1 } }, { id: 64, headers: authHeaders });
  const merchantMutationDryRunBody = parseJSON(merchantMutationDryRun);
  const jumpGateStatusTool = await mcpJSONRPC("tools/call", { name: "get_jump_gate_status", arguments: {} }, { id: 58, headers: authHeaders });
  const jumpGateStatusToolBody = parseJSON(jumpGateStatusTool);
  const empireOverviewTool = await mcpJSONRPC("tools/call", { name: "get_empire_overview", arguments: { planetType: 1 } }, { id: 45, headers: authHeaders });
  const empireOverviewToolBody = parseJSON(empireOverviewTool);
  const technologyTreeTool = await mcpJSONRPC("tools/call", { name: "get_technology_tree", arguments: { detailsId: 204, infoId: 1 } }, { id: 46, headers: authHeaders });
  const technologyTreeToolBody = parseJSON(technologyTreeTool);
  const buildingOptionsTool = await mcpJSONRPC("tools/call", { name: "get_building_options", arguments: {} }, { id: 47, headers: authHeaders });
  const buildingOptionsToolBody = parseJSON(buildingOptionsTool);
  const researchOptionsTool = await mcpJSONRPC("tools/call", { name: "get_research_options", arguments: {} }, { id: 48, headers: authHeaders });
  const researchOptionsToolBody = parseJSON(researchOptionsTool);
  const shipyardOptionsTool = await mcpJSONRPC("tools/call", { name: "get_shipyard_options", arguments: {} }, { id: 49, headers: authHeaders });
  const shipyardOptionsToolBody = parseJSON(shipyardOptionsTool);
  const defenseOptionsTool = await mcpJSONRPC("tools/call", { name: "get_defense_options", arguments: {} }, { id: 50, headers: authHeaders });
  const defenseOptionsToolBody = parseJSON(defenseOptionsTool);
  const messagesTool = await mcpJSONRPC("tools/call", { name: "list_messages", arguments: { limit: 5 } }, { id: 29, headers: authHeaders });
  const messagesToolBody = parseJSON(messagesTool);
  const firstSpyReportID = Number((messagesToolBody.result?.structuredContent?.messages?.messages ?? []).find((message) => Number(message?.type ?? 0) === 1)?.id ?? 0);
  const reportIDForRead = firstSpyReportID > 0 ? firstSpyReportID : 999999999;
  const reportTool = await mcpJSONRPC("tools/call", { name: "get_report", arguments: { reportId: reportIDForRead } }, { id: 59, headers: authHeaders });
  const reportToolBody = parseJSON(reportTool);
  const sendMessageDryRun = await mcpJSONRPC("tools/call", { name: "send_message", arguments: { targetPlayerId: login.playerID, subject: "MCP smoke dry-run", text: "This is a dry-run from MCP smoke." } }, { id: 30, headers: authHeaders });
  const sendMessageDryRunBody = parseJSON(sendMessageDryRun);
  const firstMessageID = Number(messagesToolBody.result?.structuredContent?.messages?.messages?.[0]?.id ?? 0);
  const deleteMessageDryRun = firstMessageID > 0
    ? await mcpJSONRPC("tools/call", { name: "delete_messages", arguments: { messageIds: [firstMessageID] } }, { id: 31, headers: authHeaders })
    : undefined;
  const deleteMessageDryRunBody = deleteMessageDryRun === undefined ? {} : parseJSON(deleteMessageDryRun);
  const firstReportableMessageID = Number((messagesToolBody.result?.structuredContent?.messages?.messages ?? []).find((message) => message?.reportable === true)?.id ?? 0);
  const reportMessageDryRun = firstReportableMessageID > 0
    ? await mcpJSONRPC("tools/call", { name: "report_message", arguments: { messageId: firstReportableMessageID } }, { id: 32, headers: authHeaders })
    : undefined;
  const reportMessageDryRunBody = reportMessageDryRun === undefined ? {} : parseJSON(reportMessageDryRun);
  const validateFleetDispatchDryRun = await mcpJSONRPC("tools/call", { name: "validate_fleet_dispatch", arguments: { ships: { "202": 1 }, targetGalaxy: 9, targetSystem: 499, targetPosition: 16, targetType: 1, mission: 15, speed: 10, expeditionHours: 1 } }, { id: 34, headers: authHeaders });
  const validateFleetDispatchDryRunBody = parseJSON(validateFleetDispatchDryRun);
  const dispatchFleetWrongConfirm = await mcpJSONRPC("tools/call", { name: "dispatch_fleet", arguments: { ships: { "202": 1 }, targetGalaxy: 9, targetSystem: 499, targetPosition: 15, targetType: 1, mission: 3, speed: 10, confirm: "wrong" } }, { id: 35, headers: authHeaders });
  const dispatchFleetWrongConfirmBody = parseJSON(dispatchFleetWrongConfirm);
  const recallFleetDryRun = await mcpJSONRPC("tools/call", { name: "recall_fleet", arguments: { fleetId: 999999999 } }, { id: 33, headers: authHeaders });
  const recallFleetDryRunBody = parseJSON(recallFleetDryRun);
  const phalanxScanDryRun = await mcpJSONRPC("tools/call", { name: "scan_phalanx", arguments: { targetPlanetId: 999999999 } }, { id: 60, headers: authHeaders });
  const phalanxScanDryRunBody = parseJSON(phalanxScanDryRun);
  const jumpGateDryRun = await mcpJSONRPC("tools/call", { name: "jump_gate", arguments: { targetMoonId: 999999999, ships: { "202": 1 } } }, { id: 65, headers: authHeaders });
  const jumpGateDryRunBody = parseJSON(jumpGateDryRun);
  const cancelBuildingQueueDryRun = await mcpJSONRPC("tools/call", { name: "cancel_building_queue", arguments: { listId: 999999999 } }, { id: 36, headers: authHeaders });
  const cancelBuildingQueueDryRunBody = parseJSON(cancelBuildingQueueDryRun);
  const cancelResearchQueueDryRun = await mcpJSONRPC("tools/call", { name: "cancel_research_queue", arguments: {} }, { id: 37, headers: authHeaders });
  const cancelResearchQueueDryRunBody = parseJSON(cancelResearchQueueDryRun);
  const enqueueShipyardOrderDryRun = await mcpJSONRPC("tools/call", { name: "enqueue_shipyard_order", arguments: { kind: "fleet", itemId: 204, amount: 1 } }, { id: 38, headers: authHeaders });
  const enqueueShipyardOrderDryRunBody = parseJSON(enqueueShipyardOrderDryRun);
  const updateResourceProductionDryRun = await mcpJSONRPC("tools/call", { name: "update_resource_production", arguments: { production: { "1": 80 } } }, { id: 39, headers: authHeaders });
  const updateResourceProductionDryRunBody = parseJSON(updateResourceProductionDryRun);
  const recruitOfficerDryRun = await mcpJSONRPC("tools/call", { name: "recruit_officer", arguments: { officerId: 1, days: 7 } }, { id: 40, headers: authHeaders });
  const recruitOfficerDryRunBody = parseJSON(recruitOfficerDryRun);
  const mutateBuildingDryRun = await mcpJSONRPC("tools/call", { name: "mutate_building", arguments: { action: "add", techId: 1 } }, { id: 67, headers: authHeaders });
  const mutateBuildingDryRunBody = parseJSON(mutateBuildingDryRun);
  const startResearchDryRun = await mcpJSONRPC("tools/call", { name: "start_research", arguments: { techId: 106 } }, { id: 68, headers: authHeaders });
  const startResearchDryRunBody = parseJSON(startResearchDryRun);
  const mutatePlanetDryRun = await mcpJSONRPC("tools/call", { name: "mutate_planet", arguments: { action: "rename", planetId: Number(planetsToolBody.result?.structuredContent?.planets?.[0]?.id ?? 0), name: "MCP dry-run" } }, { id: 69, headers: authHeaders });
  const mutatePlanetDryRunBody = parseJSON(mutatePlanetDryRun);
  const mutateFleetTemplateDryRun = await mcpJSONRPC("tools/call", { name: "mutate_fleet_template", arguments: { action: "save", name: "MCP dry-run", ships: { "202": 1 } } }, { id: 70, headers: authHeaders });
  const mutateFleetTemplateDryRunBody = parseJSON(mutateFleetTemplateDryRun);
  const mutateCommanderQueueDryRun = await mcpJSONRPC("tools/call", { name: "mutate_commander_queue", arguments: { action: "add", targetPlanetId: Number(planetsToolBody.result?.structuredContent?.planets?.[0]?.id ?? 0), techId: 1 } }, { id: 71, headers: authHeaders });
  const mutateCommanderQueueDryRunBody = parseJSON(mutateCommanderQueueDryRun);
  const missileLaunchDryRun = await mcpJSONRPC("tools/call", { name: "launch_interplanetary_missiles", arguments: { targetPlanetId: 999999999, amount: 1, targetDefenseId: 401 } }, { id: 72, headers: authHeaders });
  const missileLaunchDryRunBody = parseJSON(missileLaunchDryRun);
  const galaxyDispatchDryRun = await mcpJSONRPC("tools/call", { name: "dispatch_galaxy_action", arguments: { action: "spy", targetGalaxy: 9, targetSystem: 499, targetPosition: 15, amount: 1 } }, { id: 73, headers: authHeaders });
  const galaxyDispatchDryRunBody = parseJSON(galaxyDispatchDryRun);
  const allianceMutationDryRun = await mcpJSONRPC("tools/call", { name: "mutate_alliance", arguments: { action: "leave" } }, { id: 74, headers: authHeaders });
  const allianceMutationDryRunBody = parseJSON(allianceMutationDryRun);
  const accountOptionsDryRun = await mcpJSONRPC("tools/call", { name: "update_account_options", arguments: { action: "settings" } }, { id: 75, headers: authHeaders });
  const accountOptionsDryRunBody = parseJSON(accountOptionsDryRun);
  const couponDryRun = await mcpJSONRPC("tools/call", { name: "redeem_coupon", arguments: { couponCode: "MCP-SMOKE-MISSING" } }, { id: 76, headers: authHeaders });
  const couponDryRunBody = parseJSON(couponDryRun);
  const invalidParamsTool = await mcpJSONRPC("tools/call", { name: "get_planet_resources", arguments: { planetId: "abc" } }, { id: 27, headers: authHeaders });
  const invalidParamsToolBody = parseJSON(invalidParamsTool);
  const tokenListAfterUse = await request(`/api/game/mcp-tokens${login.search}`, {
    headers: { Cookie: login.cookiePair }
  });
  const tokenListAfterUseBody = parseJSON(tokenListAfterUse);
  const tokenRowAfterUse = (tokenListAfterUseBody.tokens ?? []).find((token) => Number(token.id ?? 0) === tokenID);
  const noExpiryCreate = await request(`/api/game/mcp-tokens${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({ name: `go-mcp-never-${Date.now().toString(36)}`, scopes: ["mcp:read"], expiresInSeconds: 0 })
  });
  const noExpiryCreateBody = parseJSON(noExpiryCreate);
  const noExpiryTokenID = Number(noExpiryCreateBody.token?.id ?? 0);
  const tokenListBeforeLimit = await request(`/api/game/mcp-tokens${login.search}`, {
    headers: { Cookie: login.cookiePair }
  });
  const tokenListBeforeLimitBody = parseJSON(tokenListBeforeLimit);
  const noExpiryTokenRow = (tokenListBeforeLimitBody.tokens ?? []).find((token) => Number(token.id ?? 0) === noExpiryTokenID);
  const maxActiveTokens = Number(tokenListBeforeLimitBody.maxActiveTokens ?? 0);
  const fillerTokenIDs = [];
  for (let index = (tokenListBeforeLimitBody.tokens ?? []).length; index < maxActiveTokens; index += 1) {
    const filler = await request(`/api/game/mcp-tokens${login.search}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
      body: JSON.stringify({ name: `go-mcp-limit-${index}`, scopes: ["mcp:read"], expiresInSeconds: 3600 })
    });
    const fillerBody = parseJSON(filler);
    fillerTokenIDs.push(Number(fillerBody.token?.id ?? 0));
  }
  const tokenCreateOverLimit = await request(`/api/game/mcp-tokens${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({ name: "go-mcp-sixth", scopes: ["mcp:read"], expiresInSeconds: 3600 })
  });
  for (const cleanupTokenID of [noExpiryTokenID, ...fillerTokenIDs]) {
    if (cleanupTokenID > 0) {
      await request(`/api/game/mcp-tokens/revoke${login.search}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
        body: JSON.stringify({ tokenId: cleanupTokenID })
      });
    }
  }
  const revoke = await request(`/api/game/mcp-tokens/revoke${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({ tokenId: tokenID })
  });
  const revokeBody = parseJSON(revoke);
  const tokenListAfterRevoke = await request(`/api/game/mcp-tokens${login.search}`, {
    headers: { Cookie: login.cookiePair }
  });
  const tokenListAfterRevokeBody = parseJSON(tokenListAfterRevoke);
  const accessAfterRevoke = await mcpJSONRPC("tools/call", { name: "get_mcp_access", arguments: {} }, { id: 28, headers: authHeaders });
  const accessAfterRevokeBody = parseJSON(accessAfterRevoke);
  const staffRevoke = staffTokenID === 0 ? undefined : await request(`/api/game/mcp-tokens/revoke${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({ tokenId: staffTokenID })
  });
  const staffRevokeBody = staffRevoke === undefined ? {} : parseJSON(staffRevoke);
  const limitedRevoke = limitedTokenID === 0 ? undefined : await request(`/api/game/mcp-tokens/revoke${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({ tokenId: limitedTokenID })
  });
  const limitedRevokeBody = limitedRevoke === undefined ? {} : parseJSON(limitedRevoke);
  const documentedTools = mcpGuideToolGroups.flatMap((group) => group.tools);
  const staffToolsExpected = ["get_admin_access", "get_admin_panel", "mutate_admin_panel"];
  const expectedTools = documentedTools.filter((name) => !staffToolsExpected.includes(name));
  const authedToolNames = toolNames(authedToolsBody);
  const staffToolNames = toolNames(staffToolsBody);
  const roleLevelMatches = currentRole === "admin" ? currentUserType >= 2 : currentRole === "operator" ? currentUserType === 1 : currentUserType === 0;
  const scopeInventoryMatches = availableScopes.includes("mcp:read") && (
    currentRole === "admin"
      ? availableScopes.includes("mcp:operator") && availableScopes.includes("mcp:admin")
      : currentRole === "operator"
        ? availableScopes.includes("mcp:operator") && !availableScopes.includes("mcp:admin")
        : !availableScopes.includes("mcp:operator") && !availableScopes.includes("mcp:admin")
  );
  cases.push(finalize({
    case: "go_mcp_role_scope_flow",
    checks: [
      check(["player", "operator", "admin"].includes(currentRole) && roleLevelMatches, "MCP token management exposes the current database role", {
        role: currentRole,
        userType: currentUserType
      }),
      check(scopeInventoryMatches, "MCP token management exposes only scopes allowed by the current role", { role: currentRole, availableScopes }),
      check(deniedStaffScope === "" || deniedStaffCreate?.status === 400, "Player and Operator accounts cannot mint a token above their role", {
        role: currentRole,
        deniedScope: deniedStaffScope,
        status: deniedStaffCreate?.status
      }),
      check(staffScope === "" || (staffTokenCreate?.status === 200 && staffSecret.startsWith("ogmcp_") && staffTokenID > 0), "Operator and Admin accounts can mint their role-scoped staff token", {
        role: currentRole,
        staffScope,
        status: staffTokenCreate?.status,
        tokenID: staffTokenID
      }),
      check(staffScope === "" || (staffTools?.status === 200 && staffToolsExpected.every((name) => staffToolNames.includes(name))), "Staff token exposes the role-aware administration tools", { role: currentRole, staffToolNames }),
      check(staffScope === "" || (staffAccess?.status === 200 && staffAccessBody.result?.structuredContent?.adminAccess?.role === currentRole && staffAccessBody.result?.structuredContent?.adminAccess?.scopeRole === currentRole), "get_admin_access reports both account role and token scope ceiling", staffAccessBody.result ?? {}),
      check(staffScope === "" || (staffPanel?.status === 200 && staffPanelBody.result?.structuredContent?.adminPanel?.mode === "Home" && staffPanelBody.result?.structuredContent?.adminPanel?.scopeRole === currentRole), "get_admin_panel reads a staff-accessible panel under the current role", staffPanelBody.result ?? {}),
      check(staffScope === "" || (staffMutation?.status === 200 && staffMutationBody.result?.structuredContent?.adminMutation?.dryRun === true && staffMutationBody.result?.structuredContent?.adminMutation?.executed === false && staffMutationBody.result?.structuredContent?.adminMutation?.requiresConfirmation === true), "mutate_admin_panel defaults to a non-mutating confirmation preview", staffMutationBody.result ?? {}),
      check(staffScope === "" || (currentRole === "admin"
        ? staffAdminOnlyPanel?.status === 200 && staffAdminOnlyPanelBody.result?.structuredContent?.adminPanel?.mode === "Bots"
        : staffAdminOnlyPanel?.status === 403 && staffAdminOnlyPanelBody.error?.code === -32003), "Admin-only panels follow the effective staff role", staffAdminOnlyPanelBody),
      check(currentRole !== "admin" || (limitedTokenCreate?.status === 200 && limitedSecret.startsWith("ogmcp_") && limitedTokenID > 0), "Admin can deliberately mint an Operator-limited token", {
        status: limitedTokenCreate?.status,
        tokenID: limitedTokenID
      }),
      check(currentRole !== "admin" || (limitedAccess?.status === 200 && limitedAccessBody.result?.structuredContent?.adminAccess?.role === "admin" && limitedAccessBody.result?.structuredContent?.adminAccess?.scopeRole === "operator"), "Operator scope limits an Admin account to Operator MCP authority", limitedAccessBody.result ?? {}),
      check(currentRole !== "admin" || (limitedAdminOnlyPanel?.status === 403 && limitedAdminOnlyPanelBody.error?.code === -32003), "Operator-limited Admin token cannot read Admin-only panels", limitedAdminOnlyPanelBody),
      check(staffScope === "" || (staffRevoke?.status === 200 && staffRevokeBody.revoked === true), "Staff MCP token is revoked after the role smoke flow", staffRevokeBody),
      check(currentRole !== "admin" || (limitedRevoke?.status === 200 && limitedRevokeBody.revoked === true), "Operator-limited MCP token is revoked after the scope-ceiling flow", limitedRevokeBody),
      check(!authedToolNames.some((name) => staffToolsExpected.includes(name)), "A token without staff scope never exposes administration tools", { authedToolNames })
    ]
  }));
  cases.push(finalize({
    case: "go_mcp_user_token_flow",
    checks: [
      check(login.response.status === 200 && login.playerID > 0 && login.cookiePair !== "", "smoke user can log in for MCP token management", {
        status: login.response.status,
        playerID: login.playerID
      }),
      check(sessionID !== "", "smoke login exposes a public session for OAuth consent", { sessionID }),
      check(clientRegistration.status === 201 && oauthClientID.startsWith("ogmcp_client_"), "OAuth dynamic client registration returns public client id", clientRegistrationBody),
      check(oauthConsent.status === 200 && oauthConsent.body.includes("Authorize MCP access") && hiddenInputValue(oauthConsent.body, "consent_token") !== "", "OAuth authorize shows consent page before approval", { status: oauthConsent.status }),
      check(oauthApprove.status === 302 && oauthApproveLocation.startsWith(oauthRedirectURI) && oauthCallback.searchParams.get("state") === "go-mcp-smoke-state" && oauthCode !== "", "OAuth authorize approval redirects with code and state", { status: oauthApprove.status, location: oauthApproveLocation }),
      check(oauthToken.status === 200 && oauthSecret.startsWith("ogmcp_") && oauthTokenBody.token_type === "Bearer" && Number(oauthTokenBody.expires_in ?? 0) > 0, "OAuth token exchange returns bearer access token", oauthTokenBody),
      check(oauthIDToken.split(".").length === 3 && oauthIDClaims.iss === baseUrl && oauthIDClaims.aud === oauthClientID && oauthIDClaims.sub === `player:${login.playerID}`, "OAuth openid exchange returns ID token claims", oauthIDClaims),
      check(oauthTools.status === 200 && toolNames(oauthToolsBody).length === expectedTools.length && expectedTools.every((name) => toolNames(oauthToolsBody).includes(name)), "OAuth bearer token exposes exactly the documented player MCP tools", { oauthToolNames: toolNames(oauthToolsBody) }),
      check(Number(oauthTokenRow?.id ?? 0) > 0 && Number(oauthTokenRow?.expiresAt ?? 0) > Number(oauthTokenRow?.createdAt ?? 0), "OAuth exchange persists a revocable expiring MCP token row", { oauthTokenRow }),
      check(oauthRevoke.status === 200 && oauthRevokeBody.revoked === true, "OAuth revocation endpoint revokes created MCP token", oauthRevokeBody),
      check(oauthAccessAfterRevoke.status === 401 && oauthAccessAfterRevokeBody.error?.code === -32001, "OAuth-revoked bearer token is rejected by MCP", oauthAccessAfterRevokeBody),
      check(tokenListBefore.status === 200 && tokenListBeforeBody.authenticated === true && Array.isArray(tokenListBeforeBody.tokens), "MCP token list authenticates game session", tokenListBeforeBody),
      check(tokenListBeforeBody.maxActiveTokens === 5 && (tokenListBeforeBody.expiryOptions ?? []).some((option) => Number(option.seconds) === 0) && (tokenListBeforeBody.expiryOptions ?? []).some((option) => Number(option.seconds) === 604800), "MCP token list exposes active-token and expiration policies", tokenListBeforeBody),
      check(tokenCreate.status === 200 && tokenCreateBody.authenticated === true && tokenID > 0 && Number(tokenCreateBody.token?.expiresAt ?? 0) - Number(tokenCreateBody.token?.createdAt ?? 0) === 604800, "MCP token create honors the selected expiration period", tokenCreateBody.token ?? {}),
      check(secret.startsWith("ogmcp_") && secret.length > 12, "MCP token create returns a one-time bearer secret", {
        secretPrefix: secret.slice(0, 6),
        secretLength: secret.length
      }),
      check(tokenRowAfterCreate !== undefined && Number(tokenRowAfterCreate?.expiresAt ?? 0) - Number(tokenRowAfterCreate?.createdAt ?? 0) === 604800, "MCP token list includes the selected expiration", { tokenID, tokenRowAfterCreate }),
      check(noExpiryCreate.status === 200 && noExpiryTokenID > 0 && Number(noExpiryCreateBody.token?.expiresAt ?? 0) === 0 && Number(noExpiryTokenRow?.expiresAt ?? 0) === 0, "MCP token create and list support no expiration", { noExpiryCreateBody, noExpiryTokenRow }),
      check(maxActiveTokens === 5 && fillerTokenIDs.every((id) => id > 0) && tokenCreateOverLimit.status === 409 && tokenCreateOverLimit.body.includes("maximum of 5"), "MCP token creation rejects a sixth active token", { maxActiveTokens, fillerTokenIDs, status: tokenCreateOverLimit.status, body: tokenCreateOverLimit.body }),
      check(!String(tokenListAfterCreate.body ?? "").includes(secret), "MCP token list never exposes the plaintext secret"),
      check(authedTools.status === 200 && authedToolNames.length === expectedTools.length && expectedTools.every((name) => authedToolNames.includes(name)), "bearer token exposes exactly the documented player MCP tools", {
        authedToolNames
      }),
      check(accessTool.status === 200 && Number(accessToolBody.result?.structuredContent?.playerId ?? 0) === login.playerID, "get_mcp_access returns bearer player id", accessToolBody.result ?? {}),
      check((accessToolBody.result?.structuredContent?.scopes ?? []).includes("mcp:read"), "get_mcp_access returns bearer scopes", accessToolBody.result?.structuredContent ?? {}),
      check(planetsTool.status === 200 && (planetsToolBody.result?.structuredContent?.planets ?? []).length > 0, "list_planets returns at least one planet", planetsToolBody.result ?? {}),
      check(overviewTool.status === 200 && Number(overviewToolBody.result?.structuredContent?.overview?.playerId ?? 0) === login.playerID, "get_account_overview returns current player data", overviewToolBody.result ?? {}),
      check(resourcesTool.status === 200 && Number(resourcesToolBody.result?.structuredContent?.resources?.playerId ?? 0) === login.playerID, "get_planet_resources returns current player data", resourcesToolBody.result ?? {}),
      check(resourceProductionTool.status === 200 && Number(resourceProductionToolBody.result?.structuredContent?.resourceProductionOptions?.playerId ?? 0) === login.playerID && Array.isArray(resourceProductionToolBody.result?.structuredContent?.resourceProductionOptions?.rows), "get_resource_production_options returns read-only legacy production settings", resourceProductionToolBody.result ?? {}),
      check(queueTool.status === 200 && Number(queueToolBody.result?.structuredContent?.buildingQueue?.playerId ?? 0) === login.playerID, "get_building_queue returns current player data", queueToolBody.result ?? {}),
      check(fleetTool.status === 200 && Number(fleetToolBody.result?.structuredContent?.fleetMovements?.playerId ?? 0) === login.playerID, "get_fleet_movements returns current player data", fleetToolBody.result ?? {}),
      check(fleetOptionsTool.status === 200 && Number(fleetOptionsToolBody.result?.structuredContent?.fleetOptions?.playerId ?? 0) === login.playerID && Array.isArray(fleetOptionsToolBody.result?.structuredContent?.fleetOptions?.ships), "get_fleet_options returns read-only legacy fleet screen state", fleetOptionsToolBody.result ?? {}),
      check(officerStatusTool.status === 200 && Number(officerStatusToolBody.result?.structuredContent?.officerStatus?.playerId ?? 0) === login.playerID && Array.isArray(officerStatusToolBody.result?.structuredContent?.officerStatus?.officers), "get_officer_status returns current officer rows and Dark Matter balances", officerStatusToolBody.result ?? {}),
      check(searchGameTool.status === 200 && Number(searchGameToolBody.result?.structuredContent?.search?.playerId ?? 0) === login.playerID && Array.isArray(searchGameToolBody.result?.structuredContent?.search?.players), "search_game returns current player search results", searchGameToolBody.result ?? {}),
      check(galaxySystemTool.status === 200 && Number(galaxySystemToolBody.result?.structuredContent?.galaxySystem?.playerId ?? 0) === login.playerID && Array.isArray(galaxySystemToolBody.result?.structuredContent?.galaxySystem?.rows), "get_galaxy_system returns current galaxy rows without mutation", galaxySystemToolBody.result ?? {}),
      check(statisticsTool.status === 200 && Number(statisticsToolBody.result?.structuredContent?.statistics?.playerId ?? 0) === login.playerID && Array.isArray(statisticsToolBody.result?.structuredContent?.statistics?.rows), "get_statistics returns current legacy ranking rows", statisticsToolBody.result ?? {}),
      check(allianceStatusTool.status === 200 && Number(allianceStatusToolBody.result?.structuredContent?.allianceStatus?.playerId ?? 0) === login.playerID && typeof allianceStatusToolBody.result?.structuredContent?.allianceStatus?.view === "string", "get_alliance_status returns read-only legacy alliance state", allianceStatusToolBody.result ?? {}),
      check(buddyStatusTool.status === 200 && Number(buddyStatusToolBody.result?.structuredContent?.buddyStatus?.playerId ?? 0) === login.playerID && Array.isArray(buddyStatusToolBody.result?.structuredContent?.buddyStatus?.rows), "get_buddy_status returns read-only legacy buddy state", buddyStatusToolBody.result ?? {}),
      check(buddyMutationDryRun.status === 200 && Number(buddyMutationDryRunBody.result?.structuredContent?.buddyMutation?.playerId ?? 0) === login.playerID && buddyMutationDryRunBody.result?.structuredContent?.buddyMutation?.dryRun === true && buddyMutationDryRunBody.result?.structuredContent?.buddyMutation?.executed === false && buddyMutationDryRunBody.result?.structuredContent?.buddyMutation?.requiresConfirmation === true && String(buddyMutationDryRunBody.result?.structuredContent?.buddyMutation?.confirmation ?? "").startsWith("mutate_buddy:"), "mutate_buddy dry-run is available under buddy_write and does not mutate before confirmation", buddyMutationDryRunBody.result ?? {}),
      check(prangerTool.status === 200 && Number(prangerToolBody.result?.structuredContent?.pranger?.playerId ?? 0) === login.playerID && Array.isArray(prangerToolBody.result?.structuredContent?.pranger?.entries), "get_pranger returns read-only legacy pranger rows", prangerToolBody.result ?? {}),
      check(notesTool.status === 200 && Number(notesToolBody.result?.structuredContent?.notes?.playerId ?? 0) === login.playerID && Array.isArray(notesToolBody.result?.structuredContent?.notes?.rows), "get_notes returns read-only legacy notes state", notesToolBody.result ?? {}),
      check(createNoteDryRun.status === 200 && Number(createNoteDryRunBody.result?.structuredContent?.createNote?.playerId ?? 0) === login.playerID && createNoteDryRunBody.result?.structuredContent?.createNote?.dryRun === true && createNoteDryRunBody.result?.structuredContent?.createNote?.executed === false && createNoteDryRunBody.result?.structuredContent?.createNote?.requiresConfirmation === true && String(createNoteDryRunBody.result?.structuredContent?.createNote?.confirmation ?? "").startsWith("create_note:"), "create_note dry-run is available under notes_write and does not mutate before confirmation", createNoteDryRunBody.result ?? {}),
      check(optionsTool.status === 200 && Number(optionsToolBody.result?.structuredContent?.options?.playerId ?? 0) === login.playerID && String(optionsToolBody.result?.structuredContent?.options?.user?.name ?? "") !== "", "get_options returns read-only legacy options state without secrets", optionsToolBody.result ?? {}),
      check(maintenanceTool.status === 200 && Number(maintenanceToolBody.result?.structuredContent?.maintenance?.playerId ?? 0) === login.playerID && typeof maintenanceToolBody.result?.structuredContent?.maintenance?.frozen === "boolean", "get_maintenance returns read-only universe maintenance state", maintenanceToolBody.result ?? {}),
      check(merchantStatusTool.status === 200 && Number(merchantStatusToolBody.result?.structuredContent?.merchantStatus?.playerId ?? 0) === login.playerID && Array.isArray(merchantStatusToolBody.result?.structuredContent?.merchantStatus?.rows), "get_merchant_status returns read-only legacy merchant state", merchantStatusToolBody.result ?? {}),
      check(merchantMutationDryRun.status === 200 && Number(merchantMutationDryRunBody.result?.structuredContent?.merchantMutation?.playerId ?? 0) === login.playerID && merchantMutationDryRunBody.result?.structuredContent?.merchantMutation?.dryRun === true && merchantMutationDryRunBody.result?.structuredContent?.merchantMutation?.executed === false && (String(merchantMutationDryRunBody.result?.structuredContent?.merchantMutation?.confirmation ?? "").startsWith("mutate_merchant:") || typeof merchantMutationDryRunBody.result?.structuredContent?.merchantMutation?.issue?.code === "string"), "mutate_merchant dry-run is available under merchant_write and does not mutate before confirmation", merchantMutationDryRunBody.result ?? {}),
      check(jumpGateStatusTool.status === 200 && Number(jumpGateStatusToolBody.result?.structuredContent?.jumpGateStatus?.playerId ?? 0) === login.playerID && Array.isArray(jumpGateStatusToolBody.result?.structuredContent?.jumpGateStatus?.targets), "get_jump_gate_status returns read-only legacy jump gate state", jumpGateStatusToolBody.result ?? {}),
      check(empireOverviewTool.status === 200 && Number(empireOverviewToolBody.result?.structuredContent?.empire?.playerId ?? 0) === login.playerID && Array.isArray(empireOverviewToolBody.result?.structuredContent?.empire?.planets), "get_empire_overview returns read-only empire aggregate rows", empireOverviewToolBody.result ?? {}),
      check(technologyTreeTool.status === 200 && Number(technologyTreeToolBody.result?.structuredContent?.technology?.playerId ?? 0) === login.playerID && Array.isArray(technologyTreeToolBody.result?.structuredContent?.technology?.groups), "get_technology_tree returns legacy requirements and info rows", technologyTreeToolBody.result ?? {}),
      check(buildingOptionsTool.status === 200 && Number(buildingOptionsToolBody.result?.structuredContent?.buildingOptions?.playerId ?? 0) === login.playerID && Array.isArray(buildingOptionsToolBody.result?.structuredContent?.buildingOptions?.items), "get_building_options returns read-only legacy building options", buildingOptionsToolBody.result ?? {}),
      check(researchOptionsTool.status === 200 && Number(researchOptionsToolBody.result?.structuredContent?.researchOptions?.playerId ?? 0) === login.playerID && Array.isArray(researchOptionsToolBody.result?.structuredContent?.researchOptions?.items), "get_research_options returns read-only legacy research options", researchOptionsToolBody.result ?? {}),
      check(shipyardOptionsTool.status === 200 && Number(shipyardOptionsToolBody.result?.structuredContent?.shipyardOptions?.playerId ?? 0) === login.playerID && Array.isArray(shipyardOptionsToolBody.result?.structuredContent?.shipyardOptions?.items), "get_shipyard_options returns read-only legacy shipyard options", shipyardOptionsToolBody.result ?? {}),
      check(defenseOptionsTool.status === 200 && Number(defenseOptionsToolBody.result?.structuredContent?.defenseOptions?.playerId ?? 0) === login.playerID && Array.isArray(defenseOptionsToolBody.result?.structuredContent?.defenseOptions?.items), "get_defense_options returns read-only legacy defense options", defenseOptionsToolBody.result ?? {}),
      check(messagesTool.status === 200 && Number(messagesToolBody.result?.structuredContent?.messages?.playerId ?? 0) === login.playerID && Array.isArray(messagesToolBody.result?.structuredContent?.messages?.messages), "list_messages returns current player message rows without mutation", messagesToolBody.result ?? {}),
      check(reportTool.status === 200 && Number(reportToolBody.result?.structuredContent?.report?.playerId ?? 0) === login.playerID && Number(reportToolBody.result?.structuredContent?.report?.id ?? 0) === reportIDForRead && typeof reportToolBody.result?.structuredContent?.report?.allowed === "boolean", "get_report returns legacy report access result without mutation", reportToolBody.result ?? {}),
      check(sendMessageDryRun.status === 200 && Number(sendMessageDryRunBody.result?.structuredContent?.sendMessage?.playerId ?? 0) === login.playerID && sendMessageDryRunBody.result?.structuredContent?.sendMessage?.dryRun === true && sendMessageDryRunBody.result?.structuredContent?.sendMessage?.requiresConfirmation === true && String(sendMessageDryRunBody.result?.structuredContent?.sendMessage?.confirmation ?? "").startsWith(`send_message:${login.playerID}:`), "send_message dry-run returns explicit confirmation without executing", sendMessageDryRunBody.result ?? {}),
      check(firstMessageID === 0 || (deleteMessageDryRun?.status === 200 && Number(deleteMessageDryRunBody.result?.structuredContent?.deleteMessages?.playerId ?? 0) === login.playerID && deleteMessageDryRunBody.result?.structuredContent?.deleteMessages?.dryRun === true && deleteMessageDryRunBody.result?.structuredContent?.deleteMessages?.requiresConfirmation === true && deleteMessageDryRunBody.result?.structuredContent?.deleteMessages?.executed === false && String(deleteMessageDryRunBody.result?.structuredContent?.deleteMessages?.confirmation ?? "").startsWith(`delete_messages:${firstMessageID}:`)), "delete_messages dry-run returns explicit confirmation for an owned visible message when available", {
        firstMessageID,
        result: deleteMessageDryRunBody.result ?? {}
      }),
      check(firstReportableMessageID === 0 || (reportMessageDryRun?.status === 200 && Number(reportMessageDryRunBody.result?.structuredContent?.reportMessage?.playerId ?? 0) === login.playerID && reportMessageDryRunBody.result?.structuredContent?.reportMessage?.dryRun === true && reportMessageDryRunBody.result?.structuredContent?.reportMessage?.reportable === true && reportMessageDryRunBody.result?.structuredContent?.reportMessage?.requiresConfirmation === true && reportMessageDryRunBody.result?.structuredContent?.reportMessage?.executed === false && String(reportMessageDryRunBody.result?.structuredContent?.reportMessage?.confirmation ?? "").startsWith(`report_message:${firstReportableMessageID}:`)), "report_message dry-run returns explicit confirmation for an owned reportable PM when available", {
        firstReportableMessageID,
        result: reportMessageDryRunBody.result ?? {}
      }),
      check(validateFleetDispatchDryRun.status === 200 && Number(validateFleetDispatchDryRunBody.result?.structuredContent?.fleetDispatchValidation?.playerId ?? 0) === login.playerID && validateFleetDispatchDryRunBody.result?.structuredContent?.fleetDispatchValidation?.dryRun === true, "validate_fleet_dispatch returns a dry-run validation result without mutation", validateFleetDispatchDryRunBody.result ?? {}),
      check(
        validateFleetDispatchDryRun.status === 200 &&
          Number(validateFleetDispatchDryRunBody.result?.structuredContent?.fleetDispatchValidation?.mission ?? 0) === 15 &&
          Number(validateFleetDispatchDryRunBody.result?.structuredContent?.fleetDispatchValidation?.holdHours ?? 0) === 1 &&
          Number(validateFleetDispatchDryRunBody.result?.structuredContent?.fleetDispatchValidation?.holdSeconds ?? 0) ===
            Math.max(1, Math.round(3600 / Math.max(1, Number(validateFleetDispatchDryRunBody.result?.structuredContent?.fleetDispatchValidation?.speedFactor ?? 1)))),
        "validate_fleet_dispatch exposes fleet-speed-scaled expedition hold timing",
        validateFleetDispatchDryRunBody.result ?? {}
      ),
      check(dispatchFleetWrongConfirm.status === 200 && dispatchFleetWrongConfirmBody.error?.code === -32602, "dispatch_fleet rejects wrong confirmation before mutation", dispatchFleetWrongConfirmBody),
      check(recallFleetDryRun.status === 200 && Number(recallFleetDryRunBody.result?.structuredContent?.recallFleet?.playerId ?? 0) === login.playerID && recallFleetDryRunBody.result?.structuredContent?.recallFleet?.dryRun === true && recallFleetDryRunBody.result?.structuredContent?.recallFleet?.executed === false && recallFleetDryRunBody.result?.structuredContent?.recallFleet?.requiresConfirmation === false && recallFleetDryRunBody.result?.structuredContent?.recallFleet?.issue?.code === "fleet_not_found", "recall_fleet dry-run is available under fleet_write and does not mutate missing fleet ids", recallFleetDryRunBody.result ?? {}),
      check(phalanxScanDryRun.status === 200 && Number(phalanxScanDryRunBody.result?.structuredContent?.phalanxScan?.playerId ?? 0) === login.playerID && phalanxScanDryRunBody.result?.structuredContent?.phalanxScan?.dryRun === true && phalanxScanDryRunBody.result?.structuredContent?.phalanxScan?.executed === false && phalanxScanDryRunBody.result?.structuredContent?.phalanxScan?.requiresConfirmation === false && typeof phalanxScanDryRunBody.result?.structuredContent?.phalanxScan?.issue?.code === "string", "scan_phalanx dry-run is available under fleet_write and does not mutate when validation fails", phalanxScanDryRunBody.result ?? {}),
      check(jumpGateDryRun.status === 200 && Number(jumpGateDryRunBody.result?.structuredContent?.jumpGate?.playerId ?? 0) === login.playerID && jumpGateDryRunBody.result?.structuredContent?.jumpGate?.dryRun === true && jumpGateDryRunBody.result?.structuredContent?.jumpGate?.executed === false && (String(jumpGateDryRunBody.result?.structuredContent?.jumpGate?.confirmation ?? "").startsWith("jump_gate:") || typeof jumpGateDryRunBody.result?.structuredContent?.jumpGate?.issue?.code === "string"), "jump_gate dry-run is available under fleet_write and does not mutate before confirmation", jumpGateDryRunBody.result ?? {}),
      check(cancelBuildingQueueDryRun.status === 200 && Number(cancelBuildingQueueDryRunBody.result?.structuredContent?.cancelBuildingQueue?.playerId ?? 0) === login.playerID && cancelBuildingQueueDryRunBody.result?.structuredContent?.cancelBuildingQueue?.dryRun === true && cancelBuildingQueueDryRunBody.result?.structuredContent?.cancelBuildingQueue?.executed === false && cancelBuildingQueueDryRunBody.result?.structuredContent?.cancelBuildingQueue?.requiresConfirmation === false && cancelBuildingQueueDryRunBody.result?.structuredContent?.cancelBuildingQueue?.issue?.code === "queue_not_found", "cancel_building_queue dry-run is available under queue_write and does not mutate missing rows", cancelBuildingQueueDryRunBody.result ?? {}),
      check(cancelResearchQueueDryRun.status === 200 && Number(cancelResearchQueueDryRunBody.result?.structuredContent?.cancelResearchQueue?.playerId ?? 0) === login.playerID && cancelResearchQueueDryRunBody.result?.structuredContent?.cancelResearchQueue?.dryRun === true && cancelResearchQueueDryRunBody.result?.structuredContent?.cancelResearchQueue?.executed === false && cancelResearchQueueDryRunBody.result?.structuredContent?.cancelResearchQueue?.requiresConfirmation === false && cancelResearchQueueDryRunBody.result?.structuredContent?.cancelResearchQueue?.issue?.code === "queue_not_found", "cancel_research_queue dry-run is available under queue_write and does not mutate missing rows", cancelResearchQueueDryRunBody.result ?? {}),
      check(enqueueShipyardOrderDryRun.status === 200 && Number(enqueueShipyardOrderDryRunBody.result?.structuredContent?.enqueueShipyardOrder?.playerId ?? 0) === login.playerID && enqueueShipyardOrderDryRunBody.result?.structuredContent?.enqueueShipyardOrder?.kind === "fleet" && Number(enqueueShipyardOrderDryRunBody.result?.structuredContent?.enqueueShipyardOrder?.itemId ?? 0) === 204 && Number(enqueueShipyardOrderDryRunBody.result?.structuredContent?.enqueueShipyardOrder?.requested ?? 0) === 1 && enqueueShipyardOrderDryRunBody.result?.structuredContent?.enqueueShipyardOrder?.dryRun === true && enqueueShipyardOrderDryRunBody.result?.structuredContent?.enqueueShipyardOrder?.executed === false, "enqueue_shipyard_order dry-run is available under queue_write and does not mutate before confirmation", enqueueShipyardOrderDryRunBody.result ?? {}),
      check(updateResourceProductionDryRun.status === 200 && Number(updateResourceProductionDryRunBody.result?.structuredContent?.updateResourceProduction?.playerId ?? 0) === login.playerID && updateResourceProductionDryRunBody.result?.structuredContent?.updateResourceProduction?.dryRun === true && updateResourceProductionDryRunBody.result?.structuredContent?.updateResourceProduction?.executed === false && Array.isArray(updateResourceProductionDryRunBody.result?.structuredContent?.updateResourceProduction?.settings), "update_resource_production dry-run is available under resources_write and does not mutate before confirmation", updateResourceProductionDryRunBody.result ?? {}),
      check(recruitOfficerDryRun.status === 200 && Number(recruitOfficerDryRunBody.result?.structuredContent?.recruitOfficer?.playerId ?? 0) === login.playerID && Number(recruitOfficerDryRunBody.result?.structuredContent?.recruitOfficer?.officerId ?? 0) === 1 && recruitOfficerDryRunBody.result?.structuredContent?.recruitOfficer?.dryRun === true && recruitOfficerDryRunBody.result?.structuredContent?.recruitOfficer?.executed === false, "recruit_officer dry-run is available under premium_write and does not mutate before confirmation", recruitOfficerDryRunBody.result ?? {}),
      check(mutateBuildingDryRun.status === 200 && mutateBuildingDryRunBody.result?.structuredContent?.buildingMutation?.dryRun === true && mutateBuildingDryRunBody.result?.structuredContent?.buildingMutation?.executed === false, "mutate_building defaults to a non-mutating dry-run", mutateBuildingDryRunBody.result ?? {}),
      check(startResearchDryRun.status === 200 && startResearchDryRunBody.result?.structuredContent?.researchMutation?.dryRun === true && startResearchDryRunBody.result?.structuredContent?.researchMutation?.executed === false, "start_research defaults to a non-mutating dry-run", startResearchDryRunBody.result ?? {}),
      check(mutatePlanetDryRun.status === 200 && mutatePlanetDryRunBody.result?.structuredContent?.planetMutation?.dryRun === true && mutatePlanetDryRunBody.result?.structuredContent?.planetMutation?.executed === false, "mutate_planet defaults to a non-mutating dry-run", mutatePlanetDryRunBody.result ?? {}),
      check(mutateFleetTemplateDryRun.status === 200 && mutateFleetTemplateDryRunBody.result?.structuredContent?.fleetTemplateMutation?.dryRun === true && mutateFleetTemplateDryRunBody.result?.structuredContent?.fleetTemplateMutation?.executed === false, "mutate_fleet_template validates Commander access without mutating", mutateFleetTemplateDryRunBody.result ?? {}),
      check(mutateCommanderQueueDryRun.status === 200 && mutateCommanderQueueDryRunBody.result?.structuredContent?.commanderQueueMutation?.dryRun === true && mutateCommanderQueueDryRunBody.result?.structuredContent?.commanderQueueMutation?.executed === false, "mutate_commander_queue validates Commander access without mutating", mutateCommanderQueueDryRunBody.result ?? {}),
      check(missileLaunchDryRun.status === 200 && missileLaunchDryRunBody.result?.structuredContent?.missileLaunch?.dryRun === true && missileLaunchDryRunBody.result?.structuredContent?.missileLaunch?.executed === false, "launch_interplanetary_missiles defaults to a non-mutating dry-run", missileLaunchDryRunBody.result ?? {}),
      check(galaxyDispatchDryRun.status === 200 && galaxyDispatchDryRunBody.result?.structuredContent?.galaxyDispatch?.dryRun === true && galaxyDispatchDryRunBody.result?.structuredContent?.galaxyDispatch?.executed === false, "dispatch_galaxy_action defaults to a non-mutating dry-run", galaxyDispatchDryRunBody.result ?? {}),
      check(allianceMutationDryRun.status === 200 && allianceMutationDryRunBody.result?.structuredContent?.allianceMutation?.dryRun === true && allianceMutationDryRunBody.result?.structuredContent?.allianceMutation?.executed === false, "mutate_alliance defaults to a non-mutating dry-run", allianceMutationDryRunBody.result ?? {}),
      check(accountOptionsDryRun.status === 200 && accountOptionsDryRunBody.result?.structuredContent?.accountOptionsMutation?.dryRun === true && accountOptionsDryRunBody.result?.structuredContent?.accountOptionsMutation?.executed === false, "update_account_options preserves omitted settings during dry-run", accountOptionsDryRunBody.result ?? {}),
      check(couponDryRun.status === 200 && couponDryRunBody.result?.structuredContent?.couponRedemption?.dryRun === true && couponDryRunBody.result?.structuredContent?.couponRedemption?.executed === false, "redeem_coupon checks the coupon without redemption during dry-run", couponDryRunBody.result ?? {}),
      check(invalidParamsTool.status === 200 && invalidParamsToolBody.error?.code === -32602, "invalid tool params return JSON-RPC invalid params", invalidParamsToolBody),
      check(Number(tokenRowAfterUse?.lastUsedAt ?? 0) > 0, "bearer tool use updates token last-used timestamp", { tokenRowAfterUse }),
      check(revoke.status === 200 && revokeBody.revoked === true, "MCP token revoke succeeds", revokeBody),
      check(!(tokenListAfterRevokeBody.tokens ?? []).some((token) => Number(token.id ?? 0) === tokenID), "revoked token is removed from token list", tokenListAfterRevokeBody),
      check(accessAfterRevoke.status === 401 && accessAfterRevokeBody.error?.code === -32001, "revoked bearer token is rejected by MCP", accessAfterRevokeBody)
    ]
  }));
} catch (error) {
  cases.push(finalize({
    case: "go_mcp_smoke_runtime",
    checks: [
      check(false, "MCP smoke did not complete", {
        error: error instanceof Error ? error.message : String(error),
        stack: error instanceof Error ? error.stack : undefined
      })
    ]
  }));
}

const result = {
  case_group: "golang_mcp_smoke",
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
