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

export type GameResourcesStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  resources?: GameResourceProduction;
};

export type GameResearchStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  research?: GameResearch;
};

export type GameShipyardStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  shipyard?: GameShipyard;
};

export type GameDefenseStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  defense?: GameDefense;
};

export type GameTechnologyStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  technology?: GameTechnology;
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
  metalCapacity: number;
  crystalCapacity: number;
  deuteriumCapacity: number;
};

type GameBuildings = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  items: GameBuildingItem[];
};

type GameResearch = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  hasLab: boolean;
  items: GameBuildingItem[];
};

type GameShipyard = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  hasShipyard: boolean;
  busy: boolean;
  items: GameShipyardItem[];
};

type GameDefense = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  hasShipyard: boolean;
  busy: boolean;
  items: GameShipyardItem[];
};

type GameTechnology = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  groups: GameTechnologyGroup[];
  details?: GameTechnologyDetails;
};

type GameTechnologyGroup = {
  key: string;
  name: string;
  items: GameTechnologyItem[];
};

type GameTechnologyItem = {
  id: number;
  name: string;
  requirements: GameTechnologyRequirement[];
  detailsAvailable: boolean;
};

type GameTechnologyRequirement = {
  id: number;
  name: string;
  level: number;
  currentLevel: number;
  met: boolean;
};

type GameTechnologyDetails = {
  target: GameTechnologyItem;
  levels: GameTechnologyDetailsLevel[];
};

type GameTechnologyDetailsLevel = {
  step: number;
  requirements: GameTechnologyRequirement[];
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

type GameShipyardItem = {
  id: number;
  name: string;
  description: string;
  count: number;
  cost: BuildingCost;
  durationSeconds: number;
  canBuild: boolean;
  meetsRequirement: boolean;
  maxBuild: number;
  blockedReason: string;
};

type BuildingCost = {
  metal: number;
  crystal: number;
  deuterium: number;
  energy: number;
};

type GameResourceProduction = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  factor: number;
  natural: ResourceProductionValues;
  rows: ResourceProductionRow[];
  storage: ResourceProductionValues;
  totals: ResourceProductionTotals;
};

type ResourceProductionRow = {
  id: number;
  name: string;
  level: number;
  percent: number;
  values: ResourceProductionValues;
};

type ResourceProductionValues = {
  metal: number;
  crystal: number;
  deuterium: number;
  energy: number;
  energyRaw: number;
  energyStored: boolean;
};

type ResourceProductionTotals = {
  hour: ResourceProductionValues;
  day: ResourceProductionValues;
  week: ResourceProductionValues;
};

type LegacyGameOverviewProps = {
  status: GameOverviewStatus | null;
  error: string | null;
  route: GameRoute;
  buildingsStatus: GameBuildingsStatus | null;
  buildingsError: string | null;
  resourcesStatus: GameResourcesStatus | null;
  resourcesError: string | null;
  resourcesPending: boolean;
  onResourcesSubmit: (production: Record<string, string>) => void;
  researchStatus: GameResearchStatus | null;
  researchError: string | null;
  shipyardStatus: GameShipyardStatus | null;
  shipyardError: string | null;
  defenseStatus: GameDefenseStatus | null;
  defenseError: string | null;
  technologyStatus: GameTechnologyStatus | null;
  technologyError: string | null;
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

export function LegacyGameOverview({
  status,
  error,
  route,
  buildingsStatus,
  buildingsError,
  resourcesStatus,
  resourcesError,
  resourcesPending,
  onResourcesSubmit,
  researchStatus,
  researchError,
  shipyardStatus,
  shipyardError,
  defenseStatus,
  defenseError,
  technologyStatus,
  technologyError
}: LegacyGameOverviewProps) {
  const overview = status?.authenticated ? status.overview : undefined;
  const issue = status && !status.authenticated ? status.issues[0]?.message ?? "Session is invalid." : null;
  const buildings = buildingsStatus?.authenticated ? buildingsStatus.buildings : undefined;
  const buildingsIssue =
    buildingsStatus && !buildingsStatus.authenticated ? buildingsStatus.issues[0]?.message ?? "Session is invalid." : null;
  const resources = resourcesStatus?.authenticated ? resourcesStatus.resources : undefined;
  const resourcesIssue =
    resourcesStatus && !resourcesStatus.authenticated ? resourcesStatus.issues[0]?.message ?? "Session is invalid." : null;
  const research = researchStatus?.authenticated ? researchStatus.research : undefined;
  const researchIssue =
    researchStatus && !researchStatus.authenticated ? researchStatus.issues[0]?.message ?? "Session is invalid." : null;
  const shipyard = shipyardStatus?.authenticated ? shipyardStatus.shipyard : undefined;
  const shipyardIssue =
    shipyardStatus && !shipyardStatus.authenticated ? shipyardStatus.issues[0]?.message ?? "Session is invalid." : null;
  const defense = defenseStatus?.authenticated ? defenseStatus.defense : undefined;
  const defenseIssue =
    defenseStatus && !defenseStatus.authenticated ? defenseStatus.issues[0]?.message ?? "Session is invalid." : null;
  const technology = technologyStatus?.authenticated ? technologyStatus.technology : undefined;
  const technologyIssue =
    technologyStatus && !technologyStatus.authenticated ? technologyStatus.issues[0]?.message ?? "Session is invalid." : null;
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
        {route.key === "resources" && resourcesError ? <LegacyMessage tone="error" text={resourcesError} /> : null}
        {route.key === "resources" && !resourcesError && resourcesIssue ? (
          <LegacyMessage tone="error" text={resourcesIssue} />
        ) : null}
        {route.key === "research" && researchError ? <LegacyMessage tone="error" text={researchError} /> : null}
        {route.key === "research" && !researchError && researchIssue ? (
          <LegacyMessage tone="error" text={researchIssue} />
        ) : null}
        {route.key === "shipyard" && shipyardError ? <LegacyMessage tone="error" text={shipyardError} /> : null}
        {route.key === "shipyard" && !shipyardError && shipyardIssue ? (
          <LegacyMessage tone="error" text={shipyardIssue} />
        ) : null}
        {route.key === "defense" && defenseError ? <LegacyMessage tone="error" text={defenseError} /> : null}
        {route.key === "defense" && !defenseError && defenseIssue ? <LegacyMessage tone="error" text={defenseIssue} /> : null}
        {route.key === "technology" && technologyError ? <LegacyMessage tone="error" text={technologyError} /> : null}
        {route.key === "technology" && !technologyError && technologyIssue ? (
          <LegacyMessage tone="error" text={technologyIssue} />
        ) : null}
        {overview && route.key === "overview" ? <OverviewTable overview={overview} /> : null}
        {overview && route.key === "buildings" && !buildings && !buildingsError && !buildingsIssue ? (
          <LegacyMessage tone="neutral" text="Loading buildings..." />
        ) : null}
        {buildings && route.key === "buildings" ? <BuildingsTable buildings={buildings} /> : null}
        {overview && route.key === "resources" && !resources && !resourcesError && !resourcesIssue ? (
          <LegacyMessage tone="neutral" text="Loading resources..." />
        ) : null}
        {resources && route.key === "resources" ? (
          <ResourcesTable onSubmit={onResourcesSubmit} pending={resourcesPending} resources={resources} />
        ) : null}
        {overview && route.key === "research" && !research && !researchError && !researchIssue ? (
          <LegacyMessage tone="neutral" text="Loading research..." />
        ) : null}
        {research && route.key === "research" ? <ResearchTable research={research} /> : null}
        {overview && route.key === "shipyard" && !shipyard && !shipyardError && !shipyardIssue ? (
          <LegacyMessage tone="neutral" text="Loading shipyard..." />
        ) : null}
        {shipyard && route.key === "shipyard" ? <ShipyardTable shipyard={shipyard} /> : null}
        {overview && route.key === "defense" && !defense && !defenseError && !defenseIssue ? (
          <LegacyMessage tone="neutral" text="Loading defense..." />
        ) : null}
        {defense && route.key === "defense" ? <DefenseTable defense={defense} /> : null}
        {overview && route.key === "technology" && !technology && !technologyError && !technologyIssue ? (
          <LegacyMessage tone="neutral" text="Loading technology..." />
        ) : null}
        {technology && route.key === "technology" ? <TechnologyTable technology={technology} /> : null}
        {overview &&
        route.key !== "overview" &&
        route.key !== "buildings" &&
        route.key !== "resources" &&
        route.key !== "research" &&
        route.key !== "shipyard" &&
        route.key !== "defense" &&
        route.key !== "technology" ? (
          <MigrationPendingGameTable route={route} />
        ) : null}
      </section>
    </main>
  );
}

function LegacyResourceHeader({ overview }: { overview: GameOverview }) {
  const planet = overview.currentPlanet;
  const resources = [
    { name: "Metal", value: planet.resources.metal, capacity: planet.resources.metalCapacity, img: `${skinBase}/images/metall.gif` },
    { name: "Crystal", value: planet.resources.crystal, capacity: planet.resources.crystalCapacity, img: `${skinBase}/images/kristall.gif` },
    {
      name: "Deuterium",
      value: planet.resources.deuterium,
      capacity: planet.resources.deuteriumCapacity,
      img: `${skinBase}/images/deuterium.gif`
    },
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
                        <span style={resource.capacity !== undefined && resource.value >= resource.capacity ? { color: "#ff0000" } : undefined}>
                          {formatLegacyNumber(resource.value)}
                        </span>
                        {resource.secondary !== undefined ? `/${formatLegacyNumber(resource.secondary)}` : null}
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
                  {part.name}: <b>{formatLegacyNumber(part.value)}</b>
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

function ResearchTable({ research }: { research: GameResearch }) {
  if (!research.hasLab) {
    return (
      <table className="legacy-overview-table legacy-research-table" width={530}>
        <tbody>
          <tr>
            <td className="legacy-c">Research</td>
          </tr>
          <tr>
            <th>In order to do this, you need to build a research lab!</th>
          </tr>
        </tbody>
      </table>
    );
  }
  return (
    <table className="legacy-overview-table legacy-research-table" width={530}>
      <tbody>
        <tr>
          <td className="legacy-l" colSpan={2}>
            Description
          </td>
          <td className="legacy-l">
            <b>Qty.</b>
          </td>
        </tr>
        {research.items.map((item) => (
          <tr data-research-row={item.id} key={item.id}>
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
                  {part.name}: <b>{formatLegacyNumber(part.value)}</b>
                </React.Fragment>
              ))}
              <br />
              Duration: {formatLegacyDuration(item.durationSeconds)}
              <br />
            </td>
            <td className="legacy-l legacy-building-action">
              <span className={item.canBuild ? "legacy-build-ok" : "legacy-build-blocked"}>
                {item.action}
                {item.action === "Research level" ? (
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

function ShipyardTable({ shipyard }: { shipyard: GameShipyard }) {
  if (!shipyard.hasShipyard) {
    return (
      <table className="legacy-overview-table legacy-shipyard-table" width={530}>
        <tbody>
          <tr>
            <td className="legacy-l" colSpan={2}>
              Description
            </td>
            <td className="legacy-l">
              <b>Qty.</b>
            </td>
          </tr>
          <tr>
            <td className="legacy-c" colSpan={3}>
              In order to do that, you need to build a shipyard!
            </td>
          </tr>
        </tbody>
      </table>
    );
  }
  return (
    <form className="legacy-shipyard-form" onSubmit={(event) => event.preventDefault()}>
      <table className="legacy-overview-table legacy-shipyard-table" width={530}>
        <tbody>
          <tr>
            <td className="legacy-l" colSpan={2}>
              Description
            </td>
            <td className="legacy-l">
              <b>Qty.</b>
            </td>
          </tr>
          {shipyard.items.map((item) => (
            <tr data-shipyard-row={item.id} key={item.id}>
              <td className="legacy-l legacy-building-image">
                <a href={gameRouteURL("/game/technology", window.location.search)}>
                  <img alt="" height={120} src={`${skinBase}/gebaeude/${item.id}.gif`} width={120} />
                </a>
              </td>
              <td className="legacy-l legacy-building-description">
                <a href={gameRouteURL("/game/technology", window.location.search)}>{item.name}</a>
                {item.count > 0 ? <> (in stock {item.count})</> : null}
                <br />
                {item.description}
                <br />
                Cost:
                {costParts(item.cost).map((part) => (
                  <React.Fragment key={part.name}>
                    {" "}
                    {part.name}: <b>{formatLegacyNumber(part.value)}</b>
                  </React.Fragment>
                ))}
                <br />
                Duration: {formatLegacyDuration(item.durationSeconds)}
                <br />
              </td>
              <td className="legacy-l legacy-building-action">
                {!item.meetsRequirement ? <span className="legacy-build-blocked">impossibly</span> : null}
                {item.meetsRequirement && item.canBuild ? (
                  <>
                    <input aria-label={item.name} defaultValue={0} maxLength={6} name={`fmenge[${item.id}]`} size={6} type="text" />
                    {item.maxBuild > 0 ? (
                      <>
                        <br />
                        <a href="#max" onClick={(event) => event.preventDefault()}>
                          (max. {item.maxBuild})
                        </a>
                      </>
                    ) : null}
                  </>
                ) : null}
              </td>
            </tr>
          ))}
          <tr>
            <td className="legacy-c" colSpan={2}>
              <input type="submit" value="Build" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function DefenseTable({ defense }: { defense: GameDefense }) {
  if (!defense.hasShipyard) {
    return (
      <table className="legacy-overview-table legacy-defense-table" width={530}>
        <tbody>
          <tr>
            <td className="legacy-l" colSpan={2}>
              Description
            </td>
            <td className="legacy-l">
              <b>Qty.</b>
            </td>
          </tr>
          <tr>
            <td className="legacy-c" colSpan={3}>
              In order to do that, you need to build a shipyard!
            </td>
          </tr>
        </tbody>
      </table>
    );
  }
  return (
    <form className="legacy-defense-form" onSubmit={(event) => event.preventDefault()}>
      <table className="legacy-overview-table legacy-defense-table" width={530}>
        <tbody>
          <tr>
            <td className="legacy-l" colSpan={2}>
              Description
            </td>
            <td className="legacy-l">
              <b>Qty.</b>
            </td>
          </tr>
          {defense.items.map((item) => (
            <tr data-defense-row={item.id} key={item.id}>
              <td className="legacy-l legacy-building-image">
                <a href={gameRouteURL("/game/technology", window.location.search)}>
                  <img alt="" height={120} src={`${skinBase}/gebaeude/${item.id}.gif`} width={120} />
                </a>
              </td>
              <td className="legacy-l legacy-building-description">
                <a href={gameRouteURL("/game/technology", window.location.search)}>{item.name}</a>
                {item.count > 0 ? <> (in stock {item.count})</> : null}
                <br />
                {item.description}
                <br />
                Cost:
                {costParts(item.cost).map((part) => (
                  <React.Fragment key={part.name}>
                    {" "}
                    {part.name}: <b>{formatLegacyNumber(part.value)}</b>
                  </React.Fragment>
                ))}
                <br />
                Duration: {formatLegacyDuration(item.durationSeconds)}
                <br />
              </td>
              <td className="legacy-l legacy-building-action">
                {item.blockedReason ? <span className="legacy-build-blocked">{item.blockedReason}</span> : null}
                {!item.blockedReason && item.canBuild ? (
                  <>
                    <input aria-label={item.name} defaultValue={0} maxLength={6} name={`fmenge[${item.id}]`} size={6} type="text" />
                    {item.maxBuild > 0 ? (
                      <>
                        <br />
                        <a href="#max" onClick={(event) => event.preventDefault()}>
                          (max. {item.maxBuild})
                        </a>
                      </>
                    ) : null}
                  </>
                ) : null}
              </td>
            </tr>
          ))}
          <tr>
            <td className="legacy-c" colSpan={2}>
              <input type="submit" value="Build" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function TechnologyTable({ technology }: { technology: GameTechnology }) {
  if (technology.details) {
    return <TechnologyDetailsTable details={technology.details} />;
  }
  return (
    <div className="legacy-center">
      <table className="legacy-overview-table legacy-technology-table" width={470}>
        <tbody>
          {technology.groups.map((group) => (
            <React.Fragment key={group.key}>
              <tr>
                <td className="legacy-c">{group.name}</td>
                <td className="legacy-c">Requirements</td>
              </tr>
              {group.items.map((item) => (
                <tr data-technology-row={item.id} key={item.id}>
                  <td className="legacy-l">
                    <table border={0} cellPadding={0} cellSpacing={0} className="legacy-technology-name-table" width="100%">
                      <tbody>
                        <tr>
                          <td align="left">
                            <a className="legacy-technology-name-link" href={technologyInfoURL(item.id)}>
                              {item.name}
                            </a>
                          </td>
                          <td align="right">
                            {item.detailsAvailable ? <a href={technologyDetailURL(item.id)}>[i]</a> : "\u00a0"}
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </td>
                  <td className="legacy-l">
                    {item.requirements.map((requirement) => (
                      <React.Fragment key={requirement.id}>
                        <span style={{ color: requirement.met ? "#00ff00" : "#ff0000" }}>
                          {requirement.name} (level {requirement.level})
                        </span>
                        <br />
                      </React.Fragment>
                    ))}
                  </td>
                </tr>
              ))}
            </React.Fragment>
          ))}
        </tbody>
      </table>
      <br />
      <br />
      <br />
      <br />
    </div>
  );
}

function TechnologyDetailsTable({ details }: { details: GameTechnologyDetails }) {
  return (
    <div className="legacy-center">
      <table className="legacy-overview-table legacy-technology-details-table" width={270}>
        <tbody>
          <tr>
            <td align="center" className="legacy-c" style={{ whiteSpace: "nowrap" }}>
              Building conditions for{" "}
              <a className="legacy-technology-detail-target" href={technologyInfoURL(details.target.id)}>
                &apos;{details.target.name}&apos;
              </a>
            </td>
          </tr>
          {details.levels.length === 0 ? (
            <tr>
              <td align="center" className="legacy-l">
                No conditions
              </td>
            </tr>
          ) : (
            details.levels.map((level) => (
              <React.Fragment key={level.step}>
                <tr>
                  <td className="legacy-c">{level.step}</td>
                </tr>
                {level.requirements.map((requirement) => (
                  <tr data-technology-detail-row={requirement.id} key={`${level.step}-${requirement.id}`}>
                    <td align="center" className="legacy-l">
                      <table border={0} className="legacy-technology-name-table" width="100%">
                        <tbody>
                          <tr>
                            <td align="left">
                              <span style={{ color: requirement.met ? "#00ff00" : "#ff0000" }}>
                                {" "}
                                {requirement.name} (level {requirement.level}){" "}
                              </span>
                            </td>
                            <td align="right">
                              <a href={technologyDetailURL(requirement.id)}>[i]</a>
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </td>
                  </tr>
                ))}
              </React.Fragment>
            ))
          )}
        </tbody>
      </table>
      <br />
      <br />
      <br />
      <br />
    </div>
  );
}

const resourceColumns: { key: keyof Pick<ResourceProductionValues, "metal" | "crystal" | "deuterium" | "energy">; label: string }[] = [
  { key: "metal", label: "Metal" },
  { key: "crystal", label: "Crystal" },
  { key: "deuterium", label: "Deuterium" },
  { key: "energy", label: "Energy" }
];

function ResourcesTable({
  resources,
  pending,
  onSubmit
}: {
  resources: GameResourceProduction;
  pending: boolean;
  onSubmit: (production: Record<string, string>) => void;
}) {
  return (
    <form
      className="legacy-resources-form"
      id="ressourcen"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(resourceProductionFormValues(event.currentTarget));
      }}
    >
      <div className="legacy-center">
        <br />
        <br />
        Production factor: {formatProductionFactor(resources.factor)}
        <table className="legacy-overview-table legacy-resources-table" width={550}>
          <tbody>
            <tr>
              <td className="legacy-c" colSpan={6}>
                Resource settings on planet &quot;{resources.currentPlanet.name}&quot;
              </td>
            </tr>
            <tr>
              <th colSpan={2}></th>
              {resourceColumns.map((column) => (
                <th key={column.key}>{column.label}</th>
              ))}
            </tr>
            <tr>
              <th colSpan={2}>Basic Income</th>
              {resourceColumns.map((column) => (
                <td className="legacy-k" key={column.key}>
                  {formatLegacyPlainNumber(resourceValue(resources.natural, column.key))}
                </td>
              ))}
            </tr>
            {resources.rows.map((row) => (
              <tr data-resource-row={row.id} key={row.id}>
                <th>
                  {row.name} ({row.id === 212 ? "Amount" : "level"} {row.level})
                </th>
                <th>&nbsp;</th>
                {resourceColumns.map((column) => (
                  <ResourceProductionCell column={column.key} key={column.key} values={row.values} />
                ))}
                <th>
                  <select aria-label={`${row.name} production`} defaultValue={row.percent} name={`last${row.id}`} size={1}>
                    {productionPercentOptions().map((percent) => (
                      <option key={percent} value={percent}>
                        {percent}%
                      </option>
                    ))}
                  </select>
                </th>
              </tr>
            ))}
            <tr>
              <th colSpan={2}>Storage capacity</th>
              {resourceColumns.map((column) => (
                <td className="legacy-k" key={column.key}>
                  <span style={{ color: "#00ff00" }}>{formatStorageValue(resources.storage, column.key)}</span>
                </td>
              ))}
              <td className="legacy-k">
                <input disabled={pending} name="action" type="submit" value="Calculate" />
              </td>
            </tr>
            <tr>
              <th colSpan={6} style={{ height: 4 }}></th>
            </tr>
            <ResourceTotalRow label="Total per hour:" values={resources.totals.hour} />
            <ResourceTotalRow label="Total per day:" values={resources.totals.day} />
            <ResourceTotalRow label="Total per week:" values={resources.totals.week} />
          </tbody>
        </table>
        <br />
        <br />
        <br />
        <br />
      </div>
    </form>
  );
}

function resourceProductionFormValues(form: HTMLFormElement): Record<string, string> {
  const formData = new FormData(form);
  const production: Record<string, string> = {};
  for (const [key, value] of formData.entries()) {
    if (!key.startsWith("last")) {
      continue;
    }
    production[key.slice(4)] = String(value);
  }
  return production;
}

function ResourceProductionCell({
  column,
  values
}: {
  column: keyof Pick<ResourceProductionValues, "metal" | "crystal" | "deuterium" | "energy">;
  values: ResourceProductionValues;
}) {
  const value = resourceValue(values, column);
  const raw = column === "energy" ? values.energyRaw : value;
  const text =
    column === "energy" && raw < 0
      ? `${formatLegacyPlainNumber(Math.abs(value))}/${formatLegacyPlainNumber(Math.abs(raw))}`
      : formatLegacySignedNumber(value);
  const color = raw > 0 || value > 0 ? "#00FF00" : raw < 0 || value < 0 ? "#FF0000" : "#FFFFFF";
  return (
    <th>
      <span style={{ color }}>{text}</span>
    </th>
  );
}

function ResourceTotalRow({ label, values }: { label: string; values: ResourceProductionValues }) {
  return (
    <tr>
      <th colSpan={2}>{label}</th>
      {resourceColumns.map((column) => {
        const value = resourceValue(values, column.key);
        return (
          <td className="legacy-k" key={column.key}>
            <span style={{ color: value > 0 ? "#00ff00" : "#ff0000" }}>{formatLegacySignedNumber(value)}</span>
          </td>
        );
      })}
    </tr>
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

function technologyInfoURL(itemID: number): string {
  const search = new URLSearchParams(window.location.search);
  search.delete("tid");
  search.set("gid", String(itemID));
  return gameRouteURL("/game/technology", search.toString());
}

function technologyDetailURL(itemID: number): string {
  const search = new URLSearchParams(window.location.search);
  search.delete("gid");
  search.set("tid", String(itemID));
  return gameRouteURL("/game/technology", search.toString());
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

function formatLegacyPlainNumber(value: number): string {
  return Math.round(Math.max(0, value)).toLocaleString("de-DE");
}

function formatLegacySignedNumber(value: number): string {
  const rounded = Math.round(value);
  const absolute = Math.abs(rounded).toLocaleString("de-DE");
  return rounded < 0 ? `-${absolute}` : absolute;
}

function formatProductionFactor(value: number): string {
  return (Math.round(value * 100) / 100).toLocaleString("en-US", { maximumFractionDigits: 2 });
}

function formatStorageValue(
  values: ResourceProductionValues,
  column: keyof Pick<ResourceProductionValues, "metal" | "crystal" | "deuterium" | "energy">
): string {
  if (column === "energy" && !values.energyStored) {
    return "-";
  }
  return `${formatLegacyPlainNumber(resourceValue(values, column) / 1000)}k`;
}

function resourceValue(
  values: ResourceProductionValues,
  column: keyof Pick<ResourceProductionValues, "metal" | "crystal" | "deuterium" | "energy">
): number {
  return values[column];
}

function productionPercentOptions(): number[] {
  const options: number[] = [];
  for (let value = 100; value >= 0; value -= 10) {
    options.push(value);
  }
  return options;
}

function costParts(cost: BuildingCost): { name: string; value: number }[] {
  return [
    { name: "Metal", value: cost.metal },
    { name: "Crystal", value: cost.crystal },
    { name: "Deuterium", value: cost.deuterium },
    { name: "Energy", value: cost.energy }
  ].filter((part) => part.value > 0);
}
