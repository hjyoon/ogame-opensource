export type GameRouteKey =
  | "overview"
  | "empire"
  | "buildings"
  | "resources"
  | "research"
  | "shipyard"
  | "fleet"
  | "technology"
  | "galaxy"
  | "defense"
  | "alliance"
  | "statistics"
  | "messages";

export type GameRoute = {
  key: GameRouteKey;
  label: string;
  path: string;
  migrated: boolean;
};

export const gameRoutes: GameRoute[] = [
  { key: "overview", label: "Overview", path: "/game/overview", migrated: true },
  { key: "empire", label: "Empire", path: "/game/empire", migrated: false },
  { key: "buildings", label: "Buildings", path: "/game/buildings", migrated: false },
  { key: "resources", label: "Resources", path: "/game/resources", migrated: false },
  { key: "research", label: "Research", path: "/game/research", migrated: false },
  { key: "shipyard", label: "Shipyard", path: "/game/shipyard", migrated: false },
  { key: "fleet", label: "Fleet", path: "/game/fleet", migrated: false },
  { key: "technology", label: "Technology", path: "/game/technology", migrated: false },
  { key: "galaxy", label: "Galaxy", path: "/game/galaxy", migrated: false },
  { key: "defense", label: "Defense", path: "/game/defense", migrated: false },
  { key: "alliance", label: "Alliance", path: "/game/alliance", migrated: false },
  { key: "statistics", label: "Statistics", path: "/game/statistics", migrated: false },
  { key: "messages", label: "Messages", path: "/game/messages", migrated: false }
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
