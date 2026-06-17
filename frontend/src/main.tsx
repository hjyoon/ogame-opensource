import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
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

const phases = [
  { key: "legacy", label: "Legacy QA", state: "active", owner: "PHP E2E" },
  { key: "shell", label: "React Shell", state: "active", owner: "Bun 1.3" },
  { key: "api", label: "Go API", state: "active", owner: "net/http" },
  { key: "domain", label: "Domain Ports", state: "queued", owner: "Core rules" }
];

function App() {
  const [pathname, setPathname] = useState(() => window.location.pathname);
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);
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
    const onPopState = () => setPathname(window.location.pathname);
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

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
  };

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

createRoot(document.getElementById("root") as HTMLElement).render(<App />);
