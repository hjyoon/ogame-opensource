import { chromium, firefox, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

type BrowserName = "chromium" | "firefox";

type ViewportSpec = {
  name: string;
  width: number;
  height: number;
};

type AuthPageSpec = {
  name: string;
  defaultEnabled?: boolean;
  legacyPage: string;
  legacyQuery?: Record<string, string>;
  migratedPath: string;
  migratedQuery?: Record<string, string>;
  legacyReady: string;
  migratedReady: string;
  requiredBoxes?: LayoutBoxName[];
  expectedTexts: string[];
};

type LayoutBoxName = "header" | "menu" | "content";

type Box = {
  x: number;
  y: number;
  width: number;
  height: number;
};

type DiffResult = {
  width: number;
  height: number;
  totalPixels: number;
  changedPixels: number;
  diffRatio: number;
  averageDelta: number;
};

type PageCapture = {
  status: number | null;
  url: string;
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  boxes: Record<string, Box | null>;
  textChecks: Record<string, boolean>;
  screenshotPath: string;
};

type CaseResult = {
  page: string;
  viewport: string;
  pass: boolean;
  parityPass: boolean;
  legacy: PageCapture;
  migrated: PageCapture;
  diff: DiffResult;
  diffPath: string;
  boxMaxDelta: number;
  diffEnforced: boolean;
  layoutEnforced: boolean;
  notes: string[];
};

const rootDir = resolve(import.meta.dir, "../..");
const browserName = browserEnv("OGAME_PLAYWRIGHT_BROWSER", "chromium");
const outputDir = authVisualOutputDir(process.env.OGAME_AUTH_VISUAL_OUTPUT_DIR, browserName);
const screenshotDir = join(outputDir, "screenshots");
const legacyBaseURL = trimTrailingSlash(process.env.OGAME_LEGACY_BASE_URL ?? "http://127.0.0.1:8888");
const migratedBaseURL = trimTrailingSlash(process.env.OGAME_GO_BASE_URL ?? "http://127.0.0.1:8890");
const loginUser = process.env.OGAME_AUTH_VISUAL_USER ?? "legor";
const loginPassword = process.env.OGAME_AUTH_VISUAL_PASS ?? "admin";
const defaultChromeExecutable = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const defaultBrowserExecutable = browserName === "firefox" ? undefined : defaultChromeExecutable;
const browserExecutable =
  process.env.OGAME_PLAYWRIGHT_EXECUTABLE ??
  (defaultBrowserExecutable && existsSync(defaultBrowserExecutable) ? defaultBrowserExecutable : undefined);
const diffEnforced = process.env.OGAME_AUTH_VISUAL_ENFORCE_DIFF === "1";
const layoutEnforced = process.env.OGAME_AUTH_VISUAL_ENFORCE_LAYOUT === "1";
const maxDiffRatio = numberEnv("OGAME_AUTH_VISUAL_MAX_DIFF_RATIO", 0);
const maxBoxDelta = numberEnv("OGAME_AUTH_VISUAL_MAX_BOX_DELTA", 0);
const colorDeltaThreshold = numberEnv("OGAME_AUTH_VISUAL_COLOR_DELTA", 0);
const pageFilter = parsePageFilter(process.env.OGAME_AUTH_VISUAL_PAGE ?? process.env.OGAME_AUTH_VISUAL_PAGES ?? "");

const viewports: ViewportSpec[] = [{ name: "desktop", width: 1024, height: 768 }];

const pageSpecs: AuthPageSpec[] = [
  {
    name: "game-overview",
    legacyPage: "overview",
    migratedPath: "/game/overview",
    legacyReady: "#content table",
    migratedReady: ".legacy-overview-table",
    expectedTexts: ["Legor", "Diameter", "Temperature", "Points", "administrator mode"]
  },
  {
    name: "game-admin",
    legacyPage: "admin",
    migratedPath: "/game/admin",
    legacyReady: "#content table.s",
    migratedReady: ".legacy-admin-home-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Fleet Logs", "Browse History", "Users", "Universe Settings", "Expedition Settings", "Modifications"]
  },
  {
    name: "game-admin-bans",
    legacyPage: "admin",
    legacyQuery: { mode: "Bans" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Bans" },
    legacyReady: "#content select[name='searchby']",
    migratedReady: ".legacy-admin-bans-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Find users", "Banned with VM", "Attack bans", "Same IP"]
  },
  {
    name: "game-admin-broadcast",
    legacyPage: "admin",
    legacyQuery: { mode: "Broadcast" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Broadcast" },
    legacyReady: "#content textarea[name='text']",
    migratedReady: ".legacy-admin-broadcast-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["To:", "All", "Players in the top 100", "Subject:"]
  },
  {
    name: "game-admin-reports",
    legacyPage: "admin",
    legacyQuery: { mode: "Reports" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Reports" },
    legacyReady: "#content select[name='deletemessages']",
    migratedReady: ".legacy-admin-reports-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Messages", "Action", "Date", "From", "Recipient", "Subject", "Delete highlighted messages"]
  },
  {
    name: "game-admin-bots",
    legacyPage: "admin",
    legacyQuery: { mode: "Bots" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Bots" },
    legacyReady: "#content input[name='name']",
    migratedReady: ".legacy-admin-bots-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Bot List:", "No bots found", "Add bot:", "Name"]
  },
  {
    name: "game-admin-coupons",
    legacyPage: "admin",
    legacyQuery: { mode: "Coupons" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Coupons" },
    legacyReady: "#content input[name='dm']",
    migratedReady: ".legacy-admin-coupons-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Code", "Dark Matter", "Activated", "Add a single coupon", "Holiday coupons"]
  },
  {
    name: "game-admin-colony-settings",
    legacyPage: "admin",
    legacyQuery: { mode: "ColonySettings" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "ColonySettings" },
    legacyReady: "#content input[name='t1_a']",
    migratedReady: ".legacy-admin-colony-settings-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Colonization settings", "Colonies in positions 1-3", "D = RND(a, b) * c"]
  },
  {
    name: "game-admin-debug",
    legacyPage: "admin",
    legacyQuery: { mode: "Debug" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Debug" },
    legacyReady: "#content input[name='filter']",
    migratedReady: ".legacy-admin-debug-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Messages", "Action", "Date", "From", "Browser", "Debug message filter:"]
  },
  {
    name: "game-admin-errors",
    legacyPage: "admin",
    legacyQuery: { mode: "Errors" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Errors" },
    legacyReady: "#content select[name='deletemessages']",
    migratedReady: ".legacy-admin-errors-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Messages", "Action", "Date", "From", "Browser", "Delete highlighted messages"]
  },
  {
    name: "game-admin-logins",
    legacyPage: "admin",
    legacyQuery: { mode: "Logins" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Logins" },
    legacyReady: "#content input[name='name']",
    migratedReady: ".legacy-admin-logins-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["By user name:", "By User ID:", "By IP address:", "Search"]
  },
  {
    name: "game-admin-userlogs",
    legacyPage: "admin",
    legacyQuery: { mode: "UserLogs" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "UserLogs" },
    legacyReady: "#content select[name='type']",
    migratedReady: ".legacy-admin-userlogs-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Recent actions of the players", "Date", "Player", "Category", "Action"]
  },
  {
    name: "game-admin-browse",
    legacyPage: "admin",
    legacyQuery: { mode: "Browse" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Browse" },
    legacyReady: "#content",
    migratedReady: ".legacy-admin-browse-title",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Recent history of transitions"]
  },
  {
    name: "game-admin-fleetlogs",
    legacyPage: "admin",
    legacyQuery: { mode: "Fleetlogs" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Fleetlogs" },
    legacyReady: "#content",
    migratedReady: ".legacy-admin-fleetlogs-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Timer", "Order", "Sent", "Arriving", "Flight time", "Start", "Target", "Fleet", "Cargo", "Fuel", "ACS", "Command"]
  },
  {
    name: "game-admin-queue",
    legacyPage: "admin",
    legacyQuery: { mode: "Queue" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Queue" },
    legacyReady: "#content input[name='player']",
    migratedReady: ".legacy-admin-queue-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["End time", "Player", "Task type", "Description", "Priority", "Control", "Show player's tasks:"]
  },
  {
    name: "game-admin-users",
    legacyPage: "admin",
    legacyQuery: { mode: "Users" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Users" },
    legacyReady: "#content",
    migratedReady: ".legacy-admin-users-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["New users:", "Date of registration", "Home Planet", "Player Name", "Active in the last 24 hours"]
  },
  {
    name: "game-admin-planets",
    legacyPage: "admin",
    legacyQuery: { mode: "Planets" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Planets" },
    legacyReady: "#content input[name='searchtext']",
    migratedReady: ".legacy-admin-planets-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["New Planets:", "Creation date", "Coordinates", "Planet", "Player", "Search", "Player name", "Planet name", "Ally tag"]
  },
  {
    name: "game-admin-uni",
    legacyPage: "admin",
    legacyQuery: { mode: "Uni" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Uni" },
    legacyReady: "#content input[name='maxusers']",
    migratedReady: ".legacy-admin-universe-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Universe 1 Settings", "Date of opening", "Hack attempt counter", "Number of players", "Maximum number of players", "Game speed", "Fleet speed"]
  },
  {
    name: "game-admin-checksum",
    legacyPage: "admin",
    legacyQuery: { mode: "Checksum" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Checksum" },
    legacyReady: "#content",
    migratedReady: ".legacy-admin-checksum-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Engine", "File path", "Checksum", "Status", "Admin Area", "Game Pages", "Registration System"]
  },
  {
    name: "game-admin-db",
    legacyPage: "admin",
    legacyQuery: { mode: "DB" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "DB" },
    legacyReady: "#content",
    migratedReady: ".legacy-admin-db-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Comparison of tables from install and real database", "Comparison of real database and tables from install", "Database Backup", "File name", "Operation"]
  },
  {
    name: "game-admin-battlesim",
    legacyPage: "admin",
    legacyQuery: { mode: "BattleSim" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BattleSim" },
    legacyReady: "#content textarea[name='battle_source']",
    migratedReady: ".legacy-admin-battlesim-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Attacker", "Defender", "Weapons:", "Shields:", "Armor:", "Fleet", "Settings", "Rapidfire", "ADM_SIM_BATTLE_SOURCE"]
  },
  {
    name: "game-admin-expedition",
    legacyPage: "admin",
    legacyQuery: { mode: "Expedition" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Expedition" },
    legacyReady: "#content input[name='dm_factor']",
    migratedReady: ".legacy-admin-expedition-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Expedition Settings", "The multiplier of Dark Matter found", "Expedition depletion settings", "Number of expeditions", "Expedition Simulator"]
  },
  {
    name: "game-admin-battle-report",
    legacyPage: "admin",
    legacyQuery: { mode: "BattleReport" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BattleReport" },
    legacyReady: "#content",
    migratedReady: ".legacy-admin-battle-report-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Battle report"]
  },
  {
    name: "game-admin-botedit",
    legacyPage: "admin",
    legacyQuery: { mode: "BotEdit" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BotEdit" },
    legacyReady: "#content #strategyId",
    migratedReady: ".legacy-admin-botedit-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Name", "New", "Rename", "Show", "Save", "-- Choose a strategy --", "Load"]
  },
  {
    name: "game-admin-raksim",
    legacyPage: "admin",
    legacyQuery: { mode: "RakSim" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "RakSim" },
    legacyReady: "#content input[name='a_weap']",
    migratedReady: ".legacy-admin-raksim-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Attacker", "Defender", "Weapons:", "Armor:", "Settings", "Defense"]
  },
  {
    name: "game-admin-loca",
    legacyPage: "admin",
    legacyQuery: { mode: "Loca" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Loca" },
    legacyReady: "#content select[name='loca_src']",
    migratedReady: ".legacy-admin-loca-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Compare localization between the specified languages", "Source language:", "Target language:"]
  },
  {
    name: "game-admin-mods",
    legacyPage: "admin",
    legacyQuery: { mode: "Mods" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Mods" },
    legacyReady: "#content .mods-container",
    migratedReady: ".legacy-admin-mods-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["ADM_MODS_HEAD", "ADM_MODS_HEAD_ACITVE", "ADM_MODS_HEAD_AVAILABLE"]
  },
  {
    name: "game-rename-planet",
    legacyPage: "renameplanet",
    migratedPath: "/game/rename-planet",
    legacyReady: "#content table",
    migratedReady: ".legacy-rename-planet-table",
    requiredBoxes: ["content"],
    expectedTexts: [
      "Rename/leave the planet",
      "Planet information",
      "Coordinates",
      "Name",
      "Actions",
      "Rename"
    ]
  },
  {
    name: "game-buildings",
    legacyPage: "b_building",
    migratedPath: "/game/buildings",
    legacyReady: "#content img[src*='gebaeude/1.gif']",
    migratedReady: "[data-building-row='1']",
    expectedTexts: ["Metal Mine", "Crystal Mine", "Deuterium Synthesizer", "Cost:", "Duration:"]
  },
  {
    name: "game-resources",
    legacyPage: "resources",
    migratedPath: "/game/resources",
    legacyReady: "#content form#ressourcen",
    migratedReady: ".legacy-resources-table",
    expectedTexts: ["Production factor:", "Resource settings on planet", "Basic Income", "Storage capacity", "Total per hour:"]
  },
  {
    name: "game-empire-redirect",
    legacyPage: "imperium",
    migratedPath: "/game/empire",
    legacyReady: "#content",
    migratedReady: ".legacy-overview-table",
    expectedTexts: ["Legor", "Diameter", "Temperature", "Points", "administrator mode"]
  },
  {
    name: "game-empire",
    defaultEnabled: false,
    legacyPage: "imperium",
    legacyQuery: { planettype: "1", no_header: "1" },
    migratedPath: "/game/empire",
    migratedQuery: { planettype: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-empire-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Empire Overview", "Name", "Coordinates", "Fields", "Resources", "Buildings", "Research", "Ships", "Defense"]
  },
  {
    name: "game-merchant",
    legacyPage: "trader",
    migratedPath: "/game/merchant",
    legacyReady: "#content table",
    migratedReady: ".legacy-merchant-call-table",
    expectedTexts: ["Merchant", "You want to sell", "Summoning a merchant costs 2500 dark matter"]
  },
  {
    name: "game-alliance",
    legacyPage: "allianzen",
    migratedPath: "/game/alliance",
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-menu-table",
    expectedTexts: ["Alliance", "Start your own alliance", "Search for alliances"]
  },
  {
    name: "game-alliance-create",
    legacyPage: "allianzen",
    legacyQuery: { a: "1" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-create-table",
    expectedTexts: ["Found an alliance", "Alliance abbreviation (3-8 characters)", "Alliance name (3-30 characters)"]
  },
  {
    name: "game-alliance-search",
    legacyPage: "allianzen",
    legacyQuery: { a: "2" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "2" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-search-table",
    expectedTexts: ["Looking for alliances.", "Seek", "Search"]
  },
  {
    name: "game-alliance-owned-home",
    defaultEnabled: false,
    legacyPage: "allianzen",
    migratedPath: "/game/alliance",
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-owned-table",
    expectedTexts: ["Your alliance", "Abbreviation", "Members", "your rank", "Internal Competency"]
  },
  {
    name: "game-alliance-management",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "5" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "5" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-management-table",
    expectedTexts: ["alliance management", "set ranks", "Alliance Members", "Edit text", "Settings"]
  },
  {
    name: "game-alliance-members",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "4" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "4" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-members-table",
    expectedTexts: ["List of members", "Name", "Status", "Points", "Coordinates", "Entry"]
  },
  {
    name: "game-alliance-applications",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { page: "bewerbungen" },
    migratedPath: "/game/alliance",
    migratedQuery: { page: "bewerbungen" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-applications-table",
    expectedTexts: ["Overview of enrollment", "Available", "Applicant", "Application Date"]
  },
  {
    name: "game-alliance-circular",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "17" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "17" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-circular-table",
    expectedTexts: ["Send general message", "Recipient", "All players", "Message text"]
  },
  {
    name: "game-alliance-application-text",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "5", t: "3" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "5", t: "3" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-management-table",
    expectedTexts: ["Edit text", "Application Text", "Sample application"]
  },
  {
    name: "game-alliance-settings",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "11", d: "2" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "11", d: "2" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-management-table",
    expectedTexts: ["Settings", "Homepage", "Alliance Logo", "Applications", "Chapter Name"]
  },
  {
    name: "game-alliance-ranks",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "6" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "6" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-ranks-table",
    expectedTexts: ["Form rights", "Rank name", "Assign new rank", "Explanation of Rights"]
  },
  {
    name: "game-officers",
    legacyPage: "micropayment",
    migratedPath: "/game/officers",
    legacyReady: "#content table",
    migratedReady: ".legacy-officers-table",
    expectedTexts: ["To the wise lord", "Dark Matter", "Officers", "Commander", "Admiral", "1 week for"]
  },
  {
    name: "game-research",
    legacyPage: "buildings",
    legacyQuery: { mode: "Forschung" },
    migratedPath: "/game/research",
    legacyReady: "#content table",
    migratedReady: ".legacy-research-table",
    expectedTexts: ["Description", "Qty.", "Computer Technology", "Energy Technology", "Impulse Drive"]
  },
  {
    name: "game-shipyard",
    legacyPage: "buildings",
    legacyQuery: { mode: "Flotte" },
    migratedPath: "/game/shipyard",
    legacyReady: "#content table",
    migratedReady: ".legacy-shipyard-table",
    expectedTexts: ["Description", "Qty.", "Small Cargo", "Light Fighter", "Solar Satellite"]
  },
  {
    name: "game-fleet",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    expectedTexts: ["Fleets", "Expeditions", "Mission", "Ships (total)", "Please select your ships for this mission:", "Ship Type", "Available"]
  },
  {
    name: "game-galaxy",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Galaxy", "Solar system", "Coord.", "Planet", "Title (activity)", "Moon", "Debris", "Player", "Alliance", "Actions", "Legend"]
  },
  {
    name: "game-technology",
    legacyPage: "techtree",
    migratedPath: "/game/technology",
    legacyReady: "#content table",
    migratedReady: ".legacy-technology-table",
    expectedTexts: ["Buildings", "Requirements", "Metal Mine", "Research", "Ships", "Defense", "Lunar Buildings"]
  },
  {
    name: "game-technology-details",
    legacyPage: "techtreedetails",
    legacyQuery: { tid: "206" },
    migratedPath: "/game/technology",
    migratedQuery: { tid: "206" },
    legacyReady: "#content table",
    migratedReady: ".legacy-technology-details-table",
    expectedTexts: ["Building conditions for", "Cruiser", "Shipyard", "Impulse Drive", "Ion Technology"]
  },
  {
    name: "game-defense",
    legacyPage: "buildings",
    legacyQuery: { mode: "Verteidigung" },
    migratedPath: "/game/defense",
    legacyReady: "#content table",
    migratedReady: ".legacy-defense-table",
    expectedTexts: ["Description", "Qty.", "Rocket Launcher"]
  },
  {
    name: "game-statistics",
    legacyPage: "statistics",
    legacyQuery: { type: "ressources", start: "1" },
    migratedPath: "/game/statistics",
    migratedQuery: { type: "ressources", start: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-statistics-table",
    expectedTexts: ["Statistics", "What kind of", "Player", "Alliance", "Points"]
  },
  {
    name: "game-statistics-alliance",
    legacyPage: "statistics",
    legacyQuery: { who: "ally", type: "ressources", start: "1" },
    migratedPath: "/game/statistics",
    migratedQuery: { who: "ally", type: "ressources", start: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-statistics-table",
    expectedTexts: ["Statistics", "What kind of", "Alliance", "Num.", "Thousand points", "Per person"]
  },
  {
    name: "game-search",
    legacyPage: "suche",
    migratedPath: "/game/search",
    legacyReady: "#content table",
    migratedReady: ".legacy-search-head-table",
    expectedTexts: ["Search Universe", "Player Name", "Planet Name", "Alliance Tag", "Alliance Name", "search"]
  },
  {
    name: "game-messages",
    legacyPage: "messages",
    migratedPath: "/game/messages",
    legacyReady: "#content table",
    migratedReady: ".legacy-messages-table",
    expectedTexts: ["Messages", "Action", "Date", "From", "Subject", "Operators"]
  },
  {
    name: "game-messages-compose",
    legacyPage: "writemessages",
    legacyQuery: { messageziel: "1" },
    migratedPath: "/game/messages",
    migratedQuery: { messageziel: "1" },
    legacyReady: "#content form",
    migratedReady: ".legacy-messages-compose-table",
    expectedTexts: ["Write message", "Recipient", "Subject", "Message(0 / 2000 characters)"]
  },
  {
    name: "game-buddy",
    legacyPage: "buddy",
    migratedPath: "/game/buddy",
    legacyReady: "#content table",
    migratedReady: ".legacy-buddy-table",
    expectedTexts: ["Buddylist", "Request", "Your requests", "Name", "Alliance", "Coords", "Status"]
  },
  {
    name: "game-options",
    legacyPage: "options",
    migratedPath: "/game/options",
    legacyReady: "#content table",
    migratedReady: ".legacy-options-table",
    expectedTexts: ["User Data", "General Options", "Galaxy View Options", "Vacation mode / Delete account"]
  },
  {
    name: "game-notes",
    legacyPage: "notizen",
    migratedPath: "/game/notes",
    legacyReady: "#content table",
    migratedReady: ".legacy-notes-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Notes", "Create a new note", "Date", "Subject", "Size"]
  },
  {
    name: "game-notes-create",
    legacyPage: "notizen",
    legacyQuery: { a: "1" },
    migratedPath: "/game/notes",
    migratedQuery: { a: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-notes-form-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Create note", "Priority", "Important", "Normal", "Unimportant", "Subject", "Notice", "Back"]
  }
];
const selectedPageSpecs = selectPageSpecs(pageSpecs, pageFilter);

await mkdir(screenshotDir, { recursive: true });

const browserType = browserName === "firefox" ? firefox : chromium;
const browser = await browserType.launch({
  ...(browserExecutable ? { executablePath: browserExecutable } : {}),
  headless: true
});

try {
  const results: CaseResult[] = [];
  for (const viewport of viewports) {
    for (const spec of selectedPageSpecs) {
      const legacyContext = await newContext(browser, viewport);
      const legacySession = await loginLegacy(legacyContext);
      const legacy = await capturePage(legacyContext, spec, "legacy", legacyURL(spec, legacySession), viewport);
      await legacyContext.close();

      const migratedContext = await newContext(browser, viewport);
      const migratedSession = await loginMigrated(migratedContext);
      const migrated = await capturePage(migratedContext, spec, "migrated", migratedURL(spec, migratedSession), viewport);
      await migratedContext.close();

      const diffPath = join(screenshotDir, `${spec.name}-${viewport.name}-diff.png`);
      const diff = await compareScreenshots(browser, legacy.screenshotPath, migrated.screenshotPath, diffPath);
      const boxMaxDelta = maxPairBoxDelta(legacy.boxes, migrated.boxes, spec.requiredBoxes);
      const notes = caseNotes(legacy, migrated, diff, boxMaxDelta);
      const parityPass = diff.diffRatio <= maxDiffRatio && boxMaxDelta <= maxBoxDelta;
      const contractPass =
        legacy.status === 200 &&
        migrated.status === 200 &&
        legacy.consoleErrors.length === 0 &&
        migrated.consoleErrors.length === 0 &&
        legacy.failedRequests.length === 0 &&
        migrated.failedRequests.length === 0 &&
        legacy.badResponses.length === 0 &&
        migrated.badResponses.length === 0 &&
        boxesPresent(legacy.boxes, spec.requiredBoxes) &&
        boxesPresent(migrated.boxes, spec.requiredBoxes) &&
        textChecksEquivalent(legacy.textChecks, migrated.textChecks);
      const pass = contractPass && (!diffEnforced || diff.diffRatio <= maxDiffRatio) && (!layoutEnforced || boxMaxDelta <= maxBoxDelta);
      results.push({
        page: spec.name,
        viewport: viewport.name,
        pass,
        parityPass,
        legacy,
        migrated,
        diff,
        diffPath,
        boxMaxDelta,
        diffEnforced,
        layoutEnforced,
        notes
      });
    }
  }

  const report = {
    generatedAt: new Date().toISOString(),
    legacyBaseURL,
    migratedBaseURL,
    browserName,
    browserExecutable: browserExecutable ?? "playwright-default",
    loginUser,
    pageFilter: pageFilter.length > 0 ? pageFilter.join(",") : "all",
    thresholds: {
      diffEnforced,
      maxDiffRatio,
      layoutEnforced,
      maxBoxDelta,
      colorDeltaThreshold
    },
    allPass: results.every((result) => result.pass),
    allParityPass: results.every((result) => result.parityPass),
    results
  };
  await writeFile(join(outputDir, "report.json"), JSON.stringify(report, null, 2));
  await writeFile(join(outputDir, "report.md"), renderMarkdown(report));
  process.stdout.write(JSON.stringify({ allPass: report.allPass, report: join(outputDir, "report.json") }, null, 2) + "\n");
  if (!report.allPass) {
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}

function parsePageFilter(value: string): string[] {
  return value
    .split(",")
    .map((name) => name.trim())
    .filter(Boolean);
}

function authVisualOutputDir(value: string | undefined, browser: string): string {
  if (!value) {
    return resolve(rootDir, ".tmp/playwright-auth-visual", browser);
  }
  return resolve(rootDir, value);
}

function selectPageSpecs(specs: AuthPageSpec[], filter: string[]): AuthPageSpec[] {
  if (filter.length === 0) {
    return specs.filter((spec) => spec.defaultEnabled !== false);
  }
  const selected = specs.filter((spec) => filter.includes(spec.name));
  const selectedNames = new Set(selected.map((spec) => spec.name));
  const missing = filter.filter((name) => !selectedNames.has(name));
  if (missing.length > 0) {
    throw new Error(`unknown auth visual page filter: ${missing.join(", ")}`);
  }
  return selected;
}

function legacyURL(spec: AuthPageSpec, session: string): string {
  const query = new URLSearchParams({ page: spec.legacyPage, session });
  for (const [key, value] of Object.entries(spec.legacyQuery ?? {})) {
    query.set(key, value);
  }
  return `${legacyBaseURL}/game/index.php?${query.toString()}`;
}

function migratedURL(spec: AuthPageSpec, session: string): string {
  const query = new URLSearchParams({ session });
  for (const [key, value] of Object.entries(spec.migratedQuery ?? {})) {
    query.set(key, value);
  }
  return `${migratedBaseURL}${spec.migratedPath}?${query.toString()}`;
}

async function newContext(browser: Browser, viewport: ViewportSpec): Promise<BrowserContext> {
  return await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    deviceScaleFactor: 1,
    locale: "en-US"
  });
}

async function loginLegacy(context: BrowserContext): Promise<string> {
  const page = await context.newPage();
  await page.goto(
    `${legacyBaseURL}/game/reg/login2.php?login=${encodeURIComponent(loginUser)}&pass=${encodeURIComponent(loginPassword)}`,
    { waitUntil: "networkidle", timeout: 15_000 }
  );
  const session = new URL(page.url()).searchParams.get("session") ?? "";
  await page.close();
  if (!session) {
    throw new Error("legacy login did not return a session");
  }
  return session;
}

async function loginMigrated(context: BrowserContext): Promise<string> {
  const page = await context.newPage();
  await page.goto(`${migratedBaseURL}/home`, { waitUntil: "networkidle", timeout: 15_000 });
  const universe = (await page.locator("select[name='universe'] option").nth(1).getAttribute("value")) ?? "http://localhost:8888";
  await page.locator("select[name='universe']").selectOption(universe);
  await page.locator("input[name='login']").fill(loginUser);
  await page.locator("input[name='pass']").fill(loginPassword);
  await page.locator("input.legacy-public-login-button").click();
  await page.waitForFunction(() => window.location.pathname === "/game/overview" && window.location.search.includes("session="), undefined, {
    timeout: 10_000
  });
  await page.locator(".legacy-overview-table").first().waitFor({ timeout: 10_000 });
  const session = new URL(page.url()).searchParams.get("session") ?? "";
  await page.close();
  if (!session) {
    throw new Error("migrated login did not return a session");
  }
  return session;
}

async function capturePage(
  context: BrowserContext,
  spec: AuthPageSpec,
  side: "legacy" | "migrated",
  url: string,
  viewport: ViewportSpec
): Promise<PageCapture> {
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
  await page.locator(side === "legacy" ? spec.legacyReady : spec.migratedReady).first().waitFor({ timeout: 10_000 });
  await page.waitForTimeout(250);
  await normalizeDynamicPageParts(page, side, spec.name);
  await waitForImages(page);

  const boxes = {
    header: await boxFor(page, side === "legacy" ? "#header_top" : ".legacy-header-top"),
    menu: await boxFor(page, side === "legacy" ? "#leftmenu" : ".legacy-leftmenu"),
    content: await boxFor(page, side === "legacy" ? "#content" : ".legacy-content")
  };
  const textChecks = await expectedTextChecks(page, spec.expectedTexts);
  const screenshotPath = join(screenshotDir, `${spec.name}-${viewport.name}-${side}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: false });
  const currentURL = page.url();
  await page.close();

  return {
    status: response?.status() ?? null,
    url: currentURL,
    consoleErrors,
    failedRequests,
    badResponses,
    boxes,
    textChecks,
    screenshotPath
  };
}

async function waitForImages(page: Page): Promise<void> {
  await page.evaluate(async () => {
    await Promise.all(
      Array.from(document.images).map(async (image) => {
        try {
          await image.decode();
        } catch {
          // Broken optional icons are reported separately through failed request tracking.
        }
      })
    );
  });
}

async function normalizeDynamicPageParts(page: Page, side: "legacy" | "migrated", pageName: string): Promise<void> {
  await page.evaluate(({ pageSide, currentPageName }) => {
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
    if (pageSide === "legacy") {
      hide("#overDiv");
    }
    hide("#header_top img[width='50'][height='50'], .legacy-header-top img[width='50'][height='50']");
    const resourceValues = Array.from(document.querySelectorAll<HTMLTableCellElement>("#resources tr:nth-child(3) td"));
    const normalizedResourceValues = ["000.000", "000.000", "0.000", "0", "0/0"];
    resourceValues.forEach((cell, index) => {
      cell.textContent = normalizedResourceValues[index] ?? "0";
    });
    if (currentPageName === "game-overview" || currentPageName === "game-empire-redirect") {
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
    for (const countdown of document.querySelectorAll<HTMLElement>("[id^='bxx'], .legacy-admin-queue-countdown")) {
      countdown.textContent = "0:00:00";
      countdown.setAttribute("title", "0");
    }
    if (currentPageName === "game-empire-redirect") {
      hide("#content img[width='200'][height='200'], .legacy-overview-table img[width='200'][height='200']");
    }
    if (currentPageName === "game-admin-db") {
      for (const row of document.querySelectorAll<HTMLTableRowElement>("#content tr, .legacy-admin-content tr")) {
        if (/backup_.*\.json|Restore Delete/.test(row.textContent ?? "")) {
          row.remove();
        }
      }
    }
    if (currentPageName.startsWith("game-admin-")) {
      for (const image of document.querySelectorAll<HTMLImageElement>("img")) {
        const box = image.getBoundingClientRect();
        if (box.top < 85 && box.left > 150) {
          image.style.visibility = "hidden";
        }
      }
    }
    if (currentPageName === "game-admin-battlesim") {
      hide("#content input, #content select, #content textarea, .legacy-admin-content input, .legacy-admin-content select, .legacy-admin-content textarea");
      for (const cell of document.querySelectorAll<HTMLTableCellElement>("#content td, .legacy-admin-content td")) {
        if (cell.textContent?.trim().startsWith("Slot:")) {
          cell.innerHTML = "&nbsp;";
        }
      }
    }
    if (currentPageName === "game-officers") {
      hide("#content img[src$='DMaterie.jpg'], .legacy-officers-table img[src$='DMaterie.jpg']");
    }
    if (currentPageName === "game-admin-fleetlogs") {
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
    if (currentPageName === "game-galaxy") {
      const galaxyTables = Array.from(document.querySelectorAll<HTMLElement>("#content table, .legacy-galaxy-table")).filter((table) => {
        const text = table.textContent ?? "";
        return text.includes("Solar system") && text.includes("Far space") && text.includes("Legend");
      });
      for (const table of galaxyTables) {
        for (const cell of table.querySelectorAll<HTMLElement>("th, td")) {
          makeTextTransparent(cell);
        }
      }
    }
    if (currentPageName === "game-admin-queue") {
      for (const cell of document.querySelectorAll<HTMLElement>("#content table th, #content table td, .legacy-admin-queue-table th, .legacy-admin-queue-table td")) {
        const text = cell.textContent ?? "";
        if (text.includes("ADM_QUEUE_FROZEN")) {
          cell.textContent = text.replace(/ADM_QUEUE_FROZEN\s+\d+/g, "ADM_QUEUE_FROZEN 000");
        }
      }
    }
    if (currentPageName === "game-statistics" || currentPageName === "game-statistics-alliance") {
      for (const cell of document.querySelectorAll<HTMLTableCellElement>(".legacy-statistics-head-table td, #content table td")) {
        if (cell.textContent?.trim().startsWith("Statistics (as of:")) {
          cell.textContent = "Statistics (as of: 2026-06-19, 00:00:00)";
          break;
        }
      }
    }
    if (currentPageName === "game-alliance-ranks") {
      for (const checkbox of document.querySelectorAll<HTMLInputElement>("#content input[type='checkbox'], .legacy-alliance-ranks-table input[type='checkbox']")) {
        const marker = document.createElement("span");
        marker.setAttribute("data-visual-checkbox", checkbox.checked ? "checked" : "unchecked");
        marker.textContent = checkbox.checked ? "\u2713" : "";
        marker.style.background = checkbox.checked ? "#1a73e8" : "#ffffff";
        marker.style.border = "1px solid #9aa9bd";
        marker.style.boxSizing = "border-box";
        marker.style.color = "#ffffff";
        marker.style.display = "inline-block";
        marker.style.fontFamily = "Arial, sans-serif";
        marker.style.fontSize = "11px";
        marker.style.height = "13px";
        marker.style.lineHeight = "11px";
        marker.style.textAlign = "center";
        marker.style.verticalAlign = "middle";
        marker.style.width = "13px";
        checkbox.replaceWith(marker);
      }
    }
    if (currentPageName === "game-options") {
      for (const checkbox of document.querySelectorAll<HTMLInputElement>("#content input[type='checkbox'], .legacy-options-table input[type='checkbox']")) {
        const marker = document.createElement("span");
        marker.setAttribute("data-visual-checkbox", checkbox.checked ? "checked" : "unchecked");
        marker.textContent = checkbox.checked ? "\u2713" : "";
        marker.style.background = checkbox.checked ? "#1a73e8" : "#ffffff";
        marker.style.border = "1px solid #9aa9bd";
        marker.style.boxSizing = "border-box";
        marker.style.color = "#ffffff";
        marker.style.display = "inline-block";
        marker.style.fontFamily = "Arial, sans-serif";
        marker.style.fontSize = "11px";
        marker.style.height = "13px";
        marker.style.lineHeight = "11px";
        marker.style.margin = getComputedStyle(checkbox).margin;
        marker.style.textAlign = "center";
        marker.style.verticalAlign = "middle";
        marker.style.width = "13px";
        checkbox.replaceWith(marker);
      }
    }
  }, { pageSide: side, currentPageName: pageName });
}

async function expectedTextChecks(page: Page, expectedTexts: string[]): Promise<Record<string, boolean>> {
  return await page.evaluate((texts) => {
    const bodyText = document.body.textContent ?? "";
    return Object.fromEntries(texts.map((text) => [text, bodyText.includes(text)]));
  }, expectedTexts);
}

async function boxFor(page: Page, selector: string): Promise<Box | null> {
  const locator = page.locator(selector).first();
  if ((await locator.count()) === 0) {
    return null;
  }
  const box = await locator.boundingBox();
  return box ? { x: box.x, y: box.y, width: box.width, height: box.height } : null;
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

function boxesPresent(boxes: Record<string, Box | null>, requiredBoxes: LayoutBoxName[] = ["header", "menu", "content"]): boolean {
  return requiredBoxes.every((boxName) => boxes[boxName] !== null);
}

function maxPairBoxDelta(
  left: Record<string, Box | null>,
  right: Record<string, Box | null>,
  requiredBoxes: LayoutBoxName[] = ["header", "menu", "content"]
): number {
  let maxDelta = 0;
  for (const key of requiredBoxes) {
    const leftBox = left[key];
    const rightBox = right[key];
    if (!leftBox || !rightBox) {
      return Number.POSITIVE_INFINITY;
    }
    maxDelta = Math.max(
      maxDelta,
      Math.abs(leftBox.x - rightBox.x),
      Math.abs(leftBox.y - rightBox.y),
      Math.abs(leftBox.width - rightBox.width),
      Math.abs(leftBox.height - rightBox.height)
    );
  }
  return maxDelta;
}

function caseNotes(legacy: PageCapture, migrated: PageCapture, diff: DiffResult, boxMaxDelta: number): string[] {
  return [
    ...legacy.consoleErrors.map((value) => `legacy console: ${value}`),
    ...migrated.consoleErrors.map((value) => `migrated console: ${value}`),
    ...legacy.failedRequests.map((value) => `legacy failed: ${value}`),
    ...migrated.failedRequests.map((value) => `migrated failed: ${value}`),
    ...legacy.badResponses.map((value) => `legacy response: ${value}`),
    ...migrated.badResponses.map((value) => `migrated response: ${value}`),
    ...textCheckMismatches(legacy.textChecks, migrated.textChecks),
    `diff ratio ${formatNumber(diff.diffRatio)}`,
    `box max delta ${formatNumber(boxMaxDelta)}`
  ];
}

function textChecksEquivalent(legacy: Record<string, boolean>, migrated: Record<string, boolean>): boolean {
  return textCheckMismatches(legacy, migrated).length === 0;
}

function textCheckMismatches(legacy: Record<string, boolean>, migrated: Record<string, boolean>): string[] {
  const texts = new Set([...Object.keys(legacy), ...Object.keys(migrated)]);
  return Array.from(texts)
    .filter((text) => legacy[text] !== migrated[text])
    .map((text) => `text parity mismatch: ${text} legacy=${legacy[text] === true} migrated=${migrated[text] === true}`);
}

function renderMarkdown(report: {
  generatedAt: string;
  legacyBaseURL: string;
  migratedBaseURL: string;
  browserName: string;
  browserExecutable: string;
  loginUser: string;
  pageFilter: string;
  thresholds: {
    diffEnforced: boolean;
    maxDiffRatio: number;
    layoutEnforced: boolean;
    maxBoxDelta: number;
    colorDeltaThreshold: number;
  };
  allPass: boolean;
  allParityPass: boolean;
  results: CaseResult[];
}): string {
  const lines = [
    "# Playwright Authenticated Visual E2E Report",
    "",
    `- Generated: ${report.generatedAt}`,
    `- Legacy: ${report.legacyBaseURL}`,
    `- Migrated: ${report.migratedBaseURL}`,
    `- Browser: ${report.browserName} (${report.browserExecutable})`,
    `- Login User: ${report.loginUser}`,
    `- Page Filter: ${report.pageFilter}`,
    `- Diff Enforced: ${report.thresholds.diffEnforced}`,
    `- Max Diff Ratio: ${formatNumber(report.thresholds.maxDiffRatio)}`,
    `- Layout Enforced: ${report.thresholds.layoutEnforced}`,
    `- Max Box Delta: ${formatNumber(report.thresholds.maxBoxDelta)}`,
    `- Color Delta Threshold: ${formatNumber(report.thresholds.colorDeltaThreshold)}`,
    `- Result: ${report.allPass ? "PASS" : "FAIL"}`,
    `- Visual Parity: ${report.allParityPass ? "PASS" : "FAIL"}${report.thresholds.diffEnforced || report.thresholds.layoutEnforced ? "" : " (not enforced)"}`,
    "",
    "| Page | Viewport | Contract | Parity | Diff Ratio | Box Max Delta | Diff Image | Notes |",
    "| --- | --- | --- | --- | ---: | ---: | --- | --- |"
  ];
  for (const result of report.results) {
    lines.push(
      `| ${result.page} | ${result.viewport} | ${result.pass ? "PASS" : "FAIL"} | ${result.parityPass ? "PASS" : "FAIL"} | ${formatNumber(
        result.diff.diffRatio
      )} | ${formatNumber(result.boxMaxDelta)} | ${result.diffPath} | ${result.notes.join("<br>") || "-"} |`
    );
  }
  lines.push("");
  return lines.join("\n");
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
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

function formatNumber(value: number): string {
  if (value === 0 || Number.isInteger(value)) {
    return String(value);
  }
  return value.toPrecision(12).replace(/\.?0+$/, "");
}
