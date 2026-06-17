const baseUrl = (process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890").replace(/\/+$/, "");

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

  const fallback = await request("/game/overview");
  cases.push(finalize({
    case: "go_spa_fallback",
    checks: [
      check(fallback.status === 200, "game route falls back to React shell", { status: fallback.status }),
      check(fallback.body.includes('<div id="root">'), "fallback response renders React mount node")
    ]
  }));

  const naturalPublicPaths = [
    "/home",
    "/register",
    "/universes",
    "/about",
    "/story",
    "/screenshots",
    "/rules",
    "/legal",
    "/migration"
  ];
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

  const legacyPublicPaths = [
    "/about.php",
    "/home.php",
    "/impressum.php",
    "/index.php",
    "/install.php",
    "/register.php",
    "/regeln.php",
    "/screenshots.php",
    "/story.php",
    "/unis.php"
  ];
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
      check(js.body.includes("/register") && js.body.includes("/universes"), "React bundle contains natural public route model")
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
  cases.push(finalize({
    case: "go_method_guards",
    checks: [
      check(postHealth.status === 405, "POST health endpoint is rejected", { status: postHealth.status }),
      check(hasHeader(postHealth, "allow", "GET, HEAD"), "method rejection returns Allow header")
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
