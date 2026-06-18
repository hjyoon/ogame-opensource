import { legacyPublicCssHrefs, publicRouteManifest } from "./publicRouteManifest";

export type PublicRouteKey =
  | "home"
  | "register"
  | "universes"
  | "about"
  | "story"
  | "screenshots"
  | "rules"
  | "legal"
  | "migration";

export type PublicRoute = {
  key: PublicRouteKey;
  path: string;
  label: string;
  eyebrow: string;
  title: string;
  summary: string;
  status: string;
  image: string;
  points: string[];
};

export type RouteResolution = {
  route: PublicRoute;
  canonicalPath: string;
  isLegacyAlias: boolean;
};

export const publicRoutes: PublicRoute[] = publicRouteManifest.map(({ legacyAliases, legacyPublicChrome, legacyVisualPath, ...route }) => route);

const routeByPath = new Map(publicRoutes.map((route) => [route.path, route]));

export const publicRouteAliases = new Map<string, string>(
  publicRouteManifest.flatMap((route) => route.legacyAliases.map((alias) => [alias, route.path]))
);

export const legacyPublicRouteKeys = new Set<PublicRouteKey>(
  publicRouteManifest.filter((route) => route.legacyPublicChrome).map((route) => route.key)
);

export const legacyPublicBootstrapPaths = Array.from(
  new Set([
    "/",
    ...publicRouteManifest
      .filter((route) => route.legacyPublicChrome)
      .flatMap((route) => [route.path, ...route.legacyAliases])
  ])
);

export { legacyPublicCssHrefs };

export function normalizePath(pathname: string): string {
  const path = pathname.split(/[?#]/, 1)[0] || "/";
  if (path === "/") {
    return "/";
  }
  return path.replace(/\/+$/, "") || "/";
}

export function resolvePublicRoute(pathname: string): RouteResolution {
  const normalized = normalizePath(pathname);
  const naturalPath = normalized === "/" ? "/home" : normalized;
  const legacyPath = publicRouteAliases.get(normalized);
  const canonicalPath = legacyPath ?? naturalPath;
  const route = routeByPath.get(canonicalPath) ?? routeByPath.get("/migration") ?? publicRoutes[0];
  return {
    route,
    canonicalPath,
    isLegacyAlias: legacyPath !== undefined
  };
}
