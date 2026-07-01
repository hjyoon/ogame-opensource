export type BrowserName = "chromium" | "firefox";
export type SideName = "legacy" | "migrated";
export type LayoutBoxName = "header" | "menu" | "content";
export type GameVisualArea =
  | "core"
  | "admin"
  | "alliance"
  | "direct"
  | "state"
  | "hover"
  | "popup"
  | "permission";

export type GameVisualAction = {
  type: "hover" | "focus" | "click" | "fill" | "select" | "check" | "uncheck" | "keyboard";
  selector?: string;
  legacySelector?: string;
  migratedSelector?: string;
  value?: string;
  waitForSelector?: string;
  waitMs?: number;
};

export type ViewportSpec = {
  name: string;
  width: number;
  height: number;
};

export type GameVisualScreenSpec = {
  name: string;
  area: GameVisualArea;
  defaultEnabled?: boolean;
  legacyPage: string;
  legacyQuery?: Record<string, string>;
  migratedPath: string;
  migratedQuery?: Record<string, string>;
  legacyReady: string;
  migratedReady: string;
  requiredBoxes?: LayoutBoxName[];
  expectedTexts: string[];
  dynamicSelectors?: string[];
  maskSelectors?: string[];
  actions?: GameVisualAction[];
  viewports?: string[];
  notes?: string[];
};

export const gameVisualViewports: ViewportSpec[] = [
  { name: "desktop", width: 1024, height: 768 },
  { name: "tablet", width: 900, height: 768 },
  { name: "popup", width: 550, height: 280 }
];

export const globalGameVisualMaskSelectors = [
  "#overDiv",
  ".legacy-galaxy-tooltip",
  ".legacy-statistics-tooltip",
  "#header_top img[width='50'][height='50']",
  ".legacy-header-top img[width='50'][height='50']"
];

const adminModeSpecs: Array<{
  mode: string;
  ready: string;
  migratedReady: string;
  expectedTexts: string[];
}> = [
  { mode: "Bans", ready: "#content select[name='searchby']", migratedReady: ".legacy-admin-bans-table", expectedTexts: ["Find users", "Banned with VM", "Attack bans", "Same IP"] },
  { mode: "Broadcast", ready: "#content textarea[name='text']", migratedReady: ".legacy-admin-broadcast-table", expectedTexts: ["To:", "All", "Players in the top 100", "Subject:"] },
  { mode: "Reports", ready: "#content select[name='deletemessages']", migratedReady: ".legacy-admin-reports-table", expectedTexts: ["Messages", "Action", "Date", "From", "Recipient", "Subject", "Delete highlighted messages"] },
  { mode: "Bots", ready: "#content input[name='name']", migratedReady: ".legacy-admin-bots-table", expectedTexts: ["Bot List:", "No bots found", "Add bot:", "Name"] },
  { mode: "Coupons", ready: "#content input[name='dm']", migratedReady: ".legacy-admin-coupons-table", expectedTexts: ["Code", "Dark Matter", "Activated", "Add a single coupon", "Holiday coupons"] },
  { mode: "ColonySettings", ready: "#content input[name='t1_a']", migratedReady: ".legacy-admin-colony-settings-table", expectedTexts: ["Colonization settings", "Colonies in positions 1-3", "D = RND(a, b) * c"] },
  { mode: "Debug", ready: "#content input[name='filter']", migratedReady: ".legacy-admin-debug-table", expectedTexts: ["Messages", "Action", "Date", "From", "Browser", "Debug message filter:"] },
  { mode: "Errors", ready: "#content select[name='deletemessages']", migratedReady: ".legacy-admin-errors-table", expectedTexts: ["Messages", "Action", "Date", "From", "Browser", "Delete highlighted messages"] },
  { mode: "Logins", ready: "#content input[name='name']", migratedReady: ".legacy-admin-logins-table", expectedTexts: ["By user name:", "By User ID:", "By IP address:", "Search"] },
  { mode: "UserLogs", ready: "#content select[name='type']", migratedReady: ".legacy-admin-userlogs-table", expectedTexts: ["Recent actions of the players", "Date", "Player", "Category", "Action"] },
  { mode: "Browse", ready: "#content", migratedReady: ".legacy-admin-browse-title", expectedTexts: ["Recent history of transitions"] },
  { mode: "Fleetlogs", ready: "#content", migratedReady: ".legacy-admin-fleetlogs-table", expectedTexts: ["Timer", "Order", "Sent", "Arriving", "Flight time", "Start", "Target", "Fleet", "Cargo", "Fuel", "ACS", "Command"] },
  { mode: "Queue", ready: "#content input[name='player']", migratedReady: ".legacy-admin-queue-table", expectedTexts: ["End time", "Player", "Task type", "Description", "Priority", "Control", "Show player's tasks:"] },
  { mode: "Users", ready: "#content", migratedReady: ".legacy-admin-users-table", expectedTexts: ["New users:", "Date of registration", "Home Planet", "Player Name", "Active in the last 24 hours"] },
  { mode: "Planets", ready: "#content", migratedReady: ".legacy-admin-planets-detail", expectedTexts: ["Creation date", "Date of removal", "Last activity", "Last state update", "Build Queue", "Diameter", "Fleet", "Defense"] },
  { mode: "Uni", ready: "#content input[name='maxusers']", migratedReady: ".legacy-admin-universe-table", expectedTexts: ["Universe 1 Settings", "Date of opening", "Hack attempt counter", "Number of players", "Maximum number of players", "Game speed", "Fleet speed"] },
  { mode: "Checksum", ready: "#content", migratedReady: ".legacy-admin-checksum-table", expectedTexts: ["Engine", "File path", "Checksum", "Status", "Admin Area", "Game Pages", "Registration System"] },
  { mode: "DB", ready: "#content", migratedReady: ".legacy-admin-db-table", expectedTexts: ["Comparison of tables from install and real database", "Database Backup", "File name", "Operation"] },
  { mode: "BattleSim", ready: "#content textarea[name='battle_source']", migratedReady: ".legacy-admin-battlesim-table", expectedTexts: ["Attacker", "Defender", "Weapons:", "Shields:", "Armor:", "Fleet", "Settings", "Rapidfire", "ADM_SIM_BATTLE_SOURCE"] },
  { mode: "Expedition", ready: "#content input[name='dm_factor']", migratedReady: ".legacy-admin-expedition-table", expectedTexts: ["Expedition Settings", "The multiplier of Dark Matter found", "Expedition depletion settings", "Number of expeditions", "Expedition Simulator"] },
  { mode: "BattleReport", ready: "#content", migratedReady: ".legacy-admin-battle-report-table", expectedTexts: ["Battle report"] },
  { mode: "BotEdit", ready: "#content #strategyId", migratedReady: ".legacy-admin-botedit-table", expectedTexts: ["Name", "New", "Rename", "Show", "Save", "-- Choose a strategy --", "Load"] },
  { mode: "RakSim", ready: "#content input[name='a_weap']", migratedReady: ".legacy-admin-raksim-table", expectedTexts: ["Attacker", "Defender", "Weapons:", "Armor:", "Settings", "Defense"] },
  { mode: "Loca", ready: "#content select[name='loca_src']", migratedReady: ".legacy-admin-loca-table", expectedTexts: ["Compare localization between the specified languages", "Source language:", "Target language:"] },
  { mode: "Mods", ready: "#content .mods-container", migratedReady: ".legacy-admin-mods-table", expectedTexts: ["ADM_MODS_HEAD", "ADM_MODS_HEAD_ACITVE", "ADM_MODS_HEAD_AVAILABLE"] }
];

export const gameVisualScreens: GameVisualScreenSpec[] = [
  {
    name: "game-overview",
    area: "core",
    legacyPage: "overview",
    migratedPath: "/game/overview",
    legacyReady: "#content table",
    migratedReady: ".legacy-overview-table",
    expectedTexts: ["Legor", "Diameter", "Temperature", "Points", "administrator mode"],
    dynamicSelectors: ["#bxx", "[id^='bxx']", ".legacy-overview-main-table"]
  },
  {
    name: "game-rename-planet",
    area: "direct",
    legacyPage: "renameplanet",
    migratedPath: "/game/rename-planet",
    legacyReady: "#content table",
    migratedReady: ".legacy-rename-planet-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Rename/leave the planet", "Planet information", "Coordinates", "Name", "Actions", "Rename"]
  },
  {
    name: "game-buildings",
    area: "core",
    legacyPage: "b_building",
    migratedPath: "/game/buildings",
    legacyReady: "#content img[src*='gebaeude/1.gif']",
    migratedReady: "[data-building-row='1']",
    expectedTexts: ["Metal Mine", "Crystal Mine", "Deuterium Synthesizer", "Cost:", "Duration:"],
    dynamicSelectors: ["#bxx", "[id^='bxx']"]
  },
  {
    name: "game-resources",
    area: "core",
    legacyPage: "resources",
    migratedPath: "/game/resources",
    legacyReady: "#content form#ressourcen",
    migratedReady: ".legacy-resources-table",
    expectedTexts: ["Production factor:", "Resource settings on planet", "Basic Income", "Storage capacity", "Total per hour:"]
  },
  {
    name: "game-resources-production-edited",
    area: "state",
    legacyPage: "resources",
    migratedPath: "/game/resources",
    legacyReady: "#content form#ressourcen",
    migratedReady: ".legacy-resources-table",
    expectedTexts: ["Production factor:", "Resource settings on planet", "Basic Income", "Storage capacity", "Total per hour:"],
    actions: [
      { type: "select", selector: "select[name='last1']", value: "0" },
      { type: "select", selector: "select[name='last2']", value: "50" },
      { type: "select", selector: "select[name='last3']", value: "100" }
    ],
    notes: ["Same-route draft state for edited production percentages without submitting."]
  },
  {
    name: "game-empire-redirect",
    area: "state",
    legacyPage: "imperium",
    migratedPath: "/game/empire",
    legacyReady: "#content",
    migratedReady: ".legacy-overview-table",
    expectedTexts: ["Legor", "Diameter", "Temperature", "Points", "administrator mode"],
    notes: ["Default legor fixture without Commander redirects to overview."]
  },
  {
    name: "game-empire",
    area: "core",
    defaultEnabled: false,
    legacyPage: "imperium",
    legacyQuery: { planettype: "1", no_header: "1" },
    migratedPath: "/game/empire",
    migratedQuery: { planettype: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-empire-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Empire Overview", "Name", "Coordinates", "Fields", "Resources", "Buildings", "Research", "Ships", "Defense"],
    notes: ["Requires a Commander-enabled account/fixture."]
  },
  {
    name: "game-merchant",
    area: "core",
    legacyPage: "trader",
    migratedPath: "/game/merchant",
    legacyReady: "#content table",
    migratedReady: ".legacy-merchant-call-table",
    expectedTexts: ["Merchant", "You want to sell", "Summoning a merchant costs 2500 dark matter"]
  },
  {
    name: "game-research",
    area: "core",
    legacyPage: "buildings",
    legacyQuery: { mode: "Forschung" },
    migratedPath: "/game/research",
    legacyReady: "#content table",
    migratedReady: ".legacy-research-table",
    expectedTexts: ["Description", "Qty.", "Computer Technology", "Energy Technology", "Impulse Drive"],
    dynamicSelectors: ["#bxx", "[id^='bxx']"]
  },
  {
    name: "game-shipyard",
    area: "core",
    legacyPage: "buildings",
    legacyQuery: { mode: "Flotte" },
    migratedPath: "/game/shipyard",
    legacyReady: "#content table",
    migratedReady: ".legacy-shipyard-table",
    expectedTexts: ["Description", "Qty.", "Small Cargo", "Light Fighter", "Solar Satellite"],
    dynamicSelectors: ["#bxx", "[id^='bxx']"]
  },
  {
    name: "game-shipyard-qty-draft",
    area: "state",
    legacyPage: "buildings",
    legacyQuery: { mode: "Flotte" },
    migratedPath: "/game/shipyard",
    legacyReady: "#content table",
    migratedReady: ".legacy-shipyard-table",
    expectedTexts: ["Description", "Qty.", "Small Cargo", "Build"],
    actions: [{ type: "fill", selector: "input[name='fmenge[202]']", value: "3" }],
    dynamicSelectors: ["#bxx", "[id^='bxx']"],
    notes: ["Same-route shipyard quantity draft without submitting the build form."]
  },
  {
    name: "game-fleet",
    area: "core",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    expectedTexts: ["Fleets", "Expeditions", "Mission", "Ships (total)", "Please select your ships for this mission:", "Ship Type", "Available"]
  },
  {
    name: "game-fleet-ship-selection-draft",
    area: "state",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    expectedTexts: ["Fleets", "Expeditions", "Please select your ships for this mission:", "Ship Type", "Available", "continue"],
    actions: [{ type: "fill", selector: "input[name='ship202']", value: "1" }],
    notes: ["Same-route fleet ship selection draft before continue."]
  },
  {
    name: "game-fleet-templates",
    area: "direct",
    defaultEnabled: false,
    legacyPage: "fleet_templates",
    migratedPath: "/game/fleet-templates",
    legacyReady: "#content",
    migratedReady: ".legacy-fleet-templates-table, .legacy-overview-table",
    expectedTexts: ["Standard fleets", "Name", "Actions"],
    notes: ["Requires Commander for the fleet-template table; otherwise parity is a redirect/locked-state check."]
  },
  {
    name: "game-technology",
    area: "core",
    legacyPage: "techtree",
    migratedPath: "/game/technology",
    legacyReady: "#content table",
    migratedReady: ".legacy-technology-table",
    expectedTexts: ["Buildings", "Requirements", "Metal Mine", "Research", "Ships", "Defense", "Lunar Buildings"]
  },
  {
    name: "game-technology-details",
    area: "state",
    legacyPage: "techtreedetails",
    legacyQuery: { tid: "206" },
    migratedPath: "/game/technology",
    migratedQuery: { tid: "206" },
    legacyReady: "#content table",
    migratedReady: ".legacy-technology-details-table",
    expectedTexts: ["Building conditions for", "Cruiser", "Shipyard", "Impulse Drive", "Ion Technology"]
  },
  {
    name: "game-changelog",
    area: "direct",
    defaultEnabled: false,
    legacyPage: "changelog",
    migratedPath: "/game/changelog",
    legacyReady: "#content",
    migratedReady: ".legacy-changelog-table, .legacy-overview-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Changelog"],
    notes: ["Direct version-link screen; disabled by default until fixture text is stable."]
  },
  {
    name: "game-galaxy",
    area: "core",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Galaxy", "Solar system", "Coord.", "Planet", "Title (activity)", "Moon", "Debris", "Player", "Alliance", "Actions", "Legend"],
    dynamicSelectors: [".legacy-galaxy-hover", ".legacy-galaxy-tooltip"],
    maskSelectors: [".legacy-galaxy-tooltip"]
  },
  {
    name: "game-galaxy-hover",
    area: "hover",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Galaxy", "Actions"],
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='Planet Visual Hover Planet']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='planet'] a",
        waitMs: 850
      }
    ],
    notes: ["Explicit galaxy planet overlib tooltip capture."]
  },
  {
    name: "game-galaxy-hover-moon",
    area: "hover",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Galaxy", "Moon", "Properties", "Actions"],
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='Moon Visual Hover Moon']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='moon'] a",
        waitMs: 850
      }
    ],
    notes: ["Explicit galaxy moon overlib tooltip capture."]
  },
  {
    name: "game-galaxy-hover-debris",
    area: "hover",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Galaxy", "Resources:", "Recycle"],
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='Resources:'][onmouseover*='Recycle']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='debris'] a",
        waitMs: 850
      }
    ],
    notes: ["Explicit galaxy debris overlib tooltip capture."]
  },
  {
    name: "game-galaxy-hover-player",
    area: "hover",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Galaxy", "Player Visualhover", "Write a message", "Statistics"],
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='Player Visualhover']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='player'] a",
        waitMs: 850
      }
    ],
    notes: ["Explicit galaxy player overlib tooltip capture."]
  },
  {
    name: "game-galaxy-hover-alliance",
    area: "hover",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Galaxy", "Alliance VGHT", "Alliance introduction", "Statistics"],
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='Alliance VGHT']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='alliance'] a",
        waitMs: 850
      }
    ],
    notes: ["Explicit galaxy alliance overlib tooltip capture."]
  },
  {
    name: "game-defense",
    area: "core",
    legacyPage: "buildings",
    legacyQuery: { mode: "Verteidigung" },
    migratedPath: "/game/defense",
    legacyReady: "#content table",
    migratedReady: ".legacy-defense-table",
    expectedTexts: ["Description", "Qty.", "Rocket Launcher"],
    dynamicSelectors: ["#bxx", "[id^='bxx']"]
  },
  {
    name: "game-defense-qty-draft",
    area: "state",
    legacyPage: "buildings",
    legacyQuery: { mode: "Verteidigung" },
    migratedPath: "/game/defense",
    legacyReady: "#content table",
    migratedReady: ".legacy-defense-table",
    expectedTexts: ["Description", "Qty.", "Rocket Launcher", "Build"],
    actions: [{ type: "fill", selector: "input[name='fmenge[401]']", value: "5" }],
    dynamicSelectors: ["#bxx", "[id^='bxx']"],
    notes: ["Same-route defense quantity draft without submitting the build form."]
  },
  {
    name: "game-alliance",
    area: "core",
    legacyPage: "allianzen",
    migratedPath: "/game/alliance",
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-menu-table",
    expectedTexts: ["Alliance", "Start your own alliance", "Search for alliances"]
  },
  {
    name: "game-alliance-create",
    area: "alliance",
    legacyPage: "allianzen",
    legacyQuery: { a: "1" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-create-table",
    expectedTexts: ["Found an alliance", "Alliance abbreviation (3-8 characters)", "Alliance name (3-30 characters)"]
  },
  {
    name: "game-alliance-create-draft",
    area: "state",
    legacyPage: "allianzen",
    legacyQuery: { a: "1" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-create-table",
    expectedTexts: ["Found an alliance", "Alliance abbreviation (3-8 characters)", "Alliance name (3-30 characters)"],
    actions: [
      { type: "fill", selector: "input[name='tag']", value: "VIS" },
      { type: "fill", selector: "input[name='name']", value: "Visual State Alliance" }
    ],
    notes: ["Same-route alliance creation draft without submitting."]
  },
  {
    name: "game-alliance-search",
    area: "alliance",
    legacyPage: "allianzen",
    legacyQuery: { a: "2" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "2" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-search-table",
    expectedTexts: ["Looking for alliances.", "Seek", "Search"]
  },
  {
    name: "game-alliance-search-draft",
    area: "state",
    legacyPage: "allianzen",
    legacyQuery: { a: "2" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "2" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-search-table",
    expectedTexts: ["Looking for alliances.", "Seek", "Search"],
    actions: [{ type: "fill", selector: "input[name='suchtext']", value: "VIS" }],
    notes: ["Same-route alliance search draft without submitting."]
  },
  {
    name: "game-alliance-owned-home",
    area: "alliance",
    defaultEnabled: false,
    legacyPage: "allianzen",
    migratedPath: "/game/alliance",
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-owned-table",
    expectedTexts: ["Your alliance", "Abbreviation", "Members", "your rank", "Internal Competency"],
    notes: ["Requires an account already in an alliance."]
  },
  {
    name: "game-alliance-management",
    area: "alliance",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "5" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "5" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-management-table",
    expectedTexts: ["alliance management", "set ranks", "Alliance Members", "Edit text", "Settings"],
    notes: ["Requires alliance owner rights."]
  },
  {
    name: "game-alliance-members",
    area: "alliance",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "4" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "4" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-members-table",
    expectedTexts: ["List of members", "Name", "Status", "Points", "Coordinates", "Entry"],
    notes: ["Requires alliance membership."]
  },
  {
    name: "game-alliance-applications",
    area: "alliance",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { page: "bewerbungen" },
    migratedPath: "/game/alliance",
    migratedQuery: { page: "bewerbungen" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-applications-table",
    expectedTexts: ["Overview of enrollment", "Available", "Applicant", "Application Date"],
    notes: ["Requires alliance owner rights and applications fixture."]
  },
  {
    name: "game-alliance-circular",
    area: "alliance",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "17" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "17" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-circular-table",
    expectedTexts: ["Send general message", "Recipient", "All players", "Message text"],
    notes: ["Requires alliance membership."]
  },
  {
    name: "game-alliance-application-text",
    area: "alliance",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "5", t: "3" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "5", t: "3" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-management-table",
    expectedTexts: ["Edit text", "Application Text", "Sample application"],
    notes: ["Requires alliance owner rights."]
  },
  {
    name: "game-alliance-settings",
    area: "alliance",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "11", d: "2" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "11", d: "2" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-management-table",
    expectedTexts: ["Settings", "Homepage", "Alliance Logo", "Applications", "Chapter Name"],
    notes: ["Requires alliance owner rights."]
  },
  {
    name: "game-alliance-ranks",
    area: "alliance",
    defaultEnabled: false,
    legacyPage: "allianzen",
    legacyQuery: { a: "6" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "6" },
    legacyReady: "#content table",
    migratedReady: ".legacy-alliance-ranks-table",
    expectedTexts: ["Form rights", "Rank name", "Assign new rank", "Explanation of Rights"],
    notes: ["Requires alliance owner rights."]
  },
  {
    name: "game-officers",
    area: "core",
    legacyPage: "micropayment",
    migratedPath: "/game/officers",
    legacyReady: "#content table",
    migratedReady: ".legacy-officers-table",
    expectedTexts: ["To the wise lord", "Dark Matter", "Officers", "Commander", "Admiral", "1 week for"]
  },
  {
    name: "game-statistics",
    area: "core",
    legacyPage: "statistics",
    legacyQuery: { type: "ressources", start: "1" },
    migratedPath: "/game/statistics",
    migratedQuery: { type: "ressources", start: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-statistics-table",
    expectedTexts: ["Statistics", "What kind of", "Player", "Alliance", "Points"],
    dynamicSelectors: [".legacy-statistics-tooltip"]
  },
  {
    name: "game-statistics-alliance",
    area: "state",
    legacyPage: "statistics",
    legacyQuery: { who: "ally", type: "ressources", start: "1" },
    migratedPath: "/game/statistics",
    migratedQuery: { who: "ally", type: "ressources", start: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-statistics-table",
    expectedTexts: ["Statistics", "What kind of", "Alliance", "Num.", "Thousand points", "Per person"],
    dynamicSelectors: [".legacy-statistics-tooltip"]
  },
  {
    name: "game-search",
    area: "core",
    legacyPage: "suche",
    migratedPath: "/game/search",
    legacyReady: "#content table",
    migratedReady: ".legacy-search-head-table",
    expectedTexts: ["Search Universe", "Player Name", "Planet Name", "Alliance Tag", "Alliance Name", "search"]
  },
  {
    name: "game-search-form-draft",
    area: "state",
    legacyPage: "suche",
    migratedPath: "/game/search",
    legacyReady: "#content table",
    migratedReady: ".legacy-search-head-table",
    expectedTexts: ["Search Universe", "Player Name", "Planet Name", "Alliance Tag", "Alliance Name", "search"],
    actions: [
      { type: "select", selector: "select[name='type']", value: "playername" },
      { type: "fill", selector: "input[name='searchtext']", value: "Legor" }
    ],
    notes: ["Same-route search form draft without submitting."]
  },
  {
    name: "game-messages",
    area: "core",
    legacyPage: "messages",
    migratedPath: "/game/messages",
    legacyReady: "#content table",
    migratedReady: ".legacy-messages-table",
    expectedTexts: ["Messages", "Action", "Date", "From", "Subject", "Operators"]
  },
  {
    name: "game-messages-compose",
    area: "state",
    legacyPage: "writemessages",
    legacyQuery: { messageziel: "1" },
    migratedPath: "/game/messages",
    migratedQuery: { messageziel: "1" },
    legacyReady: "#content form",
    migratedReady: ".legacy-messages-compose-table",
    expectedTexts: ["Write message", "Recipient", "Subject", "Message(0 / 2000 characters)"]
  },
  {
    name: "game-messages-compose-draft",
    area: "state",
    legacyPage: "writemessages",
    legacyQuery: { messageziel: "1" },
    migratedPath: "/game/messages",
    migratedQuery: { messageziel: "1" },
    legacyReady: "#content form",
    migratedReady: ".legacy-messages-compose-table",
    expectedTexts: ["Write message", "Recipient", "Subject", "Message(0 / 2000 characters)"],
    actions: [
      { type: "fill", selector: "input[name='betreff']", value: "Visual draft" },
      { type: "fill", selector: "textarea[name='text']", value: "Visual regression draft body" }
    ],
    notes: ["Same-route message compose draft without sending."]
  },
  {
    name: "game-report",
    area: "direct",
    defaultEnabled: false,
    legacyPage: "bericht",
    migratedPath: "/game/report",
    legacyReady: "#content",
    migratedReady: ".legacy-report-table, .legacy-overview-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Report"],
    notes: ["Requires a seeded report id via bericht query."]
  },
  {
    name: "game-phalanx",
    area: "popup",
    defaultEnabled: false,
    legacyPage: "phalanx",
    migratedPath: "/game/phalanx",
    legacyReady: "#content",
    migratedReady: ".legacy-phalanx-table, .legacy-overview-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Sensor Phalanx"],
    viewports: ["popup"],
    notes: ["Requires a moon source and target coordinates fixture."]
  },
  {
    name: "game-buddy",
    area: "core",
    legacyPage: "buddy",
    migratedPath: "/game/buddy",
    legacyReady: "#content table",
    migratedReady: ".legacy-buddy-table",
    expectedTexts: ["Buddylist", "Request", "Your requests", "Name", "Alliance", "Coords", "Status"]
  },
  {
    name: "game-options",
    area: "core",
    legacyPage: "options",
    migratedPath: "/game/options",
    legacyReady: "#content table",
    migratedReady: ".legacy-options-table",
    expectedTexts: ["User Data", "General Options", "Galaxy View Options", "Vacation mode / Delete account"]
  },
  {
    name: "game-options-edited-draft",
    area: "state",
    legacyPage: "options",
    migratedPath: "/game/options",
    legacyReady: "#content table",
    migratedReady: ".legacy-options-table",
    expectedTexts: ["User Data", "General Options", "Galaxy View Options", "Vacation mode / Delete account"],
    actions: [
      { type: "select", selector: "select[name='settings_sort']", value: "1" },
      { type: "select", selector: "select[name='settings_order']", value: "1" },
      { type: "fill", selector: "input[name='spio_anz']", value: "6" }
    ],
    notes: ["Same-route options draft without submitting account changes."]
  },
  {
    name: "game-notes",
    area: "core",
    legacyPage: "notizen",
    migratedPath: "/game/notes",
    legacyReady: "#content table",
    migratedReady: ".legacy-notes-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Notes", "Create a new note", "Date", "Subject", "Size"]
  },
  {
    name: "game-notes-create",
    area: "state",
    legacyPage: "notizen",
    legacyQuery: { a: "1" },
    migratedPath: "/game/notes",
    migratedQuery: { a: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-notes-form-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Create note", "Priority", "Important", "Normal", "Unimportant", "Subject", "Notice", "Back"]
  },
  {
    name: "game-notes-create-draft",
    area: "state",
    legacyPage: "notizen",
    legacyQuery: { a: "1" },
    migratedPath: "/game/notes",
    migratedQuery: { a: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-notes-form-table",
    requiredBoxes: ["content"],
    expectedTexts: ["Create note", "Priority", "Important", "Normal", "Unimportant", "Subject", "Notice", "Back"],
    actions: [
      { type: "select", selector: "select[name='u']", value: "0" },
      { type: "fill", selector: "input[name='betreff']", value: "Visual note" },
      { type: "fill", selector: "textarea[name='text']", value: "Visual note draft body" }
    ],
    notes: ["Same-route note draft without saving."]
  },
  {
    name: "game-admin",
    area: "admin",
    legacyPage: "admin",
    migratedPath: "/game/admin",
    legacyReady: "#content table.s",
    migratedReady: ".legacy-admin-home-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Fleet Logs", "Browse History", "Users", "Universe Settings", "Expedition Settings", "Modifications"]
  },
  {
    name: "game-admin-bans-filter-draft",
    area: "state",
    legacyPage: "admin",
    legacyQuery: { mode: "Bans" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Bans" },
    legacyReady: "#content select[name='searchby']",
    migratedReady: ".legacy-admin-bans-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["Find users", "Banned with VM", "Attack bans", "Same IP"],
    actions: [
      { type: "select", selector: "select[name='searchby']", value: "4" },
      { type: "fill", selector: "input[name='text']", value: "Legor" },
      { type: "fill", selector: "input[name='days']", value: "1" },
      { type: "fill", selector: "input[name='hours']", value: "2" },
      { type: "fill", selector: "textarea[name='reason']", value: "Visual draft reason" }
    ],
    notes: ["Same-route admin bans form draft without submitting."]
  },
  {
    name: "game-admin-broadcast-draft",
    area: "state",
    legacyPage: "admin",
    legacyQuery: { mode: "Broadcast" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Broadcast" },
    legacyReady: "#content textarea[name='text']",
    migratedReady: ".legacy-admin-broadcast-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["To:", "All", "Players in the top 100", "Subject:"],
    actions: [
      { type: "select", selector: "select[name='cat']", value: "1" },
      { type: "fill", selector: "input[name='subj']", value: "Visual broadcast" },
      { type: "fill", selector: "textarea[name='text']", value: "Visual broadcast draft body" }
    ],
    notes: ["Same-route admin broadcast draft without sending."]
  },
  {
    name: "game-admin-queue-filter-draft",
    area: "state",
    legacyPage: "admin",
    legacyQuery: { mode: "Queue" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "Queue" },
    legacyReady: "#content input[name='player']",
    migratedReady: ".legacy-admin-queue-table",
    requiredBoxes: ["menu", "content"],
    expectedTexts: ["End time", "Player", "Task type", "Description", "Priority", "Control", "Show player's tasks:"],
    actions: [{ type: "fill", selector: "input[name='player']", value: "Legor" }],
    notes: ["Same-route admin queue filter draft without submitting."]
  },
  ...adminModeSpecs.map((spec): GameVisualScreenSpec => ({
    name: `game-admin-${kebab(spec.mode)}`,
    area: "admin",
    legacyPage: "admin",
    legacyQuery: { mode: spec.mode },
    migratedPath: "/game/admin",
    migratedQuery: { mode: spec.mode },
    legacyReady: spec.ready,
    migratedReady: spec.migratedReady,
    requiredBoxes: ["menu", "content"],
    expectedTexts: spec.expectedTexts
  }))
];

export function selectGameVisualScreens(filterValue: string): GameVisualScreenSpec[] {
  const filter = parseNameFilter(filterValue);
  if (filter.length === 0) {
    return gameVisualScreens.filter((spec) => spec.defaultEnabled !== false);
  }
  const selected = gameVisualScreens.filter((spec) => filter.includes(spec.name) || filter.includes(spec.area));
  const selectedNames = new Set(selected.flatMap((spec) => [spec.name, spec.area]));
  const missing = filter.filter((name) => !selectedNames.has(name));
  if (missing.length > 0) {
    throw new Error(`unknown authenticated game visual filter: ${missing.join(", ")}`);
  }
  return selected;
}

export function selectGameVisualViewports(filterValue: string): ViewportSpec[] {
  const filter = parseNameFilter(filterValue);
  if (filter.length === 0) {
    return gameVisualViewports.filter((viewport) => viewport.name === "desktop");
  }
  const selected = gameVisualViewports.filter((viewport) => filter.includes(viewport.name));
  const selectedNames = new Set(selected.map((viewport) => viewport.name));
  const missing = filter.filter((name) => !selectedNames.has(name));
  if (missing.length > 0) {
    throw new Error(`unknown authenticated game visual viewport filter: ${missing.join(", ")}`);
  }
  return selected;
}

function parseNameFilter(value: string): string[] {
  return value
    .split(",")
    .map((name) => name.trim())
    .filter(Boolean);
}

function kebab(value: string): string {
  return value
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .replace(/[^a-zA-Z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .toLowerCase();
}
