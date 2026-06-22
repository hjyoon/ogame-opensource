export type GameRouteKey =
  | "overview"
  | "renamePlanet"
  | "admin"
  | "empire"
  | "buildings"
  | "resources"
  | "merchant"
  | "research"
  | "shipyard"
  | "fleet"
  | "fleetTemplates"
  | "technology"
  | "galaxy"
  | "defense"
  | "alliance"
  | "officers"
  | "statistics"
  | "search"
  | "messages"
  | "report"
  | "notes"
  | "buddy"
  | "options"
  | "logout";

export type GameRoute = {
  key: GameRouteKey;
  label: string;
  path: string;
  migrated: boolean;
};

export const gameRoutes: GameRoute[] = [
  { key: "overview", label: "Overview", path: "/game/overview", migrated: true },
  { key: "renamePlanet", label: "Planet menu", path: "/game/rename-planet", migrated: true },
  { key: "admin", label: "*Admin Area*", path: "/game/admin", migrated: false },
  { key: "empire", label: "Empire", path: "/game/empire", migrated: true },
  { key: "buildings", label: "Buildings", path: "/game/buildings", migrated: true },
  { key: "resources", label: "Resources", path: "/game/resources", migrated: true },
  { key: "merchant", label: "Merchant", path: "/game/merchant", migrated: true },
  { key: "research", label: "Research", path: "/game/research", migrated: true },
  { key: "shipyard", label: "Shipyard", path: "/game/shipyard", migrated: true },
  { key: "fleet", label: "Fleet", path: "/game/fleet", migrated: true },
  { key: "fleetTemplates", label: "Standard fleets", path: "/game/fleet-templates", migrated: true },
  { key: "technology", label: "Technology", path: "/game/technology", migrated: true },
  { key: "galaxy", label: "Galaxy", path: "/game/galaxy", migrated: true },
  { key: "defense", label: "Defense", path: "/game/defense", migrated: true },
  { key: "alliance", label: "Alliance", path: "/game/alliance", migrated: false },
  { key: "officers", label: "Officers Recruitment", path: "/game/officers", migrated: false },
  { key: "statistics", label: "Statistics", path: "/game/statistics", migrated: true },
  { key: "search", label: "Search", path: "/game/search", migrated: true },
  { key: "messages", label: "Messages", path: "/game/messages", migrated: true },
  { key: "report", label: "Report", path: "/game/report", migrated: true },
  { key: "notes", label: "Notes", path: "/game/notes", migrated: true },
  { key: "buddy", label: "Buddylist", path: "/game/buddy", migrated: true },
  { key: "options", label: "Options", path: "/game/options", migrated: true },
  { key: "logout", label: "Logout", path: "/game/logout", migrated: true }
];

const overviewRoute = gameRoutes[0];
const routeByPath = new Map(gameRoutes.map((route) => [route.path, route]));
const legacyPageAliases = new Map<string, string>([
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
  ["notes", "/game/notes"],
  ["notizen", "/game/notes"],
  ["buddy", "/game/buddy"],
  ["options", "/game/options"],
  ["logout", "/game/logout"]
]);

export function normalizeGamePath(pathname: string, search = ""): string {
  const [pathOnly] = pathname.split("?");
  const query = pathname.includes("?") && search === "" ? pathname.slice(pathname.indexOf("?")) : search;
  const normalized = pathOnly.length > 1 && pathOnly.endsWith("/") ? pathOnly.slice(0, -1) : pathOnly;
  if (normalized === "/game") {
    return "/game/overview";
  }
  if (normalized === "/game/index.php") {
    const params = new URLSearchParams(query);
    const page = params.get("page") ?? "";
    const mode = params.get("mode") ?? "";
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
    return legacyPageAliases.get(page) ?? "/game/overview";
  }
  return normalized;
}

export function resolveGameRoute(pathname: string, search = ""): GameRoute {
  return routeByPath.get(normalizeGamePath(pathname, search)) ?? overviewRoute;
}

export function gameRouteURL(path: string, search: string): string {
  const query = new URLSearchParams(search);
  query.delete("lgn");
  const encoded = query.toString();
  return encoded ? `${path}?${encoded}` : path;
}

export function gamePlanetSwitchURL(pathname: string, search: string, planetID: number | string): string {
  const query = new URLSearchParams(search);
  query.set("cp", String(planetID));
  return gameRouteURL(resolveGameRoute(pathname, search).path, query.toString());
}

export type GameFleetTargetLink = {
  galaxy: number;
  system: number;
  position: number;
  mission: number;
  planetType?: number;
};

export type GameFleetTargetPrefill = {
  targetGalaxy: number;
  targetSystem: number;
  targetPlanet: number;
  targetPlanetType: number;
  targetMission: number;
};

export function gameFleetTargetURL(target: GameFleetTargetLink, search: string): string {
  const query = new URLSearchParams(search);
  query.set("galaxy", String(target.galaxy));
  query.set("system", String(target.system));
  query.set("position", String(target.position));
  query.set("planet", String(target.position));
  query.set("planettype", String(target.planetType ?? 1));
  query.set("target_mission", String(target.mission));
  return gameRouteURL("/game/fleet", query.toString());
}

export function gameFleetTargetPrefillFromSearch(search: string): GameFleetTargetPrefill | null {
  const query = new URLSearchParams(search);
  const galaxy = parseLegacyFleetTargetInt(query.get("galaxy"));
  if (galaxy === null) {
    return null;
  }
  return {
    targetGalaxy: galaxy,
    targetSystem: parseLegacyFleetTargetInt(query.get("system")) ?? 0,
    targetPlanet: parseLegacyFleetTargetInt(query.get("planet") ?? query.get("position")) ?? 0,
    targetPlanetType: parseLegacyFleetTargetInt(query.get("planettype")) ?? 0,
    targetMission: parseLegacyFleetTargetInt(query.get("target_mission")) ?? 0
  };
}

export function gameBuddyRequestURL(playerID: number, search: string): string {
  const query = new URLSearchParams(search);
  query.set("action", "7");
  query.set("buddy_id", String(playerID));
  return gameRouteURL("/game/buddy", query.toString());
}

export function gameMessageComposeURL(playerID: number, search: string): string {
  const query = new URLSearchParams(search);
  query.set("messageziel", String(playerID));
  return gameRouteURL("/game/messages", query.toString());
}

export function gameGalaxyMissileURL(coordinates: { galaxy: number; system: number; position: number }, planetID: number, playerID: number, search: string): string {
  const query = new URLSearchParams(search);
  query.set("mode", "1");
  query.set("p1", String(coordinates.galaxy));
  query.set("p2", String(coordinates.system));
  query.set("p3", String(coordinates.position));
  query.set("pdd", String(planetID));
  query.set("zp", String(playerID));
  query.set("galaxy", String(coordinates.galaxy));
  query.set("system", String(coordinates.system));
  query.set("position", String(coordinates.position));
  return gameRouteURL("/game/galaxy", query.toString());
}

function parseLegacyFleetTargetInt(value: string | null): number | null {
  if (value === null || value.trim() === "") {
    return null;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return null;
  }
  return Math.trunc(Math.abs(parsed));
}
