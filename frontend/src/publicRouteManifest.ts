import type { PublicRoute } from "./routes";

export type PublicRouteManifestEntry = PublicRoute & {
  legacyAliases: string[];
  legacyPublicChrome: boolean;
};

export const legacyPublicCssHrefs = ["/public-assets/css/styles.css", "/public-assets/css/about.css"];

export const publicRouteManifest: PublicRouteManifestEntry[] = [
  {
    key: "home",
    path: "/home",
    label: "Home",
    eyebrow: "Command Entry",
    title: "OGame Classic",
    summary: "A React public entry backed by Go draft validation, keeping the old game rules as the oracle.",
    status: "Login validation",
    image: "/legacy-assets/use/uV/planeten/small/s_normaltempplanet01.jpg",
    points: ["Legacy login field names", "Universe selection", "Strict game-rule parity"],
    legacyAliases: ["/home.php", "/index.php", "/install.php"],
    legacyPublicChrome: true
  },
  {
    key: "register",
    path: "/register",
    label: "Register",
    eyebrow: "New Commander",
    title: "Create an account",
    summary: "Registration creation now runs through native Go while keeping legacy account and home planet defaults.",
    status: "Creation API",
    image: "/legacy-assets/use/uV/planeten/small/s_dschjungelplanet03.jpg",
    points: ["Legacy field names", "Universe selection", "Login after registration"],
    legacyAliases: ["/register.php"],
    legacyPublicChrome: true
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
    points: ["Multi-universe ready", "Natural route names", "Legacy URL aliases"],
    legacyAliases: ["/unis.php"],
    legacyPublicChrome: true
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
    points: ["Go backend", "React frontend", "PHP oracle QA"],
    legacyAliases: ["/about.php"],
    legacyPublicChrome: true
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
    points: ["Content route", "Asset reuse", "No PHP page coupling"],
    legacyAliases: ["/story.php"],
    legacyPublicChrome: true
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
    points: ["Visual regression anchor", "Classic UI density", "Responsive shell"],
    legacyAliases: ["/screenshots.php"],
    legacyPublicChrome: true
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
    points: ["Public read route", "No server mutation", "Future moderation hooks"],
    legacyAliases: ["/regeln.php"],
    legacyPublicChrome: true
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
    points: ["Natural URL", "Static content shell", "Legacy alias support"],
    legacyAliases: ["/impressum.php"],
    legacyPublicChrome: false
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
    points: ["Bun 1.3 build", "Go 1.25 net/http", "97% coverage gate"],
    legacyAliases: [],
    legacyPublicChrome: false
  }
];
