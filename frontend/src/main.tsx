import React, { useEffect, useLayoutEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  LegacyGameOverview,
  type GameBuildingsStatus,
  type GameDefenseStatus,
  type GameFleetStatus,
  type GameGalaxyStatus,
  type GameLogoutStatus,
  type GameNoteDraft,
  type GameNotesStatus,
  type GameOverviewStatus,
  type GameResearchStatus,
  type GameResourcesStatus,
  type GameSearchStatus,
  type GameShipyardStatus,
  type GameStatisticsStatus,
  type GameTechnologyStatus
} from "./LegacyGameOverview";
import { LegacyPublicAbout } from "./LegacyPublicAbout";
import { LegacyPublicHome } from "./LegacyPublicHome";
import { LegacyPublicLegal } from "./LegacyPublicLegal";
import { LegacyPublicRegister } from "./LegacyPublicRegister";
import { LegacyPublicRules } from "./LegacyPublicRules";
import { LegacyPublicScreenshots } from "./LegacyPublicScreenshots";
import { LegacyPublicStory } from "./LegacyPublicStory";
import { LegacyPublicUniverses } from "./LegacyPublicUniverses";
import { resolveGameRoute } from "./gameRoutes";
import { legacyPublicCssHrefs, legacyPublicRouteKeys, publicRoutes, resolvePublicRoute } from "./routes";
import "./styles.css";

type Health = {
  status: string;
  service: string;
  environment: string;
  runtime: string;
  goTarget: string;
  bunTarget: string;
  reactTarget: string;
  staticReady: boolean;
  legacyAssetsReady: boolean;
  legacyBaseUrl: string;
};

type UniverseSummary = {
  number: number;
  name: string;
  baseUrl: string;
  language: string;
  speed: number;
  fleetSpeed: number;
  status: string;
  open: boolean;
};

type RegistrationIssue = {
  field: string;
  code: string;
  message: string;
  legacyErrorCode: number;
};

type RegistrationValidation = {
  valid: boolean;
  created?: boolean;
  issues: RegistrationIssue[];
  draft: {
    character: string;
    email: string;
    universe: string;
    agb: boolean;
  };
  account?: {
    playerId: number;
    homePlanetId: number;
    activationRequired: boolean;
  };
  session?: {
    redirectTo: string;
    universeNumber: number;
  };
};

type RegistrationDraft = {
  character: string;
  password: string;
  email: string;
  universe: string;
  agb: boolean;
};

type LoginIssue = {
  field: string;
  code: string;
  message: string;
  legacyErrorCode: number;
};

type LoginValidation = {
  valid: boolean;
  issues: LoginIssue[];
  draft: {
    login: string;
    universe: string;
  };
  session?: {
    redirectTo: string;
    universeNumber: number;
  };
};

type LoginDraft = {
  login: string;
  pass: string;
  universe: string;
};

const phases = [
  { key: "legacy", label: "Legacy QA", state: "active", owner: "PHP E2E" },
  { key: "shell", label: "React Shell", state: "active", owner: "Bun 1.3" },
  { key: "api", label: "Go API", state: "active", owner: "net/http" },
  { key: "domain", label: "Domain Ports", state: "queued", owner: "Core rules" }
];

function isLegacyPublicPath(pathname: string) {
  return legacyPublicRouteKeys.has(resolvePublicRoute(pathname).route.key);
}

function ensureLegacyPublicCss() {
  for (const href of legacyPublicCssHrefs) {
    if (!document.head.querySelector(`link[data-legacy-public-css="${href}"]`)) {
      const link = document.createElement("link");
      link.dataset.legacyPublicCss = href;
      link.href = href;
      link.rel = "stylesheet";
      document.head.appendChild(link);
    }
  }
}

function syncLegacyPublicChrome(enabled: boolean) {
  document.body.classList.toggle("legacy-public-body", enabled);
  if (enabled) {
    document.body.style.setProperty("--legacy-public-body-bg", 'url("/public-assets/img/sterne_bg2.jpg")');
    ensureLegacyPublicCss();
    return;
  }
  document.body.style.removeProperty("--legacy-public-body-bg");
  document.head.querySelectorAll("link[data-legacy-public-css]").forEach((link) => link.remove());
}

function dispatchClientNavigation(url: string) {
  window.history.pushState({}, "", url);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

function clientNavigableURL(href: string) {
  const target = new URL(href, window.location.href);
  const targetPath = `${target.pathname}${target.search}`;
  const currentPath = `${window.location.pathname}${window.location.search}`;
  if (target.origin !== window.location.origin || target.hash !== "" || targetPath === currentPath) {
    return null;
  }
  const route = resolvePublicRoute(target.pathname).route;
  if (legacyPublicRouteKeys.has(route.key) || target.pathname.startsWith("/game")) {
    return targetPath;
  }
  return null;
}

function App() {
  const [pathname, setPathname] = useState(() => window.location.pathname);
  const [search, setSearch] = useState(() => window.location.search);
  const [health, setHealth] = useState<Health | null>(null);
  const [universes, setUniverses] = useState<UniverseSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [registrationDraft, setRegistrationDraft] = useState<RegistrationDraft>({
    character: "",
    password: "",
    email: "",
    universe: "",
    agb: false
  });
  const [registrationResult, setRegistrationResult] = useState<RegistrationValidation | null>(null);
  const [registrationPending, setRegistrationPending] = useState(false);
  const [registrationError, setRegistrationError] = useState<string | null>(null);
  const [loginDraft, setLoginDraft] = useState<LoginDraft>({
    login: "",
    pass: "",
    universe: ""
  });
  const [loginResult, setLoginResult] = useState<LoginValidation | null>(null);
  const [loginPending, setLoginPending] = useState(false);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [gameOverview, setGameOverview] = useState<GameOverviewStatus | null>(null);
  const [gameOverviewError, setGameOverviewError] = useState<string | null>(null);
  const [gameBuildings, setGameBuildings] = useState<GameBuildingsStatus | null>(null);
  const [gameBuildingsError, setGameBuildingsError] = useState<string | null>(null);
  const [gameResources, setGameResources] = useState<GameResourcesStatus | null>(null);
  const [gameResourcesError, setGameResourcesError] = useState<string | null>(null);
  const [gameResourcesPending, setGameResourcesPending] = useState(false);
  const [gameResearch, setGameResearch] = useState<GameResearchStatus | null>(null);
  const [gameResearchError, setGameResearchError] = useState<string | null>(null);
  const [gameShipyard, setGameShipyard] = useState<GameShipyardStatus | null>(null);
  const [gameShipyardError, setGameShipyardError] = useState<string | null>(null);
  const [gameFleet, setGameFleet] = useState<GameFleetStatus | null>(null);
  const [gameFleetError, setGameFleetError] = useState<string | null>(null);
  const [gameGalaxy, setGameGalaxy] = useState<GameGalaxyStatus | null>(null);
  const [gameGalaxyError, setGameGalaxyError] = useState<string | null>(null);
  const [gameDefense, setGameDefense] = useState<GameDefenseStatus | null>(null);
  const [gameDefenseError, setGameDefenseError] = useState<string | null>(null);
  const [gameTechnology, setGameTechnology] = useState<GameTechnologyStatus | null>(null);
  const [gameTechnologyError, setGameTechnologyError] = useState<string | null>(null);
  const [gameStatistics, setGameStatistics] = useState<GameStatisticsStatus | null>(null);
  const [gameStatisticsError, setGameStatisticsError] = useState<string | null>(null);
  const [gameSearch, setGameSearch] = useState<GameSearchStatus | null>(null);
  const [gameSearchError, setGameSearchError] = useState<string | null>(null);
  const [gameNotes, setGameNotes] = useState<GameNotesStatus | null>(null);
  const [gameNotesError, setGameNotesError] = useState<string | null>(null);
  const [gameNotesPending, setGameNotesPending] = useState(false);
  const [gameLogout, setGameLogout] = useState<GameLogoutStatus | null>(null);
  const [gameLogoutError, setGameLogoutError] = useState<string | null>(null);
  const resolution = resolvePublicRoute(pathname);
  const route = resolution.route;
  const gameRoute = pathname.startsWith("/game") ? resolveGameRoute(pathname) : null;
  const isLegacyPublicRoute = legacyPublicRouteKeys.has(route.key);

  useLayoutEffect(() => {
    syncLegacyPublicChrome(isLegacyPublicRoute);
  }, [isLegacyPublicRoute]);

  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) {
        return;
      }
      const anchor = (event.target instanceof Element ? event.target.closest("a[href]") : null) as HTMLAnchorElement | null;
      if (!anchor || (anchor.target && anchor.target !== "_self")) {
        return;
      }
      const target = clientNavigableURL(anchor.getAttribute("href") ?? "");
      if (!target) {
        return;
      }
      event.preventDefault();
      dispatchClientNavigation(target);
    };
    document.addEventListener("click", onClick);
    return () => document.removeEventListener("click", onClick);
  }, []);

  useEffect(() => {
    fetch("/api/healthz")
      .then((response) => {
        if (!response.ok) {
          throw new Error(`healthz returned ${response.status}`);
        }
        return response.json() as Promise<Health>;
      })
      .then(setHealth)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
  }, []);

  useEffect(() => {
    fetch("/api/public/universes")
      .then((response) => {
        if (!response.ok) {
          throw new Error(`universes returned ${response.status}`);
        }
        return response.json() as Promise<{ universes: UniverseSummary[] }>;
      })
      .then((payload) => setUniverses(payload.universes))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
  }, []);

  useEffect(() => {
    if (registrationDraft.universe === "" && universes.length > 0) {
      const recommended = universes.find((universe) => universe.number === 3 && universe.baseUrl) ?? universes[0];
      if (recommended?.baseUrl) {
        setRegistrationDraft((current) => ({ ...current, universe: recommended.baseUrl }));
      }
    }
  }, [registrationDraft.universe, universes]);

  useEffect(() => {
    const onPopState = () => {
      setPathname(window.location.pathname);
      setSearch(window.location.search);
    };
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (!pathname.startsWith("/game") || gameRoute?.key === "logout" || publicSession === "") {
      setGameOverview(null);
      setGameOverviewError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const overviewSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      overviewSearch.set("cp", selectedPlanet);
    }
    fetch(`/api/game/overview?${overviewSearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameOverviewStatus>)
      .then((payload) => {
        setGameOverview(payload);
        setGameOverviewError(null);
      })
      .catch((err: unknown) => setGameOverviewError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, pathname, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "buildings" || publicSession === "") {
      setGameBuildings(null);
      setGameBuildingsError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const buildingsSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      buildingsSearch.set("cp", selectedPlanet);
    }
    fetch(`/api/game/buildings?${buildingsSearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameBuildingsStatus>)
      .then((payload) => {
        setGameBuildings(payload);
        setGameBuildingsError(null);
      })
      .catch((err: unknown) => setGameBuildingsError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  const submitGameResources = (production: Record<string, string>) => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (publicSession === "") {
      setGameResourcesError("Session is invalid.");
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const resourcesSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      resourcesSearch.set("cp", selectedPlanet);
    }
    setGameResourcesPending(true);
    setGameResourcesError(null);
    fetch(`/api/game/resources?${resourcesSearch.toString()}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify({ production })
    })
      .then(async (response) => {
        const text = await response.text();
        const payload = text ? (JSON.parse(text) as GameResourcesStatus) : null;
        if (!response.ok && response.status !== 401) {
          throw new Error(text || `resources returned ${response.status}`);
        }
        if (!payload) {
          throw new Error("resources response was empty");
        }
        return payload;
      })
      .then((payload) => {
        setGameResources(payload);
        setGameResourcesError(null);
      })
      .catch((err: unknown) => setGameResourcesError(err instanceof Error ? err.message : String(err)))
      .finally(() => setGameResourcesPending(false));
  };

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "resources" || publicSession === "") {
      setGameResources(null);
      setGameResourcesError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const resourcesSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      resourcesSearch.set("cp", selectedPlanet);
    }
    fetch(`/api/game/resources?${resourcesSearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameResourcesStatus>)
      .then((payload) => {
        setGameResources(payload);
        setGameResourcesError(null);
      })
      .catch((err: unknown) => setGameResourcesError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "research" || publicSession === "") {
      setGameResearch(null);
      setGameResearchError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const researchSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      researchSearch.set("cp", selectedPlanet);
    }
    fetch(`/api/game/research?${researchSearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameResearchStatus>)
      .then((payload) => {
        setGameResearch(payload);
        setGameResearchError(null);
      })
      .catch((err: unknown) => setGameResearchError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "shipyard" || publicSession === "") {
      setGameShipyard(null);
      setGameShipyardError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const shipyardSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      shipyardSearch.set("cp", selectedPlanet);
    }
    fetch(`/api/game/shipyard?${shipyardSearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameShipyardStatus>)
      .then((payload) => {
        setGameShipyard(payload);
        setGameShipyardError(null);
      })
      .catch((err: unknown) => setGameShipyardError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "fleet" || publicSession === "") {
      setGameFleet(null);
      setGameFleetError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const fleetSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      fleetSearch.set("cp", selectedPlanet);
    }
    fetch(`/api/game/fleet?${fleetSearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameFleetStatus>)
      .then((payload) => {
        setGameFleet(payload);
        setGameFleetError(null);
      })
      .catch((err: unknown) => setGameFleetError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "galaxy" || publicSession === "") {
      setGameGalaxy(null);
      setGameGalaxyError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const galaxySearch = new URLSearchParams({ session: publicSession });
    for (const key of ["cp", "galaxy", "system", "position", "p1", "p2", "p3"]) {
      const value = currentSearch.get(key);
      if (value) {
        galaxySearch.set(key, value);
      }
    }
    fetch(`/api/game/galaxy?${galaxySearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameGalaxyStatus>)
      .then((payload) => {
        setGameGalaxy(payload);
        setGameGalaxyError(null);
      })
      .catch((err: unknown) => setGameGalaxyError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "defense" || publicSession === "") {
      setGameDefense(null);
      setGameDefenseError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const defenseSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      defenseSearch.set("cp", selectedPlanet);
    }
    fetch(`/api/game/defense?${defenseSearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameDefenseStatus>)
      .then((payload) => {
        setGameDefense(payload);
        setGameDefenseError(null);
      })
      .catch((err: unknown) => setGameDefenseError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "technology" || publicSession === "") {
      setGameTechnology(null);
      setGameTechnologyError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const technologySearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      technologySearch.set("cp", selectedPlanet);
    }
    const selectedTechnology = currentSearch.get("tid");
    if (selectedTechnology) {
      technologySearch.set("tid", selectedTechnology);
    }
    fetch(`/api/game/technology?${technologySearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameTechnologyStatus>)
      .then((payload) => {
        setGameTechnology(payload);
        setGameTechnologyError(null);
      })
      .catch((err: unknown) => setGameTechnologyError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "statistics" || publicSession === "") {
      setGameStatistics(null);
      setGameStatisticsError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const statisticsSearch = new URLSearchParams({ session: publicSession });
    for (const key of ["cp", "who", "type", "start"]) {
      const value = currentSearch.get(key);
      if (value) {
        statisticsSearch.set(key, value);
      }
    }
    fetch(`/api/game/statistics?${statisticsSearch.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameStatisticsStatus>)
      .then((payload) => {
        setGameStatistics(payload);
        setGameStatisticsError(null);
      })
      .catch((err: unknown) => setGameStatisticsError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "search" || publicSession === "") {
      setGameSearch(null);
      setGameSearchError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const searchRequest = new URLSearchParams({ session: publicSession });
    for (const key of ["cp", "type", "searchtext"]) {
      const value = currentSearch.get(key);
      if (value) {
        searchRequest.set(key, value);
      }
    }
    fetch(`/api/game/search?${searchRequest.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameSearchStatus>)
      .then((payload) => {
        setGameSearch(payload);
        setGameSearchError(null);
      })
      .catch((err: unknown) => setGameSearchError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "notes" || publicSession === "") {
      setGameNotes(null);
      setGameNotesError(null);
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const notesRequest = new URLSearchParams({ session: publicSession });
    for (const key of ["cp", "a", "n"]) {
      const value = currentSearch.get(key);
      if (value) {
        notesRequest.set(key, value);
      }
    }
    fetch(`/api/game/notes?${notesRequest.toString()}`, { credentials: "same-origin" })
      .then((response) => response.json() as Promise<GameNotesStatus>)
      .then((payload) => {
        setGameNotes(payload);
        setGameNotesError(null);
      })
      .catch((err: unknown) => setGameNotesError(err instanceof Error ? err.message : String(err)));
  }, [gameRoute?.key, search]);

  const submitGameNotesMutation = (body: Record<string, unknown>) => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (publicSession === "") {
      setGameNotesError("Session is invalid.");
      return;
    }
    const currentSearch = new URLSearchParams(search);
    const notesSearch = new URLSearchParams({ session: publicSession });
    const selectedPlanet = currentSearch.get("cp");
    if (selectedPlanet) {
      notesSearch.set("cp", selectedPlanet);
    }
    setGameNotesPending(true);
    setGameNotesError(null);
    fetch(`/api/game/notes?${notesSearch.toString()}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify(body)
    })
      .then(async (response) => {
        const text = await response.text();
        const payload = text ? (JSON.parse(text) as GameNotesStatus) : null;
        if (!response.ok && response.status !== 401) {
          throw new Error(text || `notes returned ${response.status}`);
        }
        if (!payload) {
          throw new Error("notes response was empty");
        }
        return payload;
      })
      .then((payload) => {
        setGameNotes(payload);
        setGameNotesError(null);
        dispatchClientNavigation(`/game/notes?${notesSearch.toString()}`);
      })
      .catch((err: unknown) => setGameNotesError(err instanceof Error ? err.message : String(err)))
      .finally(() => setGameNotesPending(false));
  };

  const submitGameNoteCreate = (draft: GameNoteDraft) => {
    submitGameNotesMutation({ action: "create", subject: draft.subject, text: draft.text, priority: draft.priority });
  };

  const submitGameNoteUpdate = (noteId: number, draft: GameNoteDraft) => {
    submitGameNotesMutation({ action: "update", noteId, subject: draft.subject, text: draft.text, priority: draft.priority });
  };

  const submitGameNoteDelete = (noteIds: number[]) => {
    submitGameNotesMutation({ action: "delete", noteIds });
  };

  useEffect(() => {
    const publicSession = new URLSearchParams(search).get("session") ?? "";
    if (gameRoute?.key !== "logout") {
      setGameLogout(null);
      setGameLogoutError(null);
      return;
    }

    let cancelled = false;
    let redirectTimer: number | undefined;
    const scheduleHomeRedirect = (target: string) => {
      redirectTimer = window.setTimeout(() => dispatchClientNavigation(target || "/home"), 3_000);
    };

    if (publicSession === "") {
      setGameLogout({ loggedOut: false, redirectTo: "/home" });
      setGameLogoutError(null);
      scheduleHomeRedirect("/home");
      return () => {
        if (redirectTimer !== undefined) {
          window.clearTimeout(redirectTimer);
        }
      };
    }

    setGameLogout(null);
    setGameLogoutError(null);
    fetch(`/api/game/logout?${new URLSearchParams({ session: publicSession }).toString()}`, {
      method: "POST",
      credentials: "same-origin"
    })
      .then(async (response) => {
        const text = await response.text();
        if (!response.ok) {
          throw new Error(text || `logout returned ${response.status}`);
        }
        return (text ? JSON.parse(text) : { loggedOut: false, redirectTo: "/home" }) as GameLogoutStatus;
      })
      .then((payload) => {
        if (cancelled) {
          return;
        }
        setGameLogout(payload);
        setGameLogoutError(null);
        scheduleHomeRedirect(payload.redirectTo);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setGameLogoutError(err instanceof Error ? err.message : String(err));
        }
      });

    return () => {
      cancelled = true;
      if (redirectTimer !== undefined) {
        window.clearTimeout(redirectTimer);
      }
    };
  }, [gameRoute?.key, search]);

  const checks = useMemo(
    () => [
      ["Go target", health?.goTarget ?? "1.25"],
      ["React target", health?.reactTarget ?? "19"],
      ["Bun target", health?.bunTarget ?? "1.3"],
      ["Legacy oracle", health?.legacyBaseUrl ?? "pending"]
    ],
    [health]
  );

  const navigate = (event: React.MouseEvent<HTMLAnchorElement>, path: string) => {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) {
      return;
    }
    event.preventDefault();
    dispatchClientNavigation(path);
  };

  if (pathname.startsWith("/game")) {
    return (
      <LegacyGameOverview
        buildingsError={gameBuildingsError}
        buildingsStatus={gameBuildings}
        defenseError={gameDefenseError}
        defenseStatus={gameDefense}
        error={gameOverviewError}
        fleetError={gameFleetError}
        fleetStatus={gameFleet}
        galaxyError={gameGalaxyError}
        galaxyStatus={gameGalaxy}
        logoutError={gameLogoutError}
        logoutStatus={gameLogout}
        notesError={gameNotesError}
        notesPending={gameNotesPending}
        notesStatus={gameNotes}
        onNotesCreate={submitGameNoteCreate}
        onNotesDelete={submitGameNoteDelete}
        onNotesUpdate={submitGameNoteUpdate}
        onResourcesSubmit={submitGameResources}
        route={gameRoute ?? resolveGameRoute(pathname)}
        resourcesError={gameResourcesError}
        resourcesPending={gameResourcesPending}
        resourcesStatus={gameResources}
        researchError={gameResearchError}
        researchStatus={gameResearch}
        shipyardError={gameShipyardError}
        shipyardStatus={gameShipyard}
        statisticsError={gameStatisticsError}
        statisticsStatus={gameStatistics}
        searchError={gameSearchError}
        searchStatus={gameSearch}
        status={gameOverview}
        technologyError={gameTechnologyError}
        technologyStatus={gameTechnology}
      />
    );
  }

  const updateRegistrationDraft = (field: keyof RegistrationDraft, value: string | boolean) => {
    setRegistrationDraft((current) => ({ ...current, [field]: value }));
    setRegistrationResult(null);
    setRegistrationError(null);
  };

  const submitRegistration = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setRegistrationPending(true);
    setRegistrationError(null);
    fetch("/api/public/registration", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify(registrationDraft)
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`registration returned ${response.status}`);
        }
        return response.json() as Promise<RegistrationValidation>;
      })
      .then((result) => {
        setRegistrationResult(result);
        if (result.valid && result.created && result.session?.redirectTo) {
          const target = new URL(result.session.redirectTo, window.location.origin);
          window.history.pushState({}, "", `${target.pathname}${target.search}`);
          setPathname(target.pathname);
          setSearch(target.search);
        }
      })
      .catch((err: unknown) => setRegistrationError(err instanceof Error ? err.message : String(err)))
      .finally(() => setRegistrationPending(false));
  };

  const updateLoginDraft = (field: keyof LoginDraft, value: string) => {
    setLoginDraft((current) => ({ ...current, [field]: value }));
    setLoginResult(null);
    setLoginError(null);
  };

  const submitLogin = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setLoginPending(true);
    setLoginError(null);
    fetch("/api/public/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(loginDraft)
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`login returned ${response.status}`);
        }
        return response.json() as Promise<LoginValidation>;
      })
      .then((result) => {
        setLoginResult(result);
        if (result.valid && result.session?.redirectTo) {
          const target = new URL(result.session.redirectTo, window.location.origin);
          window.history.pushState({}, "", `${target.pathname}${target.search}`);
          setPathname(target.pathname);
          setSearch(target.search);
        }
      })
      .catch((err: unknown) => setLoginError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoginPending(false));
  };

  if (route.key === "home") {
    return (
      <LegacyPublicHome
        loginDraft={loginDraft}
        loginError={loginError}
        loginPending={loginPending}
        loginResult={loginResult}
        onLoginChange={updateLoginDraft}
        onLoginSubmit={submitLogin}
        universes={universes}
      />
    );
  }

  if (route.key === "register") {
    return (
      <LegacyPublicRegister
        loginDraft={loginDraft}
        loginError={loginError}
        loginPending={loginPending}
        loginResult={loginResult}
        onLoginChange={updateLoginDraft}
        onLoginSubmit={submitLogin}
        onRegistrationChange={updateRegistrationDraft}
        onRegistrationSubmit={submitRegistration}
        registrationDraft={registrationDraft}
        registrationError={registrationError}
        registrationPending={registrationPending}
        registrationResult={registrationResult}
        universes={universes}
      />
    );
  }

  if (route.key === "about") {
    return (
      <LegacyPublicAbout
        loginDraft={loginDraft}
        loginError={loginError}
        loginPending={loginPending}
        loginResult={loginResult}
        onLoginChange={updateLoginDraft}
        onLoginSubmit={submitLogin}
        universes={universes}
      />
    );
  }

  if (route.key === "story") {
    return (
      <LegacyPublicStory
        loginDraft={loginDraft}
        loginError={loginError}
        loginPending={loginPending}
        loginResult={loginResult}
        onLoginChange={updateLoginDraft}
        onLoginSubmit={submitLogin}
        universes={universes}
      />
    );
  }

  if (route.key === "screenshots") {
    return (
      <LegacyPublicScreenshots
        loginDraft={loginDraft}
        loginError={loginError}
        loginPending={loginPending}
        loginResult={loginResult}
        onLoginChange={updateLoginDraft}
        onLoginSubmit={submitLogin}
        universes={universes}
      />
    );
  }

  if (route.key === "rules") {
    return (
      <LegacyPublicRules
        loginDraft={loginDraft}
        loginError={loginError}
        loginPending={loginPending}
        loginResult={loginResult}
        onLoginChange={updateLoginDraft}
        onLoginSubmit={submitLogin}
        universes={universes}
      />
    );
  }

  if (route.key === "universes") {
    return (
      <LegacyPublicUniverses
        loginDraft={loginDraft}
        loginError={loginError}
        loginPending={loginPending}
        loginResult={loginResult}
        onLoginChange={updateLoginDraft}
        onLoginSubmit={submitLogin}
        universes={universes}
      />
    );
  }

  if (route.key === "legal") {
    return <LegacyPublicLegal />;
  }

  return (
    <main className="app-shell" data-route={route.key} data-legacy-alias={resolution.isLegacyAlias ? "true" : "false"}>
      <nav className="top-nav" aria-label="Public navigation">
        {publicRoutes.slice(0, 8).map((item) => (
          <a
            aria-current={item.key === route.key ? "page" : undefined}
            href={item.path}
            key={item.key}
            onClick={(event) => navigate(event, item.path)}
          >
            {item.label}
          </a>
        ))}
      </nav>

      <section className="status-band">
        <div>
          <p className="eyebrow">{route.eyebrow}</p>
          <h1>{route.title}</h1>
          <p className="subtle">{route.summary}</p>
        </div>
        <img className="planet" alt={`${route.label} visual asset`} src={route.image} />
      </section>

      <section className="grid two">
        <div className="panel">
          <div className="panel-title">
            <span>{route.label}</span>
            <strong className="badge neutral">{route.status}</strong>
          </div>
          <div className="route-points">
            {route.points.map((point) => (
              <div className="gate" key={point}>
                <span className="dot active" />
                <span>{point}</span>
              </div>
            ))}
          </div>
          {resolution.isLegacyAlias ? (
            <p className="alias-note">Legacy URL alias. Canonical route: {resolution.canonicalPath}</p>
          ) : null}
          {pathname.startsWith("/game") && gameOverview?.authenticated && gameOverview.overview ? (
            <p className="form-success">
              Commander: {gameOverview.overview.commander} · {gameOverview.overview.currentPlanet.name} [
              {gameOverview.overview.currentPlanet.coordinates.galaxy}:{gameOverview.overview.currentPlanet.coordinates.system}:
              {gameOverview.overview.currentPlanet.coordinates.position}] · Metal{" "}
              {Math.floor(gameOverview.overview.currentPlanet.resources.metal)} · Crystal{" "}
              {Math.floor(gameOverview.overview.currentPlanet.resources.crystal)} · Deuterium{" "}
              {Math.floor(gameOverview.overview.currentPlanet.resources.deuterium)}
            </p>
          ) : null}
          {pathname.startsWith("/game") && gameOverview && !gameOverview.authenticated ? (
            <p className="form-error">{gameOverview.issues[0]?.message ?? "Session is invalid."}</p>
          ) : null}
          {pathname.startsWith("/game") && gameOverviewError ? <p className="form-error">{gameOverviewError}</p> : null}
        </div>

        <div className="panel">
          <div className="panel-title">
            <span>Runtime</span>
            <strong className={error ? "badge bad" : "badge good"}>{error ? "degraded" : health?.status ?? "loading"}</strong>
          </div>
          <dl className="facts">
            {checks.map(([label, value]) => (
              <React.Fragment key={label}>
                <dt>{label}</dt>
                <dd>{value}</dd>
              </React.Fragment>
            ))}
            <dt>Server</dt>
            <dd>{health?.runtime ?? error ?? "waiting for /api/healthz"}</dd>
          </dl>
        </div>

        <div className="panel">
          <div className="panel-title">
            <span>Compatibility Gates</span>
            <strong className="badge neutral">baseline</strong>
          </div>
          <div className="gate-list">
            <Gate label="Existing Docker E2E" ready />
            <Gate label="Static React build" ready={Boolean(health?.staticReady)} />
            <Gate label="Legacy image assets" ready={Boolean(health?.legacyAssetsReady)} />
            <Gate label="Universe catalog API" ready={universes.length > 0} />
            <Gate label="Game overview API" ready={Boolean(gameOverview?.authenticated || !pathname.startsWith("/game"))} />
          </div>
        </div>
      </section>

      <section className="panel" id="migration">
        <div className="panel-title">
          <span>Migration Phases</span>
          <strong className="badge neutral">stepwise</strong>
        </div>
        <div className="phase-grid">
          {phases.map((phase) => (
            <article className="phase" key={phase.key}>
              <span className={`dot ${phase.state}`} />
              <h2>{phase.label}</h2>
              <p>{phase.owner}</p>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}

function Gate({ label, ready }: { label: string; ready: boolean }) {
  return (
    <div className="gate">
      <span className={`dot ${ready ? "active" : "queued"}`} />
      <span>{label}</span>
      <strong>{ready ? "ready" : "pending"}</strong>
    </div>
  );
}

syncLegacyPublicChrome(isLegacyPublicPath(window.location.pathname));
createRoot(document.getElementById("root") as HTMLElement).render(<App />);
