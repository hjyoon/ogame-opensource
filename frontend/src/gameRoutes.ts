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
  | "technology"
  | "galaxy"
  | "defense"
  | "alliance"
  | "officers"
  | "statistics"
  | "search"
  | "messages"
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
  { key: "empire", label: "Empire", path: "/game/empire", migrated: false },
  { key: "buildings", label: "Buildings", path: "/game/buildings", migrated: true },
  { key: "resources", label: "Resources", path: "/game/resources", migrated: true },
  { key: "merchant", label: "Merchant", path: "/game/merchant", migrated: false },
  { key: "research", label: "Research", path: "/game/research", migrated: true },
  { key: "shipyard", label: "Shipyard", path: "/game/shipyard", migrated: true },
  { key: "fleet", label: "Fleet", path: "/game/fleet", migrated: true },
  { key: "technology", label: "Technology", path: "/game/technology", migrated: true },
  { key: "galaxy", label: "Galaxy", path: "/game/galaxy", migrated: true },
  { key: "defense", label: "Defense", path: "/game/defense", migrated: true },
  { key: "alliance", label: "Alliance", path: "/game/alliance", migrated: false },
  { key: "officers", label: "Officers Recruitment", path: "/game/officers", migrated: false },
  { key: "statistics", label: "Statistics", path: "/game/statistics", migrated: true },
  { key: "search", label: "Search", path: "/game/search", migrated: true },
  { key: "messages", label: "Messages", path: "/game/messages", migrated: false },
  { key: "notes", label: "Notes", path: "/game/notes", migrated: true },
  { key: "buddy", label: "Buddylist", path: "/game/buddy", migrated: false },
  { key: "options", label: "Options", path: "/game/options", migrated: false },
  { key: "logout", label: "Logout", path: "/game/logout", migrated: true }
];

const overviewRoute = gameRoutes[0];
const routeByPath = new Map(gameRoutes.map((route) => [route.path, route]));

export function normalizeGamePath(pathname: string): string {
  const [pathOnly] = pathname.split("?");
  const normalized = pathOnly.length > 1 && pathOnly.endsWith("/") ? pathOnly.slice(0, -1) : pathOnly;
  if (normalized === "/game") {
    return "/game/overview";
  }
  return normalized;
}

export function resolveGameRoute(pathname: string): GameRoute {
  return routeByPath.get(normalizeGamePath(pathname)) ?? overviewRoute;
}

export function gameRouteURL(path: string, search: string): string {
  const query = new URLSearchParams(search);
  const encoded = query.toString();
  return encoded ? `${path}?${encoded}` : path;
}

export function gamePlanetSwitchURL(pathname: string, search: string, planetID: number | string): string {
  const query = new URLSearchParams(search);
  query.set("cp", String(planetID));
  return gameRouteURL(resolveGameRoute(pathname).path, query.toString());
}
