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

export const publicRoutes: PublicRoute[] = [
  {
    key: "home",
    path: "/home",
    label: "Home",
    eyebrow: "Command Entry",
    title: "OGame Classic",
    summary: "A React public shell for the Go migration, keeping the old game rules as the oracle.",
    status: "Public shell",
    image: "/legacy-assets/use/uV/planeten/small/s_normaltempplanet01.jpg",
    points: ["Universe login entry", "Legacy asset compatibility", "Strict game-rule parity"]
  },
  {
    key: "register",
    path: "/register",
    label: "Register",
    eyebrow: "New Commander",
    title: "Create an account",
    summary: "Registration draft validation now runs through a native Go use case before account creation is ported.",
    status: "Validation API",
    image: "/legacy-assets/use/uV/planeten/small/s_dschjungelplanet03.jpg",
    points: ["Legacy field names", "Universe selection", "No account mutation yet"]
  },
  {
    key: "universes",
    path: "/universes",
    label: "Universes",
    eyebrow: "Universe Directory",
    title: "Choose a universe",
    summary: "The new route model separates public navigation from old PHP filenames.",
    status: "Compatibility shell",
    image: "/legacy-assets/use/uV/planeten/small/s_gasplanet04.jpg",
    points: ["Multi-universe ready", "Natural route names", "Legacy URL aliases"]
  },
  {
    key: "about",
    path: "/about",
    label: "About",
    eyebrow: "Project",
    title: "Classic mechanics, new runtime",
    summary: "The migration changes implementation shape, not gameplay outcomes.",
    status: "Reference page",
    image: "/legacy-assets/use/uV/planeten/small/s_trockenplanet08.jpg",
    points: ["Go backend", "React frontend", "PHP oracle QA"]
  },
  {
    key: "story",
    path: "/story",
    label: "Story",
    eyebrow: "Archive",
    title: "Empire briefing",
    summary: "Narrative pages move into frontend routes while gameplay remains rule-compatible.",
    status: "Static content",
    image: "/legacy-assets/use/uV/planeten/small/s_wasserplanet06.jpg",
    points: ["Content route", "Asset reuse", "No PHP page coupling"]
  },
  {
    key: "screenshots",
    path: "/screenshots",
    label: "Screens",
    eyebrow: "Visual Archive",
    title: "Legacy interface gallery",
    summary: "Screenshots and visual references guide the React rebuild without dictating PHP structure.",
    status: "Gallery shell",
    image: "/legacy-assets/use/uV/planeten/small/s_eisplanet08.jpg",
    points: ["Visual regression anchor", "Classic UI density", "Responsive shell"]
  },
  {
    key: "rules",
    path: "/rules",
    label: "Rules",
    eyebrow: "Policy",
    title: "Game conduct",
    summary: "Rules are public content; enforcement belongs in Go application use cases.",
    status: "Public content",
    image: "/legacy-assets/use/uV/planeten/small/s_normaltempplanet04.jpg",
    points: ["Public read route", "No server mutation", "Future moderation hooks"]
  },
  {
    key: "legal",
    path: "/legal",
    label: "Legal",
    eyebrow: "Impressum",
    title: "Project notice",
    summary: "Legal and project notices become stable public routes without PHP suffixes.",
    status: "Public content",
    image: "/legacy-assets/use/uV/planeten/small/s_trockenplanet04.jpg",
    points: ["Natural URL", "Static content shell", "Legacy alias support"]
  },
  {
    key: "migration",
    path: "/migration",
    label: "Migration",
    eyebrow: "Migration Console",
    title: "Go/React port status",
    summary: "Runtime, QA, and compatibility gates for the staged migration.",
    status: "Active",
    image: "/legacy-assets/use/uV/planeten/small/s_mond.jpg",
    points: ["Bun 1.3 build", "Go 1.25 net/http", "97% coverage gate"]
  }
];

const routeByPath = new Map(publicRoutes.map((route) => [route.path, route]));

const legacyAliases = new Map<string, string>([
  ["/about.php", "/about"],
  ["/home.php", "/home"],
  ["/impressum.php", "/legal"],
  ["/index.php", "/home"],
  ["/install.php", "/home"],
  ["/register.php", "/register"],
  ["/regeln.php", "/rules"],
  ["/screenshots.php", "/screenshots"],
  ["/story.php", "/story"],
  ["/unis.php", "/universes"]
]);

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
  const legacyPath = legacyAliases.get(normalized);
  const canonicalPath = legacyPath ?? naturalPath;
  const route = routeByPath.get(canonicalPath) ?? routeByPath.get("/migration") ?? publicRoutes[0];
  return {
    route,
    canonicalPath,
    isLegacyAlias: legacyPath !== undefined
  };
}
