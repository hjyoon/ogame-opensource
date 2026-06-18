import React, { useEffect, useLayoutEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { LegacyGameOverview, type GameBuildingsStatus, type GameOverviewStatus } from "./LegacyGameOverview";
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
    if (!pathname.startsWith("/game") || publicSession === "") {
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
  }, [pathname, search]);

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
        error={gameOverviewError}
        route={gameRoute ?? resolveGameRoute(pathname)}
        status={gameOverview}
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
