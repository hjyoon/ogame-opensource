import React from "react";
import { gameRouteURL, gameRoutes, type GameRoute } from "./gameRoutes";

export type GameOverviewStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  overview?: GameOverview;
};

export type GameBuildingsStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  buildings?: GameBuildings;
};

type GameOverview = {
  commander: string;
  score: {
    points: number;
    rawScore: number;
    rank: number;
    universePlayers: number;
  };
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
};

type GamePlanetOverview = {
  id: number;
  name: string;
  type: number;
  coordinates: Coordinates;
  diameter: number;
  temperature: number;
  fields: number;
  maxFields: number;
  resources: Resources;
};

type GamePlanetSummary = {
  id: number;
  name: string;
  type: number;
  coordinates: Coordinates;
  current: boolean;
};

type Coordinates = {
  galaxy: number;
  system: number;
  position: number;
};

type Resources = {
  metal: number;
  crystal: number;
  deuterium: number;
};

type GameBuildings = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  items: GameBuildingItem[];
};

type GameBuildingItem = {
  id: number;
  name: string;
  description: string;
  level: number;
  nextLevel: number;
  cost: BuildingCost;
  durationSeconds: number;
  canBuild: boolean;
  action: string;
};

type BuildingCost = {
  metal: number;
  crystal: number;
  deuterium: number;
  energy: number;
};

type LegacyGameOverviewProps = {
  status: GameOverviewStatus | null;
  error: string | null;
  route: GameRoute;
  buildingsStatus: GameBuildingsStatus | null;
  buildingsError: string | null;
};

type LegacyMenuEntry =
  | { type: "image"; height: number; src: string; width: number }
  | { type: "route"; key: GameRoute["key"] };

const skinBase = "/public-assets/evolution";
const gameImageBase = "/public-assets/game-img";
const gameRouteByKey = new Map(gameRoutes.map((route) => [route.key, route]));
const legacyMenuEntries: LegacyMenuEntry[] = [
  { type: "image", height: 40, src: `${skinBase}/gfx/ogame-produktion.jpg`, width: 110 },
  { type: "route", key: "overview" },
  { type: "route", key: "admin" },
  { type: "route", key: "buildings" },
  { type: "route", key: "resources" },
  { type: "route", key: "merchant" },
  { type: "route", key: "research" },
  { type: "route", key: "shipyard" },
  { type: "route", key: "fleet" },
  { type: "route", key: "technology" },
  { type: "route", key: "galaxy" },
  { type: "route", key: "defense" },
  { type: "image", height: 19, src: `${skinBase}/gfx/info-help.jpg`, width: 110 },
  { type: "route", key: "alliance" },
  { type: "route", key: "officers" },
  { type: "route", key: "statistics" },
  { type: "route", key: "search" },
  { type: "image", height: 19, src: `${skinBase}/gfx/user-menu.jpg`, width: 110 },
  { type: "route", key: "messages" },
  { type: "route", key: "notes" },
  { type: "route", key: "buddy" },
  { type: "route", key: "options" },
  { type: "route", key: "logout" }
];

export function LegacyGameOverview({ status, error, route, buildingsStatus, buildingsError }: LegacyGameOverviewProps) {
  const overview = status?.authenticated ? status.overview : undefined;
  const issue = status && !status.authenticated ? status.issues[0]?.message ?? "Session is invalid." : null;
  const buildings = buildingsStatus?.authenticated ? buildingsStatus.buildings : undefined;
  const buildingsIssue =
    buildingsStatus && !buildingsStatus.authenticated ? buildingsStatus.issues[0]?.message ?? "Session is invalid." : null;
  const contentClassName = route.key === "overview" ? "legacy-content legacy-content-overview" : "legacy-content";

  return (
    <main
      className="legacy-game-shell"
      style={
        {
          "--legacy-body-bg": `url("${skinBase}/img/background.jpg")`,
          "--legacy-title-bg": `url("${skinBase}/img/bg1.gif")`,
          "--legacy-row-bg": `url("${skinBase}/img/bg2.gif")`
        } as React.CSSProperties
      }
    >
      <header className="legacy-header-top" id="header_top">
        {overview ? <LegacyResourceHeader overview={overview} /> : <div className="legacy-header-placeholder">OGame</div>}
      </header>
      <LegacyLeftMenu activeRoute={route} />
      <section className={contentClassName} id="content">
        {error ? <LegacyMessage tone="error" text={error} /> : null}
        {!error && issue ? <LegacyMessage tone="error" text={issue} /> : null}
        {!error && !issue && !overview ? <LegacyMessage tone="neutral" text="Loading overview..." /> : null}
        {route.key === "buildings" && buildingsError ? <LegacyMessage tone="error" text={buildingsError} /> : null}
        {route.key === "buildings" && !buildingsError && buildingsIssue ? (
          <LegacyMessage tone="error" text={buildingsIssue} />
        ) : null}
        {overview && route.key === "overview" ? <OverviewTable overview={overview} /> : null}
        {overview && route.key === "buildings" && !buildings && !buildingsError && !buildingsIssue ? (
          <LegacyMessage tone="neutral" text="Loading buildings..." />
        ) : null}
        {buildings && route.key === "buildings" ? <BuildingsTable buildings={buildings} /> : null}
        {overview && route.key !== "overview" && route.key !== "buildings" ? <MigrationPendingGameTable route={route} /> : null}
      </section>
    </main>
  );
}

function LegacyResourceHeader({ overview }: { overview: GameOverview }) {
  const planet = overview.currentPlanet;
  const resources = [
    { name: "Metal", value: planet.resources.metal, img: `${skinBase}/images/metall.gif` },
    { name: "Crystal", value: planet.resources.crystal, img: `${skinBase}/images/kristall.gif` },
    { name: "Deuterium", value: planet.resources.deuterium, img: `${skinBase}/images/deuterium.gif` },
    { name: "Dark Matter", value: 0, img: `${gameImageBase}/dm_klein_2.jpg` },
    { name: "Energy", value: 0, secondary: 0, img: `${skinBase}/images/energie.gif` }
  ];
  const officers = ["commander", "admiral", "ingenieur", "geologe", "technokrat"];

  return (
    <table className="legacy-header-table">
      <tbody>
        <tr>
          <td className="legacy-header-cell">
            <table className="legacy-header-table">
              <tbody>
                <tr>
                  <td className="legacy-header-cell">
                    <img alt="" height={50} src={planetImagePath(planet, true)} width={50} />
                  </td>
                  <td className="legacy-header-cell">
                    <select
                      aria-label="Planet selector"
                      onChange={(event) => {
                        window.history.pushState({}, "", planetHref(Number(event.currentTarget.value)));
                        window.dispatchEvent(new PopStateEvent("popstate"));
                      }}
                      value={planet.id}
                    >
                      {overview.planetSwitcher.map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name} [{formatCoordinates(item.coordinates)}]
                        </option>
                      ))}
                    </select>
                  </td>
                </tr>
              </tbody>
            </table>
          </td>
          <td className="legacy-header-cell">
            <div className="legacy-header-stack">
              <table className="legacy-resource-table" id="resources">
                <tbody>
                  <tr>
                    {resources.map((resource) => (
                      <td className="legacy-header-cell" key={resource.name}>
                        <img alt="" height={22} src={resource.img} width={42} />
                      </td>
                    ))}
                  </tr>
                  <tr>
                    {resources.map((resource) => (
                      <td className="legacy-header-cell legacy-resource-name" key={resource.name}>
                        {resource.name}
                      </td>
                    ))}
                  </tr>
                  <tr>
                    {resources.map((resource) => (
                      <td className="legacy-header-cell" key={resource.name}>
                        {formatNumber(resource.value)}
                        {resource.secondary !== undefined ? `/${formatNumber(resource.secondary)}` : null}
                      </td>
                    ))}
                  </tr>
                </tbody>
              </table>
              <table className="legacy-officer-table">
                <tbody>
                  <tr>
                    {officers.map((officer) => (
                      <td className="legacy-header-cell" key={officer}>
                        <img alt="" height={30} src={`${gameImageBase}/${officer}_ikon_un.gif`} width={30} />
                      </td>
                    ))}
                  </tr>
                </tbody>
              </table>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  );
}

function LegacyLeftMenu({ activeRoute }: { activeRoute: GameRoute }) {
  return (
    <aside className="legacy-leftmenu" id="leftmenu">
      <div className="legacy-center">
        <div className="legacy-menu" id="menu">
          <p>
            <span className="legacy-nowrap">Universe 1 (v 0.84)</span>
          </p>
          <table>
            <tbody>
              {legacyMenuEntries.map((entry, index) =>
                entry.type === "image" ? (
                  <tr key={`${entry.src}-${index}`}>
                    <td>
                      <img alt="" height={entry.height} src={entry.src} width={entry.width} />
                    </td>
                  </tr>
                ) : (
                  <LegacyMenuRoute activeRoute={activeRoute} entry={entry} key={entry.key} />
                )
              )}
            </tbody>
          </table>
        </div>
      </div>
    </aside>
  );
}

function LegacyMenuRoute({ activeRoute, entry }: { activeRoute: GameRoute; entry: Extract<LegacyMenuEntry, { type: "route" }> }) {
  const route = gameRouteByKey.get(entry.key);
  if (!route) {
    return null;
  }
  return (
    <tr>
      <td>
        <div className="legacy-center">
          <a aria-current={route.key === activeRoute.key ? "page" : undefined} href={gameRouteURL(route.path, window.location.search)}>
            {route.label}
          </a>
        </div>
      </td>
    </tr>
  );
}

function MigrationPendingGameTable({ route }: { route: GameRoute }) {
  return (
    <table className="legacy-overview-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c">{route.label}</td>
        </tr>
        <tr>
          <th>This screen is queued for React and Go migration.</th>
        </tr>
        <tr>
          <th>The authenticated game shell, resource header, and session guard are active.</th>
        </tr>
      </tbody>
    </table>
  );
}

function BuildingsTable({ buildings }: { buildings: GameBuildings }) {
  return (
    <table className="legacy-overview-table legacy-buildings-table" width={530}>
      <tbody>
        {buildings.items.map((item) => (
          <tr data-building-row={item.id} key={item.id}>
            <td className="legacy-l legacy-building-image">
              <a href={gameRouteURL("/game/technology", window.location.search)}>
                <img alt="" height={120} src={`${skinBase}/gebaeude/${item.id}.gif`} width={120} />
              </a>
            </td>
            <td className="legacy-l legacy-building-description">
              <a href={gameRouteURL("/game/technology", window.location.search)}>{item.name}</a>
              {item.level > 0 ? <> (level {item.level})</> : null}
              <br />
              {item.description}
              <br />
              Cost:
              {costParts(item.cost).map((part) => (
                <React.Fragment key={part.name}>
                  {" "}
                  {part.name}: <b>{formatNumber(part.value)}</b>
                </React.Fragment>
              ))}
              <br />
              Duration: {formatLegacyDuration(item.durationSeconds)}
              <br />
            </td>
            <td className="legacy-l legacy-building-action">
              <span className={item.canBuild ? "legacy-build-ok" : "legacy-build-blocked"}>
                {item.action}
                {item.action === "Build level" ? (
                  <>
                    <br />
                    level {item.nextLevel}
                  </>
                ) : null}
              </span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function OverviewTable({ overview }: { overview: GameOverview }) {
  const planet = overview.currentPlanet;
  const moon = overview.planetSwitcher.find(
    (item) =>
      item.type === 0 &&
      item.coordinates.galaxy === planet.coordinates.galaxy &&
      item.coordinates.system === planet.coordinates.system &&
      item.coordinates.position === planet.coordinates.position
  );
  const otherPlanets = overview.planetSwitcher.filter((item) => item.type !== 0 && item.id !== planet.id);

  return (
    <table className="legacy-overview-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c" colSpan={4}>
            <a href="/game/overview" title="Planet menu">
              Planet "{planet.name}"
            </a>{" "}
            ({overview.commander})
          </td>
        </tr>
        <tr>
          <th>Server time</th>
          <th colSpan={3}>{formatLegacyDate(new Date())}</th>
        </tr>
        <tr>
          <td className="legacy-c" colSpan={4}>
            Events
          </td>
        </tr>
        <tr>
          <th colSpan={4}>&nbsp;</th>
        </tr>
        <tr>
          <th>
            {moon ? (
              <>
                {moon.name}
                <br />
                <a href={planetHref(moon.id)}>
                  <img alt="Moon" height={50} src={planetImagePath(moon, true)} width={50} />
                </a>
              </>
            ) : null}
          </th>
          <th colSpan={2}>
            <img alt="" height={200} src={planetImagePath(planet, false)} width={200} />
            <br />
            <div className="legacy-center">free</div>
            <br />
          </th>
          <th className="legacy-s">
            <table className="legacy-planet-list">
              <tbody>
                {otherPlanets.length === 0 ? (
                  <tr>
                    <th>&nbsp;</th>
                  </tr>
                ) : (
                  rowsOfTwo(otherPlanets).map((row, index) => (
                    <tr key={index}>
                      {row.map((item) => (
                        <th key={item.id}>
                          {item.name}
                          <br />
                          <a href={planetHref(item.id)} title={`${item.name} [${formatCoordinates(item.coordinates)}]`}>
                            <img
                              alt=""
                              height={50}
                              src={planetImagePath(item, false)}
                              title={`${item.name} [${formatCoordinates(item.coordinates)}]`}
                              width={50}
                            />
                          </a>
                          <br />
                          <div className="legacy-center">free</div>
                        </th>
                      ))}
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </th>
        </tr>
        <tr>
          <th>Diameter</th>
          <th colSpan={3}>
            {formatLegacyNumber(planet.diameter)} км ({planet.fields} / {planet.maxFields} fields)
          </th>
        </tr>
        <tr>
          <th>Temperature</th>
          <th colSpan={3}>
            approx. {planet.temperature}°C to {planet.temperature + 40}°C
          </th>
        </tr>
        <tr>
          <th>Position</th>
          <th colSpan={3}>
            <a href="/game/overview">[{formatCoordinates(planet.coordinates)}]</a>
          </th>
        </tr>
        <tr>
          <th>Points</th>
          <th colSpan={3}>
            {formatLegacyNumber(overview.score.points)} (Rank{" "}
            <a href="/game/overview">{formatLegacyNumber(overview.score.rank)}</a> of {formatLegacyNumber(overview.score.universePlayers)}
            )
          </th>
        </tr>
      </tbody>
    </table>
  );
}

function LegacyMessage({ tone, text }: { tone: "error" | "neutral"; text: string }) {
  return (
    <table className="legacy-overview-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c">{tone === "error" ? "Error" : "Overview"}</td>
        </tr>
        <tr>
          <th>{text}</th>
        </tr>
      </tbody>
    </table>
  );
}

function rowsOfTwo(items: GamePlanetSummary[]): GamePlanetSummary[][] {
  const rows: GamePlanetSummary[][] = [];
  for (let index = 0; index < items.length; index += 2) {
    rows.push(items.slice(index, index + 2));
  }
  return rows;
}

function planetHref(planetID: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("cp", String(planetID));
  return gameRouteURL("/game/overview", search.toString());
}

function planetImagePath(planet: GamePlanetOverview | GamePlanetSummary, small: boolean): string {
  if (planet.type === 0) {
    return `${skinBase}/planeten/${small ? "small/s_" : ""}mond.jpg`;
  }
  const imageID = (planet.id % 7) + 1;
  const category = planetCategory(planet.coordinates.position);
  const filename = `${category}${String(imageID).padStart(2, "0")}.jpg`;
  return `${skinBase}/planeten/${small ? "small/s_" : ""}${filename}`;
}

function planetCategory(position: number): string {
  if (position <= 3) {
    return "trockenplanet";
  }
  if (position <= 6) {
    return "dschjungelplanet";
  }
  if (position <= 9) {
    return "normaltempplanet";
  }
  if (position <= 12) {
    return "wasserplanet";
  }
  return "eisplanet";
}

function formatCoordinates(coordinates: Coordinates): string {
  return `${coordinates.galaxy}:${coordinates.system}:${coordinates.position}`;
}

function formatNumber(value: number): string {
  return Math.floor(Math.max(0, value)).toLocaleString("en-US");
}

function formatLegacyDuration(totalSeconds: number): string {
  const safe = Math.max(0, Math.floor(totalSeconds));
  const days = Math.floor(safe / 86400);
  const hours = Math.floor(safe / 3600) % 24;
  const minutes = Math.floor(safe / 60) % 60;
  const seconds = safe % 60;
  const parts: string[] = [];
  if (days > 0) {
    parts.push(`${days}d`);
  }
  if (hours > 0 || days > 0) {
    parts.push(`${hours}h`);
  }
  if (minutes > 0 || days > 0) {
    parts.push(`${minutes}m`);
  }
  if (seconds > 0 || parts.length === 0) {
    parts.push(`${seconds}s`);
  }
  return parts.join(" ");
}

function formatLegacyDate(date: Date): string {
  const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
  return `${weekdays[date.getDay()]} ${months[date.getMonth()]} ${date.getDate()} ${date.getHours()}:${String(
    date.getMinutes()
  ).padStart(2, "0")}:${String(date.getSeconds()).padStart(2, "0")}`;
}

function formatLegacyNumber(value: number): string {
  return Math.floor(Math.max(0, value)).toLocaleString("de-DE");
}

function costParts(cost: BuildingCost): { name: string; value: number }[] {
  return [
    { name: "Metal", value: cost.metal },
    { name: "Crystal", value: cost.crystal },
    { name: "Deuterium", value: cost.deuterium },
    { name: "Energy", value: cost.energy }
  ].filter((part) => part.value > 0);
}
