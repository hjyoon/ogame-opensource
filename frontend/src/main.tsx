import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { LegacyGameOverview, type GameOverviewStatus } from "./LegacyGameOverview";
import { LegacyPublicAbout } from "./LegacyPublicAbout";
import { LegacyPublicHome } from "./LegacyPublicHome";
import { LegacyPublicRegister } from "./LegacyPublicRegister";
import { publicRoutes, resolvePublicRoute } from "./routes";
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
  issues: RegistrationIssue[];
  draft: {
    character: string;
    email: string;
    universe: string;
    agb: boolean;
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
  const resolution = resolvePublicRoute(pathname);
  const route = resolution.route;

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
    if (registrationDraft.universe === "" && universes[0]?.baseUrl) {
      setRegistrationDraft((current) => ({ ...current, universe: universes[0].baseUrl }));
    }
    if (loginDraft.universe === "" && universes[0]?.baseUrl) {
      setLoginDraft((current) => ({ ...current, universe: universes[0].baseUrl }));
    }
  }, [loginDraft.universe, registrationDraft.universe, universes]);

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
    window.history.pushState({}, "", path);
    setPathname(path);
    setSearch("");
  };

  if (pathname.startsWith("/game")) {
    return <LegacyGameOverview error={gameOverviewError} status={gameOverview} />;
  }

  const updateRegistrationDraft = (field: keyof RegistrationDraft, value: string | boolean) => {
    setRegistrationDraft((current) => ({ ...current, [field]: value }));
    setRegistrationResult(null);
    setRegistrationError(null);
  };

  const validateRegistration = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setRegistrationPending(true);
    setRegistrationError(null);
    fetch("/api/public/registration/validate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(registrationDraft)
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`registration validation returned ${response.status}`);
        }
        return response.json() as Promise<RegistrationValidation>;
      })
      .then(setRegistrationResult)
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
      .then(setLoginResult)
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
        onRegistrationSubmit={validateRegistration}
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

      {route.key === "universes" ? (
        <section className="panel" data-testid="universe-catalog">
          <div className="panel-title">
            <span>Universe Catalog</span>
            <strong className="badge good">{universes.length} listed</strong>
          </div>
          <div className="universe-list">
            {universes.map((universe) => (
              <article className="universe-row" key={universe.number}>
                <div>
                  <h2>{universe.name}</h2>
                  <p>{universe.language.toUpperCase()} · Economy {universe.speed}x · Fleet {universe.fleetSpeed}x</p>
                </div>
                <a href={universe.baseUrl}>{universe.open ? "Open" : "Closed"}</a>
              </article>
            ))}
          </div>
        </section>
      ) : null}

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

createRoot(document.getElementById("root") as HTMLElement).render(<App />);
