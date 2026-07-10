import { createHash, randomBytes } from "node:crypto";

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
  const oauthJWKS = await request("/.well-known/jwks.json");
  const oauthTokenUnavailable = await request("/oauth/token", { method: "POST" });
  const oauthExternalRedirectReject = await request(`/oauth/authorize?response_type=code&client_id=external&redirect_uri=${encodeURIComponent("https://client.example/callback")}&scope=mcp:read&code_challenge=${"a".repeat(43)}&code_challenge_method=S256`);
  const mcpParseErrorBody = parseJSON(mcpParseError);
  const mcpInitializeBody = parseJSON(mcpInitialize);
  const mcpPingBody = parseJSON(mcpPing);
  const mcpListAnonBody = parseJSON(mcpListAnon);
  const mcpHealthToolBody = parseJSON(mcpHealthTool);
  const mcpUnauthorizedAccessBody = parseJSON(mcpUnauthorizedAccess);
  const oauthMetadataBody = parseJSON(oauthMetadata);
  const oauthJWKSBody = parseJSON(oauthJWKS);
  const oauthTokenUnavailableBody = parseJSON(oauthTokenUnavailable);
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
      check(oauthMetadata.status === 200 && oauthMetadataBody.issuer === baseUrl, "OAuth authorization server metadata uses current origin issuer", oauthMetadataBody),
      check(oauthMetadataBody.authorization_endpoint === `${baseUrl}/oauth/authorize`, "OAuth metadata exposes authorize endpoint", oauthMetadataBody),
      check((oauthMetadataBody.code_challenge_methods_supported ?? []).includes("S256"), "OAuth metadata requires PKCE S256 support", oauthMetadataBody),
      check(oauthMetadataBody.jwks_uri === `${baseUrl}/.well-known/jwks.json` && (oauthMetadataBody.id_token_signing_alg_values_supported ?? []).includes("EdDSA"), "OAuth metadata exposes OIDC JWKS and EdDSA", oauthMetadataBody),
      check(oauthJWKS.status === 200 && (oauthJWKSBody.keys ?? []).some((key) => key.kty === "OKP" && key.crv === "Ed25519"), "OIDC JWKS exposes Ed25519 signing key", oauthJWKSBody),
      check(oauthTokenUnavailable.status === 400 && oauthTokenUnavailableBody.error === "invalid_request", "OAuth token endpoint rejects malformed exchange requests", oauthTokenUnavailableBody),
      check(oauthExternalRedirectReject.status === 400 && oauthExternalRedirectRejectBody.error === "invalid_request", "OAuth authorize rejects non-loopback redirects without allow-list", oauthExternalRedirectRejectBody)
    ]
  }));

  const login = await loginGameUser(universe);
  const sessionID = new URLSearchParams(login.search.startsWith("?") ? login.search.slice(1) : login.search).get("session") ?? "";
  const oauthVerifier = pkceVerifier();
  const oauthClientID = `go-mcp-smoke-${Date.now().toString(36)}`;
  const oauthRedirectURI = `${baseUrl}/oauth/callback`;
  const oauthParams = new URLSearchParams({
    response_type: "code",
    client_id: oauthClientID,
    redirect_uri: oauthRedirectURI,
    scope: "openid profile mcp:read mcp:messages mcp:fleet",
    state: "go-mcp-smoke-state",
    code_challenge: pkceChallenge(oauthVerifier),
    code_challenge_method: "S256",
    session: sessionID
  });
  const oauthConsent = await request(`/oauth/authorize?${oauthParams}`, {
    headers: { Cookie: login.cookiePair }
  });
  const oauthApprove = await request(`/oauth/authorize?${oauthParams}&consent=approve`, {
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
  const oauthTokenRow = (oauthTokenListBody.tokens ?? []).find((token) => String(token.name ?? "") === `OAuth ${oauthClientID}`);
  const oauthRevoke = await request(`/api/game/mcp-tokens/revoke${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({ tokenId: Number(oauthTokenRow?.id ?? 0) })
  });
  const oauthRevokeBody = parseJSON(oauthRevoke);
  const tokenListBefore = await request(`/api/game/mcp-tokens${login.search}`, {
    headers: { Cookie: login.cookiePair }
  });
  const tokenListBeforeBody = parseJSON(tokenListBefore);
  const tokenCreate = await request(`/api/game/mcp-tokens${login.search}`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Cookie: login.cookiePair },
    body: JSON.stringify({
      name: `go-mcp-smoke-${Date.now().toString(36)}`,
      scopes: ["mcp:read", "mcp:messages", "mcp:fleet"]
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
  const queueTool = await mcpJSONRPC("tools/call", { name: "get_building_queue", arguments: {} }, { id: 25, headers: authHeaders });
  const queueToolBody = parseJSON(queueTool);
  const fleetTool = await mcpJSONRPC("tools/call", { name: "get_fleet_movements", arguments: {} }, { id: 26, headers: authHeaders });
  const fleetToolBody = parseJSON(fleetTool);
  const invalidParamsTool = await mcpJSONRPC("tools/call", { name: "get_planet_resources", arguments: { planetId: "abc" } }, { id: 27, headers: authHeaders });
  const invalidParamsToolBody = parseJSON(invalidParamsTool);
  const tokenListAfterUse = await request(`/api/game/mcp-tokens${login.search}`, {
    headers: { Cookie: login.cookiePair }
  });
  const tokenListAfterUseBody = parseJSON(tokenListAfterUse);
  const tokenRowAfterUse = (tokenListAfterUseBody.tokens ?? []).find((token) => Number(token.id ?? 0) === tokenID);
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
  const expectedTools = [
    "get_server_health",
    "get_mcp_access",
    "list_planets",
    "get_account_overview",
    "get_planet_resources",
    "get_building_queue",
    "get_fleet_movements"
  ];
  const authedToolNames = toolNames(authedToolsBody);
  cases.push(finalize({
    case: "go_mcp_user_token_flow",
    checks: [
      check(login.response.status === 200 && login.playerID > 0 && login.cookiePair !== "", "smoke user can log in for MCP token management", {
        status: login.response.status,
        playerID: login.playerID
      }),
      check(sessionID !== "", "smoke login exposes a public session for OAuth consent", { sessionID }),
      check(oauthConsent.status === 200 && oauthConsent.body.includes("Authorize MCP access"), "OAuth authorize shows consent page before approval", { status: oauthConsent.status }),
      check(oauthApprove.status === 302 && oauthApproveLocation.startsWith(oauthRedirectURI) && oauthCallback.searchParams.get("state") === "go-mcp-smoke-state" && oauthCode !== "", "OAuth authorize approval redirects with code and state", { status: oauthApprove.status, location: oauthApproveLocation }),
      check(oauthToken.status === 200 && oauthSecret.startsWith("ogmcp_") && oauthTokenBody.token_type === "Bearer", "OAuth token exchange returns bearer access token", oauthTokenBody),
      check(oauthIDToken.split(".").length === 3 && oauthIDClaims.iss === baseUrl && oauthIDClaims.aud === oauthClientID && oauthIDClaims.sub === `player:${login.playerID}`, "OAuth openid exchange returns ID token claims", oauthIDClaims),
      check(oauthTools.status === 200 && expectedTools.every((name) => toolNames(oauthToolsBody).includes(name)), "OAuth bearer token exposes MCP read tools", { oauthToolNames: toolNames(oauthToolsBody) }),
      check(Number(oauthTokenRow?.id ?? 0) > 0, "OAuth exchange persists a revocable MCP token row", { oauthTokenRow }),
      check(oauthRevoke.status === 200 && oauthRevokeBody.revoked === true, "OAuth-created MCP token can be revoked", oauthRevokeBody),
      check(tokenListBefore.status === 200 && tokenListBeforeBody.authenticated === true && Array.isArray(tokenListBeforeBody.tokens), "MCP token list authenticates game session", tokenListBeforeBody),
      check(tokenCreate.status === 200 && tokenCreateBody.authenticated === true && tokenID > 0, "MCP token create returns a persisted token id", tokenCreateBody.token ?? {}),
      check(secret.startsWith("ogmcp_") && secret.length > 12, "MCP token create returns a one-time bearer secret", {
        secretPrefix: secret.slice(0, 6),
        secretLength: secret.length
      }),
      check(tokenRowAfterCreate !== undefined, "MCP token list includes the newly created token", { tokenID, tokenRowAfterCreate }),
      check(!String(tokenListAfterCreate.body ?? "").includes(secret), "MCP token list never exposes the plaintext secret"),
      check(authedTools.status === 200 && expectedTools.every((name) => authedToolNames.includes(name)), "bearer token exposes all current read tools", {
        authedToolNames
      }),
      check(accessTool.status === 200 && Number(accessToolBody.result?.structuredContent?.playerId ?? 0) === login.playerID, "get_mcp_access returns bearer player id", accessToolBody.result ?? {}),
      check((accessToolBody.result?.structuredContent?.scopes ?? []).includes("mcp:read"), "get_mcp_access returns bearer scopes", accessToolBody.result?.structuredContent ?? {}),
      check(planetsTool.status === 200 && (planetsToolBody.result?.structuredContent?.planets ?? []).length > 0, "list_planets returns at least one planet", planetsToolBody.result ?? {}),
      check(overviewTool.status === 200 && Number(overviewToolBody.result?.structuredContent?.overview?.playerId ?? 0) === login.playerID, "get_account_overview returns current player data", overviewToolBody.result ?? {}),
      check(resourcesTool.status === 200 && Number(resourcesToolBody.result?.structuredContent?.resources?.playerId ?? 0) === login.playerID, "get_planet_resources returns current player data", resourcesToolBody.result ?? {}),
      check(queueTool.status === 200 && Number(queueToolBody.result?.structuredContent?.buildingQueue?.playerId ?? 0) === login.playerID, "get_building_queue returns current player data", queueToolBody.result ?? {}),
      check(fleetTool.status === 200 && Number(fleetToolBody.result?.structuredContent?.fleetMovements?.playerId ?? 0) === login.playerID, "get_fleet_movements returns current player data", fleetToolBody.result ?? {}),
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
