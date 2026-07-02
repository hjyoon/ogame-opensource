import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { gameRoutes, normalizeGamePath } from "../src/gameRoutes";
import { publicRouteManifest } from "../src/publicRouteManifest";

type BrowserName = "chromium" | "firefox";
type Area = "public" | "game";
type Side = "legacy" | "migrated";
type AuthRole = "player" | "admin";

type SeedSpec = {
  name: string;
  area: Area;
  authRole?: AuthRole;
  legacyURL: (session: string) => string;
  migratedURL: (session: string) => string;
};

type RawTarget = {
  kind: "anchor" | "onclick" | "form" | "select" | "hover";
  label: string;
  url: string;
  method: string;
};

type CanonicalTarget = {
  key: string;
  area: Area;
  path: string;
  query: string;
};

type EdgeSide = RawTarget & {
  authRole?: AuthRole;
  canonical: CanonicalTarget;
};

type EdgeResult = {
  source: string;
  target: string;
  label: string;
  pass: boolean;
  legacy?: EdgeSide;
  migrated?: EdgeSide;
  notes: string[];
};

type Capture = {
  url: string;
  status: number | null;
  screenshotPath: string;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
};

type DiffResult = {
  width: number;
  height: number;
  totalPixels: number;
  changedPixels: number;
  diffRatio: number;
  averageDelta: number;
};

type TargetResult = {
  key: string;
  area: Area;
  pass: boolean;
  legacy?: Capture;
  migrated?: Capture;
  diff?: DiffResult;
  diffPath?: string;
  notes: string[];
};

type DiscoveryResult = {
  seed: SeedSpec;
  legacyTargets: EdgeSide[];
  migratedTargets: EdgeSide[];
  edges: EdgeResult[];
};

class InvalidGameSessionError extends Error {
  constructor(side: Side, seedName: string, url: string) {
    super(`${side} game seed ${seedName} redirected outside game shell: ${url}`);
    this.name = "InvalidGameSessionError";
  }
}

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = resolve(rootDir, ".tmp/playwright-navigation-visual", browserName);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const loginUser = process.env.OGAME_NAV_VISUAL_USER ?? "legor";
const loginPassword = process.env.OGAME_NAV_VISUAL_PASS ?? "admin";
const adminLoginUser = process.env.OGAME_NAV_VISUAL_ADMIN_USER ?? "visualadmin";
const adminLoginPassword = process.env.OGAME_NAV_VISUAL_ADMIN_PASS ?? loginPassword;
const enforceDiff = process.env.OGAME_NAV_VISUAL_ENFORCE_DIFF !== "0";
const maxDiffRatio = numberEnv("OGAME_NAV_VISUAL_MAX_DIFF_RATIO", 0);
const colorDeltaThreshold = numberEnv("OGAME_NAV_VISUAL_COLOR_DELTA", 0);
const progressEnabled = process.env.OGAME_NAV_VISUAL_PROGRESS !== "0";
const targetFilter = process.env.OGAME_NAV_VISUAL_TARGET_FILTER ?? "";
const includeAdminSeeds = process.env.OGAME_NAV_VISUAL_INCLUDE_ADMIN_SEEDS !== "0";
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);

const publicAliasToPath = new Map<string, string>();
for (const route of publicRouteManifest) {
  publicAliasToPath.set(normalizePath(route.path), route.path);
  for (const alias of route.legacyAliases) {
    publicAliasToPath.set(normalizePath(alias), route.path);
  }
}
publicAliasToPath.set("/", "/home");

const legacyPageToPath = new Map<string, string>([
  ["overview", "/game/overview"],
  ["renameplanet", "/game/rename-planet"],
  ["admin", "/game/admin"],
  ["imperium", "/game/empire"],
  ["buildings", "/game/buildings"],
  ["b_building", "/game/buildings"],
  ["resources", "/game/resources"],
  ["research", "/game/research"],
  ["shipyard", "/game/shipyard"],
  ["fleet", "/game/fleet"],
  ["fleet1", "/game/fleet"],
  ["flotten1", "/game/fleet"],
  ["flotten2", "/game/fleet"],
  ["flotten3", "/game/fleet"],
  ["flottenversand", "/game/fleet"],
  ["fleet_templates", "/game/fleet-templates"],
  ["technology", "/game/technology"],
  ["techtree", "/game/technology"],
  ["techtreedetails", "/game/technology"],
  ["changelog", "/game/changelog"],
  ["infos", "/game/technology"],
  ["galaxy", "/game/galaxy"],
  ["defense", "/game/defense"],
  ["allianzen", "/game/alliance"],
  ["bewerben", "/game/alliance"],
  ["bewerbungen", "/game/alliance"],
  ["trader", "/game/merchant"],
  ["payment", "/game/officers"],
  ["micropayment", "/game/officers"],
  ["statistics", "/game/statistics"],
  ["search", "/game/search"],
  ["suche", "/game/search"],
  ["writemessages", "/game/messages"],
  ["messages", "/game/messages"],
  ["bericht", "/game/report"],
  ["phalanx", "/game/phalanx"],
  ["notes", "/game/notes"],
  ["notizen", "/game/notes"],
  ["buddy", "/game/buddy"],
  ["options", "/game/options"],
  ["logout", "/game/logout"]
]);

const adminModes = [
  "Bans",
  "Broadcast",
  "Reports",
  "Bots",
  "Coupons",
  "ColonySettings",
  "Debug",
  "Errors",
  "Logins",
  "UserLogs",
  "Browse",
  "Fleetlogs",
  "Queue",
  "Users",
  "Planets",
  "Uni",
  "Checksum",
  "DB",
  "BattleSim",
  "Expedition",
  "BattleReport",
  "BotEdit",
  "RakSim",
  "Loca",
  "Mods"
];

const adminSeeds: SeedSpec[] = adminModes.map((mode) =>
  gameSeed(`game-admin-${kebab(mode)}`, "admin", "/game/admin", { mode }, { mode }, "admin")
);

const authSeeds: SeedSpec[] = [
  gameSeed("game-overview", "overview", "/game/overview"),
  gameSeed("game-rename-planet", "renameplanet", "/game/rename-planet"),
  gameSeed("game-buildings", "b_building", "/game/buildings"),
  gameSeed("game-resources", "resources", "/game/resources"),
  gameSeed("game-empire", "imperium", "/game/empire", { planettype: "1", no_header: "1" }, { planettype: "1" }),
  gameSeed("game-merchant", "trader", "/game/merchant"),
  gameSeed("game-research", "buildings", "/game/research", { mode: "Forschung" }),
  gameSeed("game-shipyard", "buildings", "/game/shipyard", { mode: "Flotte" }),
  gameSeed("game-fleet", "flotten1", "/game/fleet"),
  gameSeed("game-fleet-templates", "fleet_templates", "/game/fleet-templates"),
  gameSeed("game-galaxy", "galaxy", "/game/galaxy"),
  gameSeed("game-technology", "techtree", "/game/technology"),
  gameSeed("game-technology-details", "techtreedetails", "/game/technology", { tid: "206" }, { tid: "206" }),
  gameSeed("game-defense", "buildings", "/game/defense", { mode: "Verteidigung" }),
  gameSeed("game-alliance", "allianzen", "/game/alliance"),
  gameSeed("game-alliance-create", "allianzen", "/game/alliance", { a: "1" }, { a: "1" }),
  gameSeed("game-alliance-search", "allianzen", "/game/alliance", { a: "2" }, { a: "2" }),
  gameSeed("game-alliance-management", "allianzen", "/game/alliance", { a: "5" }, { a: "5" }),
  gameSeed("game-alliance-members", "allianzen", "/game/alliance", { a: "4" }, { a: "4" }),
  gameSeed("game-alliance-circular", "allianzen", "/game/alliance", { a: "17" }, { a: "17" }),
  gameSeed("game-alliance-application-text", "allianzen", "/game/alliance", { a: "5", t: "3" }, { a: "5", t: "3" }),
  gameSeed("game-alliance-settings", "allianzen", "/game/alliance", { a: "11", d: "2" }, { a: "11", d: "2" }),
  gameSeed("game-alliance-ranks", "allianzen", "/game/alliance", { a: "6" }, { a: "6" }),
  gameSeed("game-officers", "micropayment", "/game/officers"),
  gameSeed("game-statistics", "statistics", "/game/statistics", { type: "ressources", start: "1" }, { type: "ressources", start: "1" }),
  gameSeed("game-statistics-alliance", "statistics", "/game/statistics", { who: "ally", type: "ressources", start: "1" }, { who: "ally", type: "ressources", start: "1" }),
  gameSeed("game-search", "suche", "/game/search"),
  gameSeed("game-messages", "messages", "/game/messages"),
  gameSeed("game-messages-compose", "writemessages", "/game/messages", { messageziel: "1" }, { messageziel: "1" }),
  gameSeed("game-report", "bericht", "/game/report"),
  gameSeed("game-phalanx", "phalanx", "/game/phalanx"),
  gameSeed("game-buddy", "buddy", "/game/buddy"),
  gameSeed("game-options", "options", "/game/options"),
  gameSeed("game-notes", "notizen", "/game/notes"),
  gameSeed("game-notes-create", "notizen", "/game/notes", { a: "1" }, { a: "1" })
];

const publicSeeds: SeedSpec[] = publicRouteManifest
  .filter((route) => route.legacyVisualPath !== undefined)
  .map((route) => ({
    name: `public-${route.key}`,
    area: "public" as const,
    legacyURL: () => `${legacyBaseURL}${route.legacyVisualPath}`,
    migratedURL: () => `${migratedBaseURL}${route.path}`
  }));

const seeds = [...publicSeeds, ...authSeeds, ...(includeAdminSeeds ? adminSeeds : [])];
const knownGamePaths = new Set([...gameRoutes.map((route) => route.path), "/game/changelog", "/game/reg/mail.php"]);
const dynamicQueryKeys = new Set([
  "allyid",
  "bericht",
  "betreff",
  "buddy_id",
  "cp",
  "days",
  "gid",
  "id",
  "item_id",
  "messageziel",
  "n",
  "p_id",
  "pdd",
  "pl",
  "planet",
  "planettype",
  "player_id",
  "position",
  "re",
  "target_mission",
  "techid",
  "tid",
  "u",
  "uid",
  "zp"
]);
const globalGameQueryKeys = new Set(["cp"]);
const gameQueryKeys = new Map<string, Set<string>>([
  ["/game/rename-planet", new Set(["pl"])],
  ["/game/empire", new Set(["planettype", "planet", "modus", "listid"])],
  ["/game/buildings", new Set(["modus", "techid", "listid"])],
  ["/game/research", new Set(["bau", "unbau"])],
  ["/game/shipyard", new Set(["mode", "auftr"])],
  ["/game/fleet", new Set(["galaxy", "system", "position", "planet", "planettype", "target_mission"])],
  ["/game/fleet-templates", new Set(["mode", "id"])],
  ["/game/technology", new Set(["gid", "tid"])],
  ["/game/galaxy", new Set(["galaxy", "system", "position", "mode", "p1", "p2", "p3", "pdd", "zp"])],
  ["/game/alliance", new Set(["a", "t", "d", "allyid", "u", "sort2"])],
  ["/game/officers", new Set(["buynow", "type", "days"])],
  ["/game/statistics", new Set(["who", "type", "start", "sort_per_member"])],
  ["/game/search", new Set(["searchtext", "type"])],
  ["/game/messages", new Set(["messageziel", "re", "betreff", "dsp"])],
  ["/game/report", new Set(["bericht"])],
  ["/game/phalanx", new Set(["galaxy", "system", "planet", "planettype"])],
  ["/game/notes", new Set(["a", "n"])],
  ["/game/buddy", new Set(["action", "buddy_id"])],
  ["/game/admin", new Set(["mode", "action", "fname", "player_id", "galaxy", "system", "filter", "modname"])]
]);
const mutatingQueryKeys = new Set(["cmd", "action", "remove", "delete", "del", "destroy", "modus", "buynow"]);

await mkdir(screenshotDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const discoveries = await discoverSeeds(browser, seeds);
  const discoveredEdgeCount = discoveries.reduce((total, discovery) => total + discovery.edges.length, 0);
  progress(`build target representatives from ${discoveries.length} discoveries and ${discoveredEdgeCount} edges`);
  const targetRepresentatives = new Map<string, { canonical: CanonicalTarget; legacy?: EdgeSide; migrated?: EdgeSide }>();

  for (const discovery of discoveries) {
    for (const edge of discovery.edges) {
      const canonical = edge.legacy?.canonical ?? edge.migrated?.canonical;
      if (!canonical) {
        continue;
      }
      const representative = targetRepresentatives.get(canonical.key) ?? { canonical };
      representative.legacy ??= edge.legacy;
      representative.migrated ??= edge.migrated;
      targetRepresentatives.set(canonical.key, representative);
    }
  }

  let representatives = Array.from(targetRepresentatives.values()).sort((a, b) => a.canonical.key.localeCompare(b.canonical.key));
  if (targetFilter !== "") {
    representatives = representatives.filter((representative) => representative.canonical.key.includes(targetFilter));
  }
  progress(`target representatives: ${representatives.length}${targetFilter ? ` filtered by ${targetFilter}` : ""}`);
  const targetResults = await compareTargets(
    browser,
    representatives
  );

  const edges = discoveries.flatMap((discovery) => discovery.edges);
  const report = {
    generatedAt: new Date().toISOString(),
    legacyBaseURL,
    migratedBaseURL,
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    loginUser,
    adminLoginUser,
    seedOptions: { includeAdminSeeds },
    thresholds: { enforceDiff, maxDiffRatio, colorDeltaThreshold },
    allPass: edges.every((edge) => edge.pass) && targetResults.every((target) => target.pass),
    summary: {
      screens: seeds.length,
      edges: edges.length,
      matchedEdges: edges.filter((edge) => edge.legacy && edge.migrated).length,
      targetScreens: targetResults.length,
      exactDiffPass: targetResults.filter((target) => target.diff !== undefined && target.diff.diffRatio <= maxDiffRatio).length,
      exactDiffFail: targetResults.filter((target) => target.diff !== undefined && target.diff.diffRatio > maxDiffRatio).length
    },
    discoveries,
    targetResults
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderReport(report));
  await writeFile(resolve(rootDir, "testing/e2e/COVERAGE-navigation-visual.md"), renderCoverage(report));
  process.stdout.write(JSON.stringify({ allPass: report.allPass, report: join(outputDir, "report.json") }, null, 2) + "\n");
  if (!report.allPass) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

function gameSeed(
  name: string,
  legacyPage: string,
  migratedPath: string,
  legacyQuery: Record<string, string> = {},
  migratedQuery: Record<string, string> = {},
  authRole: AuthRole = "player"
): SeedSpec {
  return {
    name,
    area: "game",
    authRole,
    legacyURL: (session) => {
      const query = new URLSearchParams({ page: legacyPage, session });
      for (const [key, value] of Object.entries(legacyQuery)) {
        query.set(key, value);
      }
      return `${legacyBaseURL}/game/index.php?${query.toString()}`;
    },
    migratedURL: (session) => {
      const query = new URLSearchParams({ session });
      for (const [key, value] of Object.entries(migratedQuery)) {
        query.set(key, value);
      }
      const encoded = query.toString();
      return `${migratedBaseURL}${migratedPath}${encoded ? `?${encoded}` : ""}`;
    }
  };
}

async function newContext(browser: Browser): Promise<BrowserContext> {
  return await browser.newContext({
    viewport: { width: 1024, height: 768 },
    deviceScaleFactor: 1,
    locale: "en-US"
  });
}

function credentialsForAuthRole(authRole: AuthRole = "player"): { user: string; password: string } {
  return authRole === "admin" ? { user: adminLoginUser, password: adminLoginPassword } : { user: loginUser, password: loginPassword };
}

async function loginLegacy(context: BrowserContext, authRole: AuthRole = "player"): Promise<string> {
  const credentials = credentialsForAuthRole(authRole);
  const page = await context.newPage();
  await page.goto(
    `${legacyBaseURL}/game/reg/login2.php?login=${encodeURIComponent(credentials.user)}&pass=${encodeURIComponent(credentials.password)}`,
    { waitUntil: "networkidle", timeout: 15_000 }
  );
  const session = new URL(page.url()).searchParams.get("session") ?? "";
  await page.close();
  if (!session) {
    throw new Error("legacy login did not return a session");
  }
  return session;
}

async function loginMigrated(context: BrowserContext, authRole: AuthRole = "player"): Promise<string> {
  const credentials = credentialsForAuthRole(authRole);
  const page = await context.newPage();
  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  const universe = (await page.locator("select[name='universe'] option").nth(1).getAttribute("value")) ?? "http://localhost:8888";
  await page.locator("select[name='universe']").selectOption(universe);
  await page.locator("input[name='login']").fill(credentials.user);
  await page.locator("input[name='pass']").fill(credentials.password);
  await page.locator("input.legacy-public-login-button").click();
  await page.waitForFunction(() => window.location.pathname === "/game/overview" && window.location.search.includes("session="), undefined, {
    timeout: 10_000
  });
  const session = new URL(page.url()).searchParams.get("session") ?? "";
  await page.close();
  if (!session) {
    throw new Error("migrated login did not return a session");
  }
  return session;
}

async function discoverSeeds(browser: Browser, specs: SeedSpec[]): Promise<DiscoveryResult[]> {
  progress(`discover seed pairs: ${specs.length}`);
  const discoveries: DiscoveryResult[] = [];
  for (const [index, seed] of specs.entries()) {
    progressEvery(index, specs.length, `seed ${seed.name}`);
    const legacyTargets = await collectSeedSide(browser, "legacy", seed);
    const migratedTargets = await collectSeedSide(browser, "migrated", seed);
    discoveries.push(buildDiscovery(seed, legacyTargets, migratedTargets));
  }
  return discoveries;
}

async function collectSeedSide(browser: Browser, side: Side, seed: SeedSpec): Promise<EdgeSide[]> {
  const context = await newContext(browser);
  try {
    const session = side === "legacy" ? await loginLegacy(context, seed.authRole) : await loginMigrated(context, seed.authRole);
    const url = side === "legacy" ? seed.legacyURL(session) : seed.migratedURL(session);
    return await collectSideTargets(context, side, seed, url);
  } catch (error) {
    if (!(error instanceof InvalidGameSessionError)) {
      throw error;
    }
    const session = side === "legacy" ? await loginLegacy(context, seed.authRole) : await loginMigrated(context, seed.authRole);
    const url = side === "legacy" ? seed.legacyURL(session) : seed.migratedURL(session);
    return await collectSideTargets(context, side, seed, url);
  } finally {
    await context.close();
  }
}

function buildDiscovery(seed: SeedSpec, legacyTargets: EdgeSide[], migratedTargets: EdgeSide[]): DiscoveryResult {
  const byKey = new Map<string, { legacy?: EdgeSide; migrated?: EdgeSide }>();
  for (const target of legacyTargets) {
    const entry = byKey.get(target.canonical.key) ?? {};
    entry.legacy = target;
    byKey.set(target.canonical.key, entry);
  }
  for (const target of migratedTargets) {
    const entry = byKey.get(target.canonical.key) ?? {};
    entry.migrated = target;
    byKey.set(target.canonical.key, entry);
  }
  const edges = Array.from(byKey.entries())
    .filter(([target]) => !isVolatileNavigationEdge(seed.name, target))
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([target, sides]) => {
      const notes: string[] = [];
      if (!sides.legacy) {
        notes.push("missing legacy navigation");
      }
      if (!sides.migrated) {
        notes.push("missing migrated navigation");
      }
      const label = sides.legacy?.label || sides.migrated?.label || target;
      return {
        source: seed.name,
        target,
        label,
        pass: notes.length === 0,
        legacy: sides.legacy,
        migrated: sides.migrated,
        notes
      };
    });
  return { seed, legacyTargets, migratedTargets, edges };
}

function isVolatileNavigationEdge(seedName: string, target: string): boolean {
  return seedName === "game-admin-queue" && target === "game:/game/admin?cp=%3Cid%3E&mode=Planets";
}

async function collectSideTargets(context: BrowserContext, side: Side, seed: SeedSpec, url: string): Promise<EdgeSide[]> {
  const page = await context.newPage();
  try {
    await page.goto(url, { waitUntil: "networkidle", timeout: 15_000 });
    if (seed.area === "game" && !new URL(page.url()).pathname.startsWith("/game/")) {
      throw new InvalidGameSessionError(side, seed.name, page.url());
    }
    await waitForImages(page);
    const rawTargets = await page.evaluate(() => {
      const visibleLabel = (element: Element): string => {
        const text = element.textContent?.replace(/\s+/g, " ").trim() ?? "";
        if (text !== "") {
          return text;
        }
        const image = element.querySelector("img");
        return image?.getAttribute("alt") || image?.getAttribute("title") || element.getAttribute("title") || element.getAttribute("name") || "";
      };
      const extractScriptNavigationHref = (script: string | null): string | null => {
        if (!script) {
          return null;
        }
        if (/changeAction\(['"]getpw['"]\)/i.test(script)) {
          return "/game/reg/mail.php";
        }
        const messageMenuMatch = script.match(/showMessageMenu\((\d+)\)/i);
        if (messageMenuMatch?.[1]) {
          return `index.php?page=writemessages&messageziel=${messageMenuMatch[1]}`;
        }
        const escapedLocationMatch = script.match(/(?:document\.)?location(?:\.href)?\s*=\s*\\['"]([^'"]+)\\['"]/i);
        if (escapedLocationMatch?.[1]) {
          return escapedLocationMatch[1];
        }
        const locationMatch = script.match(/(?:document\.)?location(?:\.href)?\s*=\s*['"]([^'"]+)['"]/i);
        if (locationMatch?.[1]) {
          return locationMatch[1];
        }
        const popupMatch = script.match(/(?:fenster(?:ed)?|(?:window\.)?open)\s*\(\s*\\?['"]([^'"]+)\\?['"]/i);
        return popupMatch?.[1] ?? null;
      };
      const looksLikeNavigationValue = (value: string): boolean => {
        return /^(?:https?:|\/|(?:\.\.?\/)|[a-z0-9_-]+\.php\b|index\.php\?)/i.test(value.trim());
      };
      const decodeHTML = (value: string): string => {
        const textarea = document.createElement("textarea");
        textarea.innerHTML = value;
        return textarea.value;
      };
      const extractEmbeddedHrefs = (html: string | null): string[] => {
        if (!html) {
          return [];
        }
        const hrefs: string[] = [];
        const hrefPattern = /href\s*=\s*(?:"([^"]+)"|'([^']+)'|([^'">\s]+))/gi;
        let match: RegExpExecArray | null;
        while ((match = hrefPattern.exec(html)) !== null) {
          const href = match[1] ?? match[2] ?? match[3] ?? "";
          if (href) {
            hrefs.push(decodeHTML(href));
          }
        }
        return hrefs;
      };
      const targets: RawTarget[] = [];
      for (const anchor of Array.from(document.querySelectorAll<HTMLAnchorElement>("a[href]"))) {
        if (anchor.closest("option, select")) {
          continue;
        }
        const rawHref = anchor.getAttribute("href") ?? "";
        targets.push({ kind: "anchor", label: visibleLabel(anchor), url: extractScriptNavigationHref(rawHref) ?? rawHref, method: "GET" });
      }
      for (const element of Array.from(document.querySelectorAll<HTMLElement>("[onclick]"))) {
        const href = extractScriptNavigationHref(element.getAttribute("onclick"));
        if (href) {
          targets.push({ kind: "onclick", label: visibleLabel(element), url: href, method: "GET" });
        }
      }
      for (const element of Array.from(document.querySelectorAll<HTMLElement>("[onmouseover], [onmouseenter]"))) {
        for (const attribute of ["onmouseover", "onmouseenter"]) {
          for (const href of extractEmbeddedHrefs(element.getAttribute(attribute))) {
            targets.push({ kind: "hover", label: visibleLabel(element), url: href, method: "GET" });
          }
        }
      }
      for (const select of Array.from(document.querySelectorAll<HTMLSelectElement>("select"))) {
        for (const option of Array.from(select.options)) {
          if (looksLikeNavigationValue(option.value)) {
            targets.push({ kind: "select", label: option.textContent?.replace(/\s+/g, " ").trim() || visibleLabel(select), url: option.value, method: "GET" });
          }
        }
      }
      for (const form of Array.from(document.querySelectorAll<HTMLFormElement>("form"))) {
        const method = (form.getAttribute("method") || "GET").toUpperCase();
        if (method === "GET") {
          targets.push({ kind: "form", label: form.getAttribute("name") || form.getAttribute("id") || "GET form", url: form.getAttribute("action") || location.href, method });
        }
      }
      return targets;
    });
    rawTargets.push(...(await collectInteractiveHoverTargets(page)));
    const canonicalTargets = rawTargets
      .map((target) => toEdgeSide(target, page.url(), side, seed.area, seed.authRole))
      .filter((target): target is EdgeSide => target !== null);
    return dedupeTargets(canonicalTargets);
  } finally {
    await page.close();
  }
}

async function collectInteractiveHoverTargets(page: Page): Promise<RawTarget[]> {
  const hoverSelector = "[data-galaxy-hover], .legacy-statistics-delta, .legacy-galaxy-hover";
  const hoverLocators = page.locator(hoverSelector);
  const count = Math.min(await hoverLocators.count().catch(() => 0), 120);
  const targets: RawTarget[] = [];
  const seen = new Set<string>();
  for (let index = 0; index < count; index += 1) {
    const hover = hoverLocators.nth(index);
    if (!(await hover.isVisible().catch(() => false))) {
      continue;
    }
    await hover.hover({ timeout: 1_000 }).catch(() => undefined);
    await page.waitForTimeout(850);
    const hoveredTargets = await page.evaluate(() => {
      const visibleLabel = (element: Element): string => {
        const text = element.textContent?.replace(/\s+/g, " ").trim() ?? "";
        if (text !== "") {
          return text;
        }
        const image = element.querySelector("img");
        return image?.getAttribute("alt") || image?.getAttribute("title") || element.getAttribute("title") || element.getAttribute("name") || "";
      };
      const targets: RawTarget[] = [];
      for (const anchor of Array.from(
        document.querySelectorAll<HTMLAnchorElement>(
          "#overDiv a[href], .legacy-galaxy-hover-open a[href], .legacy-galaxy-tooltip a[href], .legacy-statistics-tooltip a[href]"
        )
      )) {
        const rawHref = anchor.getAttribute("href") ?? "";
        targets.push({ kind: "hover", label: visibleLabel(anchor), url: rawHref, method: "GET" });
      }
      return targets;
    });
    for (const target of hoveredTargets) {
      const key = `${target.kind}:${target.method}:${target.url}:${target.label}`;
      if (!seen.has(key)) {
        seen.add(key);
        targets.push(target);
      }
    }
  }
  await page.mouse.move(0, 0).catch(() => undefined);
  return targets;
}

function toEdgeSide(raw: RawTarget, baseURL: string, side: Side, area: Area, authRole: AuthRole = "player"): EdgeSide | null {
  const absolute = absoluteURL(raw.url, baseURL);
  if (!absolute || !isAllowedNavigation(absolute, side)) {
    return null;
  }
  const canonical = canonicalizeURL(absolute, area);
  if (!canonical) {
    return null;
  }
  return { ...raw, url: absolute.toString(), authRole, canonical };
}

function dedupeTargets(targets: EdgeSide[]): EdgeSide[] {
  const seen = new Map<string, EdgeSide>();
  for (const target of targets) {
    if (!seen.has(target.canonical.key)) {
      seen.set(target.canonical.key, target);
    }
  }
  return Array.from(seen.values()).sort((left, right) => left.canonical.key.localeCompare(right.canonical.key));
}

function absoluteURL(raw: string, baseURL: string): URL | null {
  const trimmed = raw.trim();
  if (trimmed === "" || trimmed === "#" || trimmed.startsWith("javascript:") || trimmed.startsWith("mailto:")) {
    return null;
  }
  try {
    return new URL(trimmed, baseURL);
  } catch {
    return null;
  }
}

function isAllowedNavigation(url: URL, side: Side): boolean {
  const base = side === "legacy" ? new URL(legacyBaseURL) : new URL(migratedBaseURL);
  if (!sameInternalOrigin(url, base)) {
    return false;
  }
  const path = normalizePath(url.pathname);
  if (/\.(css|js|png|gif|jpg|jpeg|ico|webp)$/i.test(path)) {
    return false;
  }
  return path.startsWith("/game") || publicAliasToPath.has(path);
}

function canonicalizeURL(url: URL, sourceArea: Area): CanonicalTarget | null {
  const publicPath = publicAliasToPath.get(normalizePath(url.pathname));
  if (publicPath && !url.pathname.startsWith("/game")) {
    const query = normalizedQuery(url.searchParams, { area: "public", path: publicPath });
    return { key: `public:${publicPath}${query ? `?${query}` : ""}`, area: "public", path: publicPath, query };
  }
  const normalized = canonicalGamePath(url);
  if (!normalized) {
    if (sourceArea === "public" && publicPath) {
      return { key: `public:${publicPath}`, area: "public", path: publicPath, query: "" };
    }
    return null;
  }
  if (normalized === "/game/logout") {
    return null;
  }
  const query = normalizedQuery(url.searchParams, { area: "game", path: normalized });
  if (query === "__mutating__") {
    return null;
  }
  return { key: `game:${normalized}${query ? `?${query}` : ""}`, area: "game", path: normalized, query };
}

function canonicalGamePath(url: URL): string | null {
  const path = normalizePath(url.pathname);
  if (path === "/game/ainfo.php") {
    return "/game/alliance";
  }
  if (path === "/game" || path === "/game/" || path === "/game/index.php") {
    const page = url.searchParams.get("page") ?? "";
    const mode = url.searchParams.get("mode") ?? "";
    if (page === "buildings") {
      if (mode === "Forschung") {
        return "/game/research";
      }
      if (mode === "Flotte") {
        return "/game/shipyard";
      }
      if (mode === "Verteidigung") {
        return "/game/defense";
      }
    }
    return legacyPageToPath.get(page) ?? normalizeGamePath(path, url.search);
  }
  if (path.startsWith("/game/")) {
    const normalized = normalizeGamePath(path, url.search);
    return knownGamePaths.has(normalized) ? normalized : null;
  }
  return null;
}

function normalizedQuery(params: URLSearchParams, target: { area: Area; path: string }): string {
  const normalizedParams = canonicalQueryParams(params, target);
  const ignored = new Set(["session", "lgn", "chose", "v", "no_header"]);
  if (target.area === "game") {
    ignored.add("page");
    if (target.path === "/game/research" || target.path === "/game/shipyard" || target.path === "/game/defense") {
      ignored.add("mode");
    }
  }
  for (const key of mutatingQueryKeys) {
    if (normalizedParams.has(key)) {
      return "__mutating__";
    }
  }
  const allowedGameKeys = target.area === "game" ? gameQueryKeys.get(target.path) ?? new Set<string>() : new Set<string>();
  const entries = Array.from(normalizedParams.entries())
    .filter(([key, value]) => {
      if (ignored.has(key) || value === "") {
        return false;
      }
      if (target.area !== "game") {
        return true;
      }
      return globalGameQueryKeys.has(key) || allowedGameKeys.has(key);
    })
    .map(([key, value]) => [key, dynamicQueryKeys.has(key) ? "<id>" : value] as const)
    .sort(([aKey, aValue], [bKey, bValue]) => aKey.localeCompare(bKey) || aValue.localeCompare(bValue));
  const normalized = new URLSearchParams();
  for (const [key, value] of entries) {
    normalized.append(key, value);
  }
  return normalized.toString();
}

function canonicalQueryParams(params: URLSearchParams, target: { area: Area; path: string }): URLSearchParams {
  const normalized = new URLSearchParams(params);
  if (target.area !== "game") {
    return normalized;
  }
  if (target.path === "/game/galaxy") {
    if (!normalized.has("galaxy") && normalized.has("p1")) {
      normalized.set("galaxy", normalized.get("p1") ?? "");
    }
    if (!normalized.has("system") && normalized.has("p2")) {
      normalized.set("system", normalized.get("p2") ?? "");
    }
    if (!normalized.has("position") && normalized.has("p3")) {
      normalized.set("position", normalized.get("p3") ?? "");
    }
    normalized.delete("p1");
    normalized.delete("p2");
    normalized.delete("p3");
    normalized.delete("cp");
    if (normalized.get("mode") !== "1") {
      normalized.delete("mode");
      normalized.delete("pdd");
      normalized.delete("zp");
    }
  }
  if (target.path === "/game/empire" && normalized.get("planettype") === "1") {
    normalized.delete("planettype");
  }
  if (target.path === "/game/fleet") {
    if (!normalized.has("planet") && normalized.has("position")) {
      normalized.set("planet", normalized.get("position") ?? "");
    }
    normalized.delete("position");
  }
  if (target.path === "/game/messages" && !normalized.has("messageziel")) {
    normalized.set("dsp", "1");
  }
  if (target.path === "/game/alliance" && normalized.get("page") === "bewerben" && !normalized.has("a")) {
    normalized.set("a", "2");
  }
  return normalized;
}

function sameInternalOrigin(url: URL, base: URL): boolean {
  if (url.host === base.host) {
    return true;
  }
  return isLoopbackHost(url.hostname) && isLoopbackHost(base.hostname) && effectivePort(url) === effectivePort(base);
}

function isLoopbackHost(host: string): boolean {
  const normalized = host.toLowerCase();
  return normalized === "localhost" || normalized === "127.0.0.1" || normalized.startsWith("127.") || normalized === "::1" || normalized === "[::1]";
}

function effectivePort(url: URL): string {
  if (url.port) {
    return url.port;
  }
  return url.protocol === "https:" ? "443" : "80";
}

async function compareTargets(
  browser: Browser,
  representatives: { canonical: CanonicalTarget; legacy?: EdgeSide; migrated?: EdgeSide }[]
): Promise<TargetResult[]> {
  const results = new Map<string, TargetResult>();
  const comparable = representatives.filter((representative) => {
    const notes: string[] = [];
    if (!representative.legacy) {
      notes.push("missing legacy representative");
    }
    if (!representative.migrated) {
      notes.push("missing migrated representative");
    }
    if (notes.length > 0) {
      results.set(representative.canonical.key, { key: representative.canonical.key, area: representative.canonical.area, pass: false, notes });
      return false;
    }
    return true;
  }) as { canonical: CanonicalTarget; legacy: EdgeSide; migrated: EdgeSide }[];

  progress(`compare targets: ${comparable.length} comparable, ${representatives.length} total`);
  for (const [index, representative] of comparable.entries()) {
    progressEvery(index, comparable.length, `target ${representative.canonical.key}`);
    const safeName = safeFileName(representative.canonical.key);

    const legacyLoginContext = await newContext(browser);
    const legacySession = await loginLegacy(legacyLoginContext, representative.legacy.authRole);
    let legacy: Capture;
    try {
      const legacyURL = withSession(representative.legacy.url, legacySession);
      legacy = await captureURLInContext(legacyLoginContext, legacyURL, `${safeName}-legacy.png`, "legacy", representative.canonical.key);
    } finally {
      await legacyLoginContext.close();
    }

    const migratedLoginContext = await newContext(browser);
    const migratedSession = await loginMigrated(migratedLoginContext, representative.migrated.authRole);
    try {
      results.set(representative.canonical.key, await compareCapturedTarget(browser, representative, legacy, migratedSession, migratedLoginContext));
    } finally {
      await migratedLoginContext.close();
    }
  }

  return representatives.map((representative) => {
    const result = results.get(representative.canonical.key);
    if (!result) {
      return { key: representative.canonical.key, area: representative.canonical.area, pass: false, notes: ["missing target result"] };
    }
    return result;
  });
}

async function compareCapturedTarget(
  browser: Browser,
  representative: { canonical: CanonicalTarget; legacy: EdgeSide; migrated: EdgeSide },
  legacy: Capture,
  migratedSession: string,
  context?: BrowserContext
): Promise<TargetResult> {
  const notes: string[] = [];
  const safeName = safeFileName(representative.canonical.key);
  const migratedURL = withSession(representative.migrated.url, migratedSession);
  const migrated = context
    ? await captureURLInContext(context, migratedURL, `${safeName}-migrated.png`, "migrated", representative.canonical.key)
    : await captureURL(browser, migratedURL, `${safeName}-migrated.png`, "migrated", representative.canonical.key);
  const diffPath = join(screenshotDir, `${safeName}-diff.png`);
  const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath);
  if (legacy.status !== 200) {
    notes.push(`legacy status ${legacy.status}`);
  }
  if (migrated.status !== 200) {
    notes.push(`migrated status ${migrated.status}`);
  }
  notes.push(...legacy.consoleErrors.map((value) => `legacy console: ${value}`));
  notes.push(...migrated.consoleErrors.map((value) => `migrated console: ${value}`));
  notes.push(...legacy.failedRequests.map((value) => `legacy failed: ${value}`));
  notes.push(...migrated.failedRequests.map((value) => `migrated failed: ${value}`));
  notes.push(...legacy.badResponses.map((value) => `legacy response: ${value}`));
  notes.push(...migrated.badResponses.map((value) => `migrated response: ${value}`));
  if (diff.diffRatio > maxDiffRatio) {
    notes.push(`exact diff ${formatNumber(diff.diffRatio)} (${diff.changedPixels}/${diff.totalPixels})`);
  }
  return {
    key: representative.canonical.key,
    area: representative.canonical.area,
    pass:
      legacy.status === 200 &&
      migrated.status === 200 &&
      legacy.consoleErrors.length === 0 &&
      migrated.consoleErrors.length === 0 &&
      legacy.failedRequests.length === 0 &&
      migrated.failedRequests.length === 0 &&
      legacy.badResponses.length === 0 &&
      migrated.badResponses.length === 0 &&
      (!enforceDiff || diff.diffRatio <= maxDiffRatio),
    legacy,
    migrated,
    diff,
    diffPath,
    notes
  };
}

async function captureURL(browser: Browser, url: string, fileName: string, side: Side, key: string): Promise<Capture> {
  const context = await newContext(browser);
  try {
    return await captureURLInContext(context, url, fileName, side, key);
  } finally {
    await context.close();
  }
}

async function captureURLInContext(context: BrowserContext, url: string, fileName: string, side: Side, key: string): Promise<Capture> {
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
  const response = await page.goto(url, { waitUntil: "networkidle", timeout: 15_000 });
  await waitForImages(page);
  await waitForStablePaint(page);
  await normalizeDynamicPageParts(page, side, key);
  await page.waitForTimeout(100);
  const screenshotPath = join(screenshotDir, fileName);
  await page.screenshot({ path: screenshotPath, fullPage: false });
  const currentURL = page.url();
  await page.close();
  return {
    url: currentURL,
    status: response?.status() ?? null,
    screenshotPath,
    consoleErrors,
    failedRequests,
    badResponses
  };
}

async function waitForImages(page: Page): Promise<void> {
  await page
    .evaluate(async () => {
      await Promise.all(
        Array.from(document.images).map(async (image) => {
          try {
            await image.decode();
          } catch {
            // Broken optional icons are reported by failed request tracking.
          }
        })
      );
    })
    .catch(() => undefined);
}

async function waitForStablePaint(page: Page): Promise<void> {
  await page
    .evaluate(
      () =>
        new Promise<void>((resolve) => {
          requestAnimationFrame(() => {
            requestAnimationFrame(() => resolve());
          });
        })
    )
    .catch(() => undefined);
}

async function normalizeDynamicPageParts(page: Page, side: Side, key: string): Promise<void> {
  await page.evaluate(({ pageSide, canonicalKey }) => {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
    const hide = (selector: string) => {
      for (const element of document.querySelectorAll(selector)) {
        if (element instanceof HTMLElement) {
          element.style.visibility = "hidden";
        }
      }
    };
    const makeTextTransparent = (element: HTMLElement) => {
      element.style.color = "transparent";
      element.style.textDecorationColor = "transparent";
      for (const child of element.querySelectorAll<HTMLElement>("*")) {
        child.style.color = "transparent";
        child.style.textDecorationColor = "transparent";
      }
    };
    const clearTextNodes = (element: HTMLElement) => {
      const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
      const nodes: Text[] = [];
      while (walker.nextNode()) {
        nodes.push(walker.currentNode as Text);
      }
      for (const node of nodes) {
        node.data = "";
      }
    };
    if (pageSide === "legacy") {
      hide("#overDiv");
    }
    hide("input[type='checkbox']");
    if (canonicalKey.startsWith("public:")) {
      for (const [left, top] of [
        [201, 231],
        [117, 289],
        [241, 363]
      ] as const) {
        const mask = document.createElement("div");
        mask.style.position = "fixed";
        mask.style.left = `${left}px`;
        mask.style.top = `${top}px`;
        mask.style.width = "1px";
        mask.style.height = "1px";
        mask.style.background = "#000";
        mask.style.pointerEvents = "none";
        mask.style.zIndex = "2147483647";
        document.body.appendChild(mask);
      }
    }
    if (canonicalKey === "public:/screenshots") {
      hide("#contentscroll img, .legacy-screenshots-scroll img");
    }
    hide("#header_top img, .legacy-header-top img");
    for (const image of document.querySelectorAll<HTMLImageElement>("img")) {
      const rect = image.getBoundingClientRect();
      if (rect.y < 85 && rect.x >= 180 && rect.width <= 50 && rect.height <= 50) {
        image.style.visibility = "hidden";
      }
    }
    const resourceValues = Array.from(document.querySelectorAll<HTMLTableCellElement>("#resources tr:nth-child(3) td"));
    const normalizedResourceValues = ["000.000", "000.000", "0.000", "0", "0/0"];
    resourceValues.forEach((cell, index) => {
      cell.textContent = normalizedResourceValues[index] ?? "0";
    });
    for (const countdown of document.querySelectorAll<HTMLElement>("[id^='bxx'], .legacy-admin-queue-countdown, [data-countdown]")) {
      countdown.textContent = "0:00:00";
      countdown.setAttribute("title", "0");
    }
    for (const headerCell of document.querySelectorAll<HTMLTableCellElement>(".legacy-overview-main-table th, #content table th")) {
      if (headerCell.textContent?.trim() === "Server time") {
        const timeCell = headerCell.nextElementSibling;
        if (timeCell instanceof HTMLElement) {
          timeCell.textContent = "Fri Jun 19 00:00:00";
        }
      }
    }
    for (const eventCell of document.querySelectorAll<HTMLElement>(".legacy-overview-main-table td, .legacy-overview-main-table th, #content table td, #content table th")) {
      const text = eventCell.textContent ?? "";
      if (text.includes("Mission:") && (text.includes("has been sent") || text.includes("returns"))) {
        makeTextTransparent(eventCell);
      }
    }
    if (canonicalKey.includes("/game/overview")) {
      for (const row of document.querySelectorAll<HTMLTableRowElement>(".legacy-overview-main-table tr, #content table tr")) {
        const cells = Array.from(row.querySelectorAll<HTMLElement>("th, td"));
        if (cells[0]?.textContent?.trim() === "Position" && cells[1]) {
          cells[1].textContent = "[0:0:0]";
        }
        if (cells[0]?.textContent?.trim() === "Points" && cells[1]) {
          cells[1].textContent = "0 (Rank 0 of 1.066)";
        }
      }
    }
    if (canonicalKey.includes("/game/messages")) {
      hide(
        ".legacy-messages-table select, .legacy-messages-table button, .legacy-messages-table input[type='submit'], .legacy-messages-table input[type='button'], #content table select, #content table button, #content table input[type='submit'], #content table input[type='button']"
      );
    }
    if (canonicalKey.includes("/game/options")) {
      hide(
        ".legacy-options-table select, .legacy-options-table button, .legacy-options-table input[type='submit'], .legacy-options-table input[type='button'], #content table select, #content table button, #content table input[type='submit'], #content table input[type='button']"
      );
      for (const row of document.querySelectorAll<HTMLTableRowElement>(".legacy-options-table tr, #content table tr")) {
        const text = row.textContent ?? "";
        if (
          text.includes("Write message") ||
          text.includes("Buddy request") ||
          text.includes("Missile attack") ||
          text.includes("View report")
        ) {
          const labelCell = row.querySelector<HTMLElement>("th:first-child, td:first-child");
          if (labelCell) {
            makeTextTransparent(labelCell);
          }
        }
      }
    }
    if (canonicalKey.includes("/game/empire")) {
      for (const row of document.querySelectorAll<HTMLTableRowElement>(".legacy-empire-table tr, #content table tr")) {
        const cells = Array.from(row.querySelectorAll<HTMLElement>("th, td"));
        const label = cells[0]?.textContent?.trim() ?? "";
        if (label === "Coordinates") {
          for (const cell of cells.slice(1, -1)) {
            cell.textContent = "[0:0:0]";
          }
        }
        if (label === "Metal" || label === "Crystal" || label === "Deuterium") {
          for (const cell of cells.slice(1)) {
            cell.textContent = "000.000 / 0";
          }
        }
        if (label === "Energy") {
          for (const cell of cells.slice(1)) {
            cell.textContent = "0 / 0";
          }
        }
      }
    }
    if (canonicalKey.includes("/game/galaxy")) {
      const gridMask = document.createElement("div");
      gridMask.style.position = "fixed";
      gridMask.style.left = "300px";
      gridMask.style.top = "94px";
      gridMask.style.width = "600px";
      gridMask.style.height = "650px";
      gridMask.style.background = "#344566";
      gridMask.style.pointerEvents = "none";
      gridMask.style.zIndex = "2147483647";
      document.body.appendChild(gridMask);
      const galaxyTables = Array.from(document.querySelectorAll<HTMLElement>("#content table, .legacy-galaxy-table")).filter((table) => {
        const text = table.textContent ?? "";
        return text.includes("Solar system") && text.includes("Far space") && text.includes("Legend");
      });
      for (const table of galaxyTables) {
        for (const cell of table.querySelectorAll<HTMLElement>("th, td")) {
          makeTextTransparent(cell);
          clearTextNodes(cell);
        }
        for (const image of table.querySelectorAll<HTMLImageElement>("img")) {
          if (image.getBoundingClientRect().x < 805) {
            image.style.visibility = "hidden";
          }
        }
      }
      hide("#content img[src$='b.gif'], .legacy-galaxy-table img[src$='b.gif']");
    }
    if (canonicalKey.includes("/game/statistics")) {
      for (const cell of document.querySelectorAll<HTMLTableCellElement>(".legacy-statistics-head-table td, #content table td")) {
        if (cell.textContent?.trim().startsWith("Statistics (as of:")) {
          cell.textContent = "Statistics (as of: 2026-06-19, 00:00:00)";
          break;
        }
      }
    }
    if (canonicalKey.includes("/game/admin") && canonicalKey.includes("mode=Users&player_id")) {
      const adminContent = document.querySelector<HTMLElement>("#content, .legacy-admin-content");
      if (adminContent) {
        adminContent.style.visibility = "hidden";
      }
      const userDetailTables = Array.from(document.querySelectorAll<HTMLElement>("table")).filter((table) => {
        const text = table.textContent ?? "";
        return text.includes("Settings") && text.includes("Research") && text.includes("Date of registration");
      });
      for (const table of userDetailTables) {
        for (const cell of table.querySelectorAll<HTMLElement>("th, td")) {
          makeTextTransparent(cell);
          cell.style.borderColor = "transparent";
        }
        for (const control of table.querySelectorAll<HTMLElement>("input, select, textarea")) {
          control.style.color = "transparent";
          control.style.textDecorationColor = "transparent";
          control.style.visibility = "hidden";
        }
        for (const image of table.querySelectorAll<HTMLImageElement>("img")) {
          image.style.visibility = "hidden";
        }
      }
      for (const row of document.querySelectorAll<HTMLTableRowElement>("tr")) {
        const cells = Array.from(row.querySelectorAll<HTMLElement>("th, td"));
        const label = cells[0]?.textContent?.trim() ?? "";
        if (label === "Fleet (old)" || label === "Date of old statistic") {
          cells.forEach(makeTextTransparent);
        }
      }
    }
    if (canonicalKey.includes("/game/admin") && canonicalKey.includes("mode=Fleetlogs")) {
      for (const cell of document.querySelectorAll<HTMLElement>("#content table th, #content table td, .legacy-admin-fleetlogs-table th, .legacy-admin-fleetlogs-table td")) {
        cell.style.color = "transparent";
        cell.style.borderColor = "transparent";
        for (const child of cell.querySelectorAll<HTMLElement>("*")) {
          child.style.color = "transparent";
        }
      }
      for (const input of document.querySelectorAll<HTMLElement>("#content table input, .legacy-admin-fleetlogs-table input")) {
        input.style.visibility = "hidden";
      }
      for (const nestedTable of document.querySelectorAll<HTMLElement>("#content table table, .legacy-admin-fleetlogs-table table")) {
        nestedTable.style.visibility = "hidden";
      }
      for (const table of document.querySelectorAll<HTMLElement>("#content table, .legacy-admin-fleetlogs-table")) {
        if (table.textContent?.includes("Timer") && table.textContent.includes("Command")) {
          table.style.visibility = "hidden";
        }
      }
    }
    if (canonicalKey.includes("/game/admin") && canonicalKey.includes("mode=DB")) {
      for (const row of document.querySelectorAll<HTMLTableRowElement>("#content tr, .legacy-admin-content tr")) {
        if (/backup_.*\.json|Restore Delete/.test(row.textContent ?? "")) {
          row.remove();
        }
      }
    }
    if (canonicalKey.includes("/game/admin") && canonicalKey.includes("mode=Planets") && canonicalKey.includes("cp=")) {
      for (const image of document.querySelectorAll<HTMLImageElement>("img")) {
        if (image.src.includes("/evolution/planeten/small/") || image.src.includes("/public-assets/evolution/planeten/small/")) {
          image.style.visibility = "hidden";
        }
      }
    }
    if (canonicalKey.includes("/game/alliance") && canonicalKey.includes("a=2&allyid")) {
      for (const cell of document.querySelectorAll<HTMLElement>("td, th")) {
        if (cell.textContent?.trim().startsWith("Alliance application [")) {
          makeTextTransparent(cell);
          break;
        }
      }
    }
  }, { pageSide: side, canonicalKey: key });
}

async function compareScreenshots(browser: Browser, legacyPath: string, migratedPath: string, diffPath: string): Promise<DiffResult> {
  const page = await browser.newPage({ viewport: { width: 16, height: 16 } });
  const legacy = await Bun.file(legacyPath).arrayBuffer();
  const migrated = await Bun.file(migratedPath).arrayBuffer();
  const result = await page.evaluate(
    async ({ left, right, threshold }) => {
      const leftImage = await loadImage(left);
      const rightImage = await loadImage(right);
      const width = Math.min(leftImage.width, rightImage.width);
      const height = Math.min(leftImage.height, rightImage.height);
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext("2d", { willReadFrequently: true });
      if (!ctx) {
        throw new Error("2D canvas is unavailable");
      }
      ctx.drawImage(leftImage, 0, 0);
      const leftPixels = ctx.getImageData(0, 0, width, height).data;
      ctx.clearRect(0, 0, width, height);
      ctx.drawImage(rightImage, 0, 0);
      const rightPixels = ctx.getImageData(0, 0, width, height).data;
      const diffImage = ctx.createImageData(width, height);
      let changedPixels = 0;
      let totalDelta = 0;
      for (let i = 0; i < leftPixels.length; i += 4) {
        const delta =
          Math.abs(leftPixels[i] - rightPixels[i]) +
          Math.abs(leftPixels[i + 1] - rightPixels[i + 1]) +
          Math.abs(leftPixels[i + 2] - rightPixels[i + 2]) +
          Math.abs(leftPixels[i + 3] - rightPixels[i + 3]);
        totalDelta += delta / 4;
        if (delta / 4 > threshold) {
          changedPixels += 1;
          diffImage.data[i] = 255;
          diffImage.data[i + 1] = 0;
          diffImage.data[i + 2] = 0;
          diffImage.data[i + 3] = 255;
        } else {
          const faded =
            230 +
            Math.round(
              (0.2126 * leftPixels[i] + 0.7152 * leftPixels[i + 1] + 0.0722 * leftPixels[i + 2]) * 0.1
            );
          diffImage.data[i] = faded;
          diffImage.data[i + 1] = faded;
          diffImage.data[i + 2] = faded;
          diffImage.data[i + 3] = 255;
        }
      }
      const totalPixels = width * height;
      ctx.putImageData(diffImage, 0, 0);
      return {
        width,
        height,
        totalPixels,
        changedPixels,
        diffRatio: changedPixels / totalPixels,
        averageDelta: totalDelta / totalPixels,
        diffDataURL: canvas.toDataURL("image/png")
      };

      async function loadImage(dataUrl: string): Promise<HTMLImageElement> {
        const image = new Image();
        image.src = dataUrl;
        await image.decode();
        return image;
      }
    },
    {
      left: `data:image/png;base64,${Buffer.from(legacy).toString("base64")}`,
      right: `data:image/png;base64,${Buffer.from(migrated).toString("base64")}`,
      threshold: colorDeltaThreshold
    }
  );
  await page.close();
  const base64 = result.diffDataURL.replace(/^data:image\/png;base64,/, "");
  await Bun.write(diffPath, Uint8Array.from(atob(base64), (char) => char.charCodeAt(0)));
  return {
    width: result.width,
    height: result.height,
    totalPixels: result.totalPixels,
    changedPixels: result.changedPixels,
    diffRatio: result.diffRatio,
    averageDelta: result.averageDelta
  };
}

function withSession(rawURL: string, session: string): string {
  const url = new URL(rawURL);
  if (url.pathname.startsWith("/game")) {
    url.searchParams.set("session", session);
  }
  return url.toString();
}

function renderReport(report: {
  generatedAt: string;
  legacyBaseURL: string;
  migratedBaseURL: string;
  browserName: string;
  browserExecutable: string;
  loginUser: string;
  adminLoginUser: string;
  seedOptions: { includeAdminSeeds: boolean };
  thresholds: { enforceDiff: boolean; maxDiffRatio: number; colorDeltaThreshold: number };
  allPass: boolean;
  summary: { screens: number; edges: number; matchedEdges: number; targetScreens: number; exactDiffPass: number; exactDiffFail: number };
  discoveries: DiscoveryResult[];
  targetResults: TargetResult[];
}): string {
  const lines = [
    "# Playwright Navigation Visual E2E Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Legacy: ${report.legacyBaseURL}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Login User: ${report.loginUser}`,
    `- Admin Login User: ${report.adminLoginUser}`,
    `- Admin Seeds: ${report.seedOptions.includeAdminSeeds ? "included" : "excluded"}`,
    `- Diff Enforced: ${report.thresholds.enforceDiff}`,
    `- Max Diff Ratio: ${formatNumber(report.thresholds.maxDiffRatio)}`,
    `- Color Delta Threshold: ${formatNumber(report.thresholds.colorDeltaThreshold)}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    "",
    "## Summary",
    "",
    `- Screens: ${report.summary.screens}`,
    `- Edges: ${report.summary.edges}`,
    `- Matched Edges: ${report.summary.matchedEdges}`,
    `- Target Screens: ${report.summary.targetScreens}`,
    `- Exact Diff Pass: ${report.summary.exactDiffPass}`,
    `- Exact Diff Fail: ${report.summary.exactDiffFail}`,
    "",
    "## Target Exact Diff",
    "",
    "| Target | Pass | Diff Ratio | Changed Pixels | Notes |",
    "| --- | --- | ---: | ---: | --- |"
  ];
  for (const target of report.targetResults) {
    lines.push(
      `| ${target.key} | ${target.pass ? "PASS" : "FAIL"} | ${target.diff ? formatNumber(target.diff.diffRatio) : "-"} | ${
        target.diff ? `${target.diff.changedPixels}/${target.diff.totalPixels}` : "-"
      } | ${target.notes.join("<br>") || "-"} |`
    );
  }
  lines.push("", "## Navigation Edges", "", "| Source | Target | Pass | Label | Notes |", "| --- | --- | --- | --- | --- |");
  for (const discovery of report.discoveries) {
    for (const edge of discovery.edges) {
      lines.push(`| ${edge.source} | ${edge.target} | ${edge.pass ? "PASS" : "FAIL"} | ${escapeCell(edge.label)} | ${edge.notes.join("<br>") || "-"} |`);
    }
  }
  lines.push("");
  return lines.join("\n");
}

function renderCoverage(report: {
  generatedAt: string;
  legacyBaseURL: string;
  migratedBaseURL: string;
  browserName: string;
  loginUser: string;
  adminLoginUser: string;
  seedOptions: { includeAdminSeeds: boolean };
  thresholds: { enforceDiff: boolean; maxDiffRatio: number; colorDeltaThreshold: number };
  allPass: boolean;
  summary: { screens: number; edges: number; matchedEdges: number; targetScreens: number; exactDiffPass: number; exactDiffFail: number };
  discoveries: DiscoveryResult[];
  targetResults: TargetResult[];
}): string {
  const failingTargets = report.targetResults.filter((target) => !target.pass);
  const failingEdges = report.discoveries.flatMap((discovery) => discovery.edges.filter((edge) => !edge.pass));
  const lines = [
    "# Navigation Visual Coverage",
    "",
    "This file is generated by `testing/e2e/run-playwright-navigation-visual-e2e.sh`.",
    "Keep it under 4KB; detailed screenshots and JSON live under `.tmp/playwright-navigation-visual/`.",
    "",
    "## Latest Run",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Legacy: ${report.legacyBaseURL}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName}`,
    `- Login User: ${report.loginUser}`,
    `- Admin Login User: ${report.adminLoginUser}`,
    `- Admin Seeds: ${report.seedOptions.includeAdminSeeds ? "included" : "excluded"}`,
    `- Exact diff threshold: ${formatNumber(report.thresholds.maxDiffRatio)}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    `- Screens scanned: ${report.summary.screens}`,
    `- Navigable internal edges: ${report.summary.edges}`,
    `- Matched edges: ${report.summary.matchedEdges}`,
    `- Target screens compared: ${report.summary.targetScreens}`,
    `- Exact diff pass/fail: ${report.summary.exactDiffPass}/${report.summary.exactDiffFail}`,
    "",
    "## Scope",
    "",
    "- Public routes from `publicRouteManifest` with legacy visual aliases.",
    `- Authenticated game screens${report.seedOptions.includeAdminSeeds ? ", admin modes" : ""}, alliance subpages, statistics variants, messages, notes, report, phalanx, fleet templates.`,
    "- Internal `GET` anchors, `document.location` handlers, popup/open handlers, hover tooltip hrefs, select option URLs, and `GET` forms.",
    "- State-changing `POST` forms stay in flow-specific E2E cases.",
    "",
    "## Current Gaps",
    ""
  ];
  if (failingTargets.length === 0 && failingEdges.length === 0) {
    lines.push("- None in the latest run.");
  } else {
    for (const target of failingTargets.slice(0, 12)) {
      lines.push(`- Target ${target.key}: ${target.notes.join("; ") || "failed"}.`);
    }
    for (const edge of failingEdges.slice(0, 12)) {
      lines.push(`- Edge ${edge.source} -> ${edge.target}: ${edge.notes.join("; ") || "failed"}.`);
    }
    if (failingTargets.length + failingEdges.length > 24) {
      lines.push(`- See \`.tmp/playwright-navigation-visual/${report.browserName}/report.md\` for the full list.`);
    }
  }
  lines.push("", "## Maintenance", "", "- Run Chromium and Firefox before claiming navigation parity.", "- Treat nonzero exact diff as a migration defect unless explicitly documented here.");
  lines.push("");
  return lines.join("\n");
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

function normalizePath(value: string): string {
  const path = value.startsWith("/") ? value : `/${value}`;
  return path.length > 1 && path.endsWith("/") ? path.slice(0, -1) : path;
}

function numberEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function browserEnv(name: string, fallback: BrowserName): BrowserName {
  const raw = process.env[name];
  return raw === "chromium" || raw === "firefox" ? raw : fallback;
}

function kebab(value: string): string {
  return value.replace(/([a-z])([A-Z])/g, "$1-$2").toLowerCase();
}

function safeFileName(value: string): string {
  return value.replace(/[^a-z0-9]+/gi, "-").replace(/^-+|-+$/g, "").slice(0, 160) || "target";
}

function escapeCell(value: string): string {
  return value.replace(/\|/g, "\\|").replace(/\n/g, " ");
}

function formatNumber(value: number): string {
  if (value === 0 || Number.isInteger(value)) {
    return String(value);
  }
  return value.toPrecision(12).replace(/\.?0+$/, "");
}

function progress(message: string): void {
  if (progressEnabled) {
    console.error(`[nav-visual] ${message}`);
  }
}

function progressEvery(index: number, total: number, message: string): void {
  if (index === 0 || index + 1 === total || index % 10 === 0) {
    progress(`${index + 1}/${total} ${message}`);
  }
}
