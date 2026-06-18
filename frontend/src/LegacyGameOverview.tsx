import React from "react";
import { gamePlanetSwitchURL, gameRouteURL, gameRoutes, type GameRoute } from "./gameRoutes";

export type GameOverviewStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
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

export type GameFleetStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  fleet?: GameFleet;
};

export type GameGalaxyStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  galaxy?: GameGalaxy;
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

export type GameStatisticsStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  statistics?: GameStatistics;
};

export type GameSearchStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  search?: GameSearch;
};

export type GameNotesStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  notes?: GameNotes;
};

export type GameLogoutStatus = {
  loggedOut: boolean;
  redirectTo: string;
};

type GameOverview = {
  commander: string;
  messages?: string[];
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

type GameFleet = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  slots: {
    used: number;
    max: number;
    baseMax: number;
    admiral: boolean;
  };
  expeditions: {
    used: number;
    max: number;
  };
  missions: GameFleetMission[];
  ships: GameFleetShip[];
};

type GameGalaxy = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  coordinates: Coordinates;
  bounds: {
    galaxies: number;
    systems: number;
  };
  rows: GameGalaxyRow[];
  populated: number;
  slots: {
    used: number;
    max: number;
    baseMax: number;
    admiral: boolean;
  };
  extra: {
    commander: boolean;
    spyProbes: number;
    recyclers: number;
    missiles: number;
    slots: {
      used: number;
      max: number;
      baseMax: number;
      admiral: boolean;
    };
  };
  notEnoughDeuterium: boolean;
  remoteSystemCostDue: boolean;
};

type GameGalaxyRow = {
  position: number;
  planet?: GameGalaxyPlanet;
  moon?: GameGalaxyPlanet;
  debris?: GameGalaxyDebris;
};

type GameGalaxyPlanet = {
  id: number;
  name: string;
  displayName: string;
  type: number;
  coordinates: Coordinates;
  diameter: number;
  temperature: number;
  activityText: string;
  destroyed: boolean;
  abandoned: boolean;
  own: boolean;
  player?: GameGalaxyPlayer;
  alliance?: { id: number; tag: string };
  actions: GameGalaxyActions;
};

type GameGalaxyPlayer = {
  id: number;
  name: string;
  rank: number;
  status: string;
  statusClass: string;
  suffixes: { text: string; class: string }[];
  own: boolean;
};

type GameGalaxyDebris = {
  id: number;
  metal: number;
  crystal: number;
  harvesters: number;
  visible: boolean;
};

type GameGalaxyActions = {
  deploy: boolean;
  transport: boolean;
  spy: boolean;
  message: boolean;
  buddy: boolean;
  missile: boolean;
  attack: boolean;
  defend: boolean;
  destroy: boolean;
  recycle: boolean;
};

type GameFleetMission = {
  id: number;
  mission: number;
  missionName: string;
  stateTitle: string;
  stateShort: string;
  ships: { id: number; name: string; count: number }[];
  totalShips: number;
  origin: Coordinates;
  target: Coordinates;
  targetType: number;
  targetOwnerName: string;
  departureAt: number;
  arrivalAt: number;
  canRecall: boolean;
  canCreateUnion: boolean;
};

type GameStatistics = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  who: string;
  type: string;
  start: number;
  total: number;
  generatedAt: number;
  rows: GameStatisticsRow[];
};

type GameStatisticsRow = {
  place: number;
  previousPlace: number;
  delta: number;
  score: number;
  displayScore: number;
  members: number;
  perMember: number;
  scoreDate: number;
  player: { id: number; name: string };
  alliance?: { id: number; tag: string };
  coordinates: Coordinates;
  own: boolean;
};

type GameSearch = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  type: string;
  text: string;
  message: string;
  playerRows: GameSearchPlayerRow[];
  allianceRows: GameSearchAllianceRow[];
};

type GameSearchPlayerRow = {
  playerId: number;
  playerName: string;
  alliance?: { id: number; tag: string };
  planetId: number;
  planetName: string;
  coordinates: Coordinates;
  place: number;
  own: boolean;
  sameAlliance: boolean;
};

type GameSearchAllianceRow = {
  allianceId: number;
  tag: string;
  name: string;
  members: number;
  score: number;
  displayScore: number;
  own: boolean;
};

type GameNotes = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  action: "list" | "create" | "edit";
  rows: GameNote[];
  editNote?: GameNote;
};

type GameNote = {
  id: number;
  subject: string;
  text: string;
  textSize: number;
  priority: number;
  priorityColor: string;
  date: number;
};

type GameFleetShip = {
  id: number;
  name: string;
  count: number;
  speed: number;
  cargo: number;
  consumption: number;
  selectable: boolean;
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
  overviewPending: boolean;
  onPlanetDelete: (password: string, deleteID: number) => void;
  onPlanetRename: (name: string) => void;
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
  fleetStatus: GameFleetStatus | null;
  fleetError: string | null;
  galaxyStatus: GameGalaxyStatus | null;
  galaxyError: string | null;
  defenseStatus: GameDefenseStatus | null;
  defenseError: string | null;
  technologyStatus: GameTechnologyStatus | null;
  technologyError: string | null;
  statisticsStatus: GameStatisticsStatus | null;
  statisticsError: string | null;
  searchStatus: GameSearchStatus | null;
  searchError: string | null;
  notesStatus: GameNotesStatus | null;
  notesError: string | null;
  notesPending: boolean;
  onNotesCreate: (draft: GameNoteDraft) => void;
  onNotesUpdate: (noteID: number, draft: GameNoteDraft) => void;
  onNotesDelete: (noteIDs: number[]) => void;
  logoutStatus: GameLogoutStatus | null;
  logoutError: string | null;
};

export type GameNoteDraft = {
  subject: string;
  text: string;
  priority: number;
};

type LegacyMenuEntry =
  | { type: "image"; height: number; src: string; width: number }
  | { type: "route"; key: GameRoute["key"] };

const skinBase = "/public-assets/evolution";
const gameImageBase = "/public-assets/game-img";
const GalaxyDeuteriumCostText = "10";
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
  overviewPending,
  onPlanetDelete,
  onPlanetRename,
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
  fleetStatus,
  fleetError,
  galaxyStatus,
  galaxyError,
  defenseStatus,
  defenseError,
  technologyStatus,
  technologyError,
  statisticsStatus,
  statisticsError,
  searchStatus,
  searchError,
  notesStatus,
  notesError,
  notesPending,
  onNotesCreate,
  onNotesUpdate,
  onNotesDelete,
  logoutStatus,
  logoutError
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
  const fleet = fleetStatus?.authenticated ? fleetStatus.fleet : undefined;
  const fleetIssue = fleetStatus && !fleetStatus.authenticated ? fleetStatus.issues[0]?.message ?? "Session is invalid." : null;
  const galaxy = galaxyStatus?.authenticated ? galaxyStatus.galaxy : undefined;
  const galaxyIssue =
    galaxyStatus && !galaxyStatus.authenticated ? galaxyStatus.issues[0]?.message ?? "Session is invalid." : null;
  const defense = defenseStatus?.authenticated ? defenseStatus.defense : undefined;
  const defenseIssue =
    defenseStatus && !defenseStatus.authenticated ? defenseStatus.issues[0]?.message ?? "Session is invalid." : null;
  const technology = technologyStatus?.authenticated ? technologyStatus.technology : undefined;
  const technologyIssue =
    technologyStatus && !technologyStatus.authenticated ? technologyStatus.issues[0]?.message ?? "Session is invalid." : null;
  const statistics = statisticsStatus?.authenticated ? statisticsStatus.statistics : undefined;
  const statisticsIssue =
    statisticsStatus && !statisticsStatus.authenticated ? statisticsStatus.issues[0]?.message ?? "Session is invalid." : null;
  const search = searchStatus?.authenticated ? searchStatus.search : undefined;
  const searchIssue = searchStatus && !searchStatus.authenticated ? searchStatus.issues[0]?.message ?? "Session is invalid." : null;
  const notes = notesStatus?.authenticated ? notesStatus.notes : undefined;
  const notesIssue = notesStatus && !notesStatus.authenticated ? notesStatus.issues[0]?.message ?? "Session is invalid." : null;
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
      {overview && route.key === "overview" && overview.messages && overview.messages.length > 0 ? (
        <LegacyPageMessage messages={overview.messages} />
      ) : null}
      <section className={contentClassName} id="content">
        {error ? <LegacyMessage tone="error" text={error} /> : null}
        {!error && issue ? <LegacyMessage tone="error" text={issue} /> : null}
        {!error && !issue && !overview && route.key !== "logout" ? <LegacyMessage tone="neutral" text="Loading overview..." /> : null}
        {route.key === "logout" ? <LogoutTable error={logoutError} status={logoutStatus} /> : null}
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
        {route.key === "fleet" && fleetError ? <LegacyMessage tone="error" text={fleetError} /> : null}
        {route.key === "fleet" && !fleetError && fleetIssue ? <LegacyMessage tone="error" text={fleetIssue} /> : null}
        {route.key === "galaxy" && galaxyError ? <LegacyMessage tone="error" text={galaxyError} /> : null}
        {route.key === "galaxy" && !galaxyError && galaxyIssue ? <LegacyMessage tone="error" text={galaxyIssue} /> : null}
        {route.key === "defense" && defenseError ? <LegacyMessage tone="error" text={defenseError} /> : null}
        {route.key === "defense" && !defenseError && defenseIssue ? <LegacyMessage tone="error" text={defenseIssue} /> : null}
        {route.key === "technology" && technologyError ? <LegacyMessage tone="error" text={technologyError} /> : null}
        {route.key === "technology" && !technologyError && technologyIssue ? (
          <LegacyMessage tone="error" text={technologyIssue} />
        ) : null}
        {route.key === "statistics" && statisticsError ? <LegacyMessage tone="error" text={statisticsError} /> : null}
        {route.key === "statistics" && !statisticsError && statisticsIssue ? (
          <LegacyMessage tone="error" text={statisticsIssue} />
        ) : null}
        {route.key === "search" && searchError ? <LegacyMessage tone="error" text={searchError} /> : null}
        {route.key === "search" && !searchError && searchIssue ? <LegacyMessage tone="error" text={searchIssue} /> : null}
        {route.key === "notes" && notesError ? <LegacyMessage tone="error" text={notesError} /> : null}
        {route.key === "notes" && !notesError && notesIssue ? <LegacyMessage tone="error" text={notesIssue} /> : null}
        {overview && route.key === "overview" ? <OverviewTable overview={overview} /> : null}
        {overview && route.key === "renamePlanet" ? (
          <RenamePlanetTable onDelete={onPlanetDelete} onRename={onPlanetRename} overview={overview} pending={overviewPending} />
        ) : null}
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
        {overview && route.key === "fleet" && !fleet && !fleetError && !fleetIssue ? (
          <LegacyMessage tone="neutral" text="Loading fleet..." />
        ) : null}
        {fleet && route.key === "fleet" ? <FleetTable fleet={fleet} /> : null}
        {overview && route.key === "galaxy" && !galaxy && !galaxyError && !galaxyIssue ? (
          <LegacyMessage tone="neutral" text="Loading galaxy..." />
        ) : null}
        {galaxy && route.key === "galaxy" ? <GalaxyTable galaxy={galaxy} /> : null}
        {overview && route.key === "defense" && !defense && !defenseError && !defenseIssue ? (
          <LegacyMessage tone="neutral" text="Loading defense..." />
        ) : null}
        {defense && route.key === "defense" ? <DefenseTable defense={defense} /> : null}
        {overview && route.key === "technology" && !technology && !technologyError && !technologyIssue ? (
          <LegacyMessage tone="neutral" text="Loading technology..." />
        ) : null}
        {technology && route.key === "technology" ? <TechnologyTable technology={technology} /> : null}
        {overview && route.key === "statistics" && !statistics && !statisticsError && !statisticsIssue ? (
          <LegacyMessage tone="neutral" text="Loading statistics..." />
        ) : null}
        {statistics && route.key === "statistics" ? <StatisticsTable statistics={statistics} /> : null}
        {overview && route.key === "search" && !search && !searchError && !searchIssue ? (
          <LegacyMessage tone="neutral" text="Loading search..." />
        ) : null}
        {search && route.key === "search" ? <SearchTable search={search} /> : null}
        {overview && route.key === "notes" && !notes && !notesError && !notesIssue ? (
          <LegacyMessage tone="neutral" text="Loading notes..." />
        ) : null}
        {notes && route.key === "notes" ? (
          <NotesTable
            notes={notes}
            onCreate={onNotesCreate}
            onDelete={onNotesDelete}
            onUpdate={onNotesUpdate}
            pending={notesPending}
          />
        ) : null}
        {overview &&
        route.key !== "overview" &&
        route.key !== "renamePlanet" &&
        route.key !== "buildings" &&
        route.key !== "resources" &&
        route.key !== "research" &&
        route.key !== "shipyard" &&
        route.key !== "fleet" &&
        route.key !== "galaxy" &&
        route.key !== "defense" &&
        route.key !== "technology" &&
        route.key !== "statistics" &&
        route.key !== "search" &&
        route.key !== "notes" &&
        route.key !== "logout" ? (
          <MigrationPendingGameTable route={route} />
        ) : null}
      </section>
    </main>
  );
}

function LegacyPageMessage({ messages }: { messages: string[] }) {
  return (
    <div className="legacy-page-messagebox" id="messagebox">
      <center>
        {messages.map((message, index) => (
          <React.Fragment key={`${message}-${index}`}>
            {message}
            <br />
          </React.Fragment>
        ))}
      </center>
    </div>
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

function LogoutTable({ error, status }: { error: string | null; status: GameLogoutStatus | null }) {
  const text = error ? error : status ? "See you soon!!" : "Logging out...";
  return (
    <table className="legacy-overview-table legacy-logout-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c">Logout</td>
        </tr>
        <tr>
          <th>{text}</th>
        </tr>
      </tbody>
    </table>
  );
}

function StatisticsTable({ statistics }: { statistics: GameStatistics }) {
  const windows = statisticsWindows(statistics.total, statistics.start);
  return (
    <>
      <form
        action={gameRouteURL("/game/statistics", window.location.search)}
        method="get"
        onSubmit={(event) => {
          event.preventDefault();
          const form = new FormData(event.currentTarget);
          const query = new URLSearchParams(window.location.search);
          query.delete("tid");
          query.set("who", String(form.get("who") ?? "player"));
          query.set("type", String(form.get("type") ?? "ressources"));
          query.set("start", String(form.get("start") ?? "-1"));
          window.history.pushState({}, "", gameRouteURL("/game/statistics", query.toString()));
          window.dispatchEvent(new PopStateEvent("popstate"));
        }}
      >
        <table className="legacy-overview-table legacy-statistics-head-table" width={525}>
          <tbody>
            <tr>
              <td className="legacy-c">Statistics (as of: {formatLegacyDateTime(statistics.generatedAt)})</td>
            </tr>
            <tr>
              <th>
                What kind of&nbsp;
                <select name="who" defaultValue={statistics.who}>
                  <option value="player">Player</option>
                  <option value="ally">Alliance</option>
                </select>
                &nbsp;by&nbsp;
                <select name="type" defaultValue={statistics.type}>
                  <option value="ressources">Points</option>
                  <option value="fleet">Fleets</option>
                  <option value="research">Research</option>
                </select>
                &nbsp;in place&nbsp;
                <select name="start" defaultValue={String(statistics.start)}>
                  <option value="-1">[Own position]</option>
                  {windows.map((start) => (
                    <option key={start} value={start}>
                      {start}-{start + 99}
                    </option>
                  ))}
                </select>
                &nbsp;
                <input type="submit" value="Show" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      {statistics.who === "ally" ? <AllianceStatisticsTable statistics={statistics} /> : <PlayerStatisticsTable statistics={statistics} />}
    </>
  );
}

function PlayerStatisticsTable({ statistics }: { statistics: GameStatistics }) {
  return (
    <table className="legacy-overview-table legacy-statistics-table" width={525}>
      <tbody>
        <tr>
          <td className="legacy-c" width={30}>
            Place
          </td>
          <td className="legacy-c">Player</td>
          <td className="legacy-c">&nbsp;</td>
          <td className="legacy-c">Alliance</td>
          <td className="legacy-c">Points</td>
        </tr>
        {statistics.rows.map((row) => (
          <tr data-statistics-row key={`${row.player.id}-${row.place}`}>
            <th>
              {row.place}&nbsp;&nbsp;
              <StatisticsDelta row={row} />
            </th>
            <th>
              <a
                href={row.own ? "#" : gameRouteURL("/game/galaxy", galaxyTargetSearch(row.coordinates))}
                style={{ color: row.own ? "lime" : "#FFFFFF" }}
              >
                {row.player.name}
              </a>
            </th>
            <th>
              {!row.own ? (
                <a href={gameRouteURL("/game/messages", window.location.search)}>
                  <img alt="Write message" src={`${skinBase}/img/m.gif`} style={{ border: 0 }} />
                </a>
              ) : null}
              &nbsp;
            </th>
            <th>
              {row.alliance ? (
                <a href={gameRouteURL("/game/alliance", window.location.search)}>{row.alliance.tag}</a>
              ) : (
                <a href={gameRouteURL("/game/alliance", window.location.search)}>&nbsp;</a>
              )}
            </th>
            <th>{formatLegacyNumber(row.displayScore)}</th>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function AllianceStatisticsTable({ statistics }: { statistics: GameStatistics }) {
  return (
    <table className="legacy-overview-table legacy-statistics-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c" width={30}>
            Place
          </td>
          <td className="legacy-c">Alliance</td>
          <td className="legacy-c">&nbsp;</td>
          <td className="legacy-c">Num.</td>
          <td className="legacy-c">Thousand points</td>
          <td className="legacy-c">Per person</td>
        </tr>
        {statistics.rows.map((row) => (
          <tr data-statistics-row key={`${row.alliance?.id ?? 0}-${row.place}`}>
            <th>
              {row.place}&nbsp;&nbsp;
              <StatisticsDelta row={row} />
            </th>
            <th>
              <a href={row.own ? "#" : gameRouteURL("/game/alliance", window.location.search)} style={{ color: row.own ? "lime" : "#FFFFFF" }}>
                {row.alliance?.tag ?? ""}
              </a>
            </th>
            <th>&nbsp;</th>
            <th>{formatLegacyNumber(row.members)}</th>
            <th>{formatLegacyNumber(row.displayScore)}</th>
            <th>{formatLegacyNumber(row.perMember)}</th>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function StatisticsDelta({ row }: { row: GameStatisticsRow }) {
  const title = `From ${formatLegacyDateTime(row.scoreDate)}`;
  if (row.delta < 0) {
    return (
      <a href="#" title={`+${Math.abs(row.delta)} ${title}`}>
        <span style={{ color: "lime" }}>+</span>
      </a>
    );
  }
  if (row.delta > 0) {
    return (
      <a href="#" title={`-${Math.abs(row.delta)} ${title}`}>
        <span style={{ color: "red" }}>-</span>
      </a>
    );
  }
  return (
    <a href="#" title={`* ${title}`}>
      <span style={{ color: "#87CEEB" }}>*</span>
    </a>
  );
}

function statisticsWindows(total: number, selectedStart: number): number[] {
  const windows: number[] = [];
  const max = Math.max(total, selectedStart, 1);
  for (let start = 1; start <= max; start += 100) {
    windows.push(start);
  }
  return windows;
}

function galaxyTargetSearch(coordinates: Coordinates): string {
  const query = new URLSearchParams(window.location.search);
  query.set("galaxy", String(coordinates.galaxy));
  query.set("system", String(coordinates.system));
  query.set("position", String(coordinates.position));
  return query.toString();
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

function FleetTable({ fleet }: { fleet: GameFleet }) {
  return (
    <>
      <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-fleet-table" width={519}>
        <tbody>
          <tr style={{ height: 20 }}>
            <td className="legacy-c" colSpan={8}>
              <table border={0} width="100%">
                <tbody>
                  <tr>
                    <td style={{ backgroundColor: "transparent" }}>
                      Fleets {fleet.slots.used} / {fleet.slots.baseMax}
                      {fleet.slots.admiral ? (
                        <span style={{ color: "lime" }}> +2</span>
                      ) : null}
                    </td>
                    <td align="right" style={{ backgroundColor: "transparent" }}>
                      {fleet.expeditions.used}/{fleet.expeditions.max} Expeditions
                    </td>
                  </tr>
                </tbody>
              </table>
            </td>
          </tr>
          <tr style={{ height: 20 }}>
            {["ID", "Mission", "Ships (total)", "Origin", "Departure Time", "Target", "Arrival Time", "Commands"].map((label) => (
              <th key={label}>{label}</th>
            ))}
          </tr>
          {fleet.missions.length === 0 ? (
            <tr style={{ height: 20 }}>
              {Array.from({ length: 8 }).map((_, index) => (
                <th key={index}>-</th>
              ))}
            </tr>
          ) : (
            fleet.missions.map((mission, index) => (
              <tr data-fleet-mission-row={mission.id} key={mission.id} style={{ height: 20 }}>
                <th>{index + 1}</th>
                <th>
                  <a title="">{mission.missionName}</a>
                  <br />
                  <a title={mission.stateTitle}>{mission.stateShort}</a>
                </th>
                <th>
                  <a title={mission.ships.map((ship) => `${ship.name}: ${formatLegacyNumber(ship.count)}`).join("\n")}>
                    {formatLegacyNumber(mission.totalShips)}
                  </a>
                </th>
                <th>
                  <a href={galaxyHref(mission.origin)}>[{formatCoordinates(mission.origin)}]</a>
                </th>
                <th>{formatFleetTimestamp(mission.departureAt)}</th>
                <th>
                  <a href={galaxyHref(mission.target)}>[{formatCoordinates(mission.target)}]</a>
                  {mission.targetOwnerName && mission.targetOwnerName !== "space" && mission.targetType <= 1 ? (
                    <>
                      <br />
                      {mission.targetOwnerName}
                    </>
                  ) : null}
                </th>
                <th>{formatFleetTimestamp(mission.arrivalAt)}</th>
                <th>
                  {mission.canCreateUnion ? (
                    <form onSubmit={(event) => event.preventDefault()}>
                      <input name="order_union" type="hidden" value={mission.id} />
                      <input type="submit" value="Union" />
                    </form>
                  ) : null}
                  {mission.canRecall ? (
                    <form onSubmit={(event) => event.preventDefault()}>
                      <input name="order_return" type="hidden" value={mission.id} />
                      <input type="submit" value="Recall" />
                    </form>
                  ) : null}
                </th>
              </tr>
            ))
          )}
        </tbody>
      </table>

      <form className="legacy-fleet-form" onSubmit={(event) => event.preventDefault()}>
        <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-fleet-select-table" width={519}>
          <tbody>
            {fleet.slots.used >= fleet.slots.max ? (
              <tr style={{ height: 20 }}>
                <th colSpan={4}>
                  <span style={{ color: "red" }}>Maximum fleet size has been reached!</span>
                </th>
              </tr>
            ) : null}
            <tr style={{ height: 20 }}>
              <td className="legacy-c" colSpan={4}>
                Please select your ships for this mission:
              </td>
            </tr>
            <tr style={{ height: 20 }}>
              <th>Ship Type</th>
              <th>Available</th>
              <th>-</th>
              <th>-</th>
            </tr>
            {fleet.ships.map((ship) => (
              <tr data-fleet-ship-row={ship.id} key={ship.id} style={{ height: 20 }}>
                <th>
                  <a title={`Speed: ${formatLegacyNumber(ship.speed)}`}>{ship.name}</a>
                </th>
                <th>
                  {formatLegacyNumber(ship.count)}
                  <input name={`maxship${ship.id}`} type="hidden" value={ship.count} />
                  <input name={`consumption${ship.id}`} type="hidden" value={ship.consumption} />
                  <input name={`speed${ship.id}`} type="hidden" value={ship.speed} />
                  <input name={`capacity${ship.id}`} type="hidden" value={ship.cargo} />
                </th>
                {ship.selectable ? (
                  <>
                    <th>
                      <a href="#max-ship" onClick={(event) => event.preventDefault()}>
                        all
                      </a>
                    </th>
                    <th>
                      <input aria-label={ship.name} defaultValue={0} name={`ship${ship.id}`} size={10} />
                    </th>
                  </>
                ) : (
                  <>
                    <th></th>
                    <th></th>
                  </>
                )}
              </tr>
            ))}
            <tr style={{ height: 20 }}>
              <th colSpan={2}>
                <a href="#clear-ships" onClick={(event) => event.preventDefault()}>
                  no ships
                </a>
              </th>
              <th colSpan={2}>
                <a href="#all-ships" onClick={(event) => event.preventDefault()}>
                  all ships
                </a>
              </th>
            </tr>
            <tr style={{ height: 20 }}>
              <th colSpan={4}>
                <input type="submit" value="continue" />
              </th>
            </tr>
            <tr>
              <th colSpan={4}></th>
            </tr>
          </tbody>
        </table>
      </form>
      <br />
      <br />
      <br />
      <br />
    </>
  );
}

function GalaxyTable({ galaxy }: { galaxy: GameGalaxy }) {
  const navigateTo = (coordinates: Coordinates) => {
    const search = new URLSearchParams(window.location.search);
    search.set("galaxy", String(clampNumber(coordinates.galaxy, 1, galaxy.bounds.galaxies)));
    search.set("system", String(clampNumber(coordinates.system, 1, galaxy.bounds.systems)));
    search.set("position", String(clampNumber(coordinates.position, 1, 16)));
    window.history.pushState({}, "", gameRouteURL("/game/galaxy", search.toString()));
    window.dispatchEvent(new PopStateEvent("popstate"));
  };
  const submitCoordinates = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    navigateTo({
      galaxy: Number(data.get("galaxy")) || galaxy.coordinates.galaxy,
      system: Number(data.get("system")) || galaxy.coordinates.system,
      position: galaxy.coordinates.position
    });
  };

  return (
    <>
      {galaxy.notEnoughDeuterium ? (
        <table className="legacy-overview-table legacy-galaxy-error-table" width={569}>
          <tbody>
            <tr>
              <td className="legacy-c"> Error</td>
            </tr>
            <tr>
              <th>Not enough deuterium!</th>
            </tr>
          </tbody>
        </table>
      ) : null}
      <form className="legacy-galaxy-form" onSubmit={submitCoordinates}>
        <table className="legacy-overview-table legacy-galaxy-nav-table" width={569}>
          <tbody>
            <tr>
              <td className="legacy-c">Galaxy</td>
              <td className="legacy-c">Solar system</td>
            </tr>
            <tr>
              <th>
                <input
                  aria-label="Galaxy"
                  defaultValue={galaxy.coordinates.galaxy}
                  maxLength={3}
                  name="galaxy"
                  size={3}
                  type="text"
                />
                <input
                  aria-label="Previous galaxy"
                  onClick={() => navigateTo({ ...galaxy.coordinates, galaxy: galaxy.coordinates.galaxy - 1 })}
                  type="button"
                  value="&lt;"
                />
                <input
                  aria-label="Next galaxy"
                  onClick={() => navigateTo({ ...galaxy.coordinates, galaxy: galaxy.coordinates.galaxy + 1 })}
                  type="button"
                  value="&gt;"
                />
              </th>
              <th>
                <input
                  aria-label="Solar system"
                  defaultValue={galaxy.coordinates.system}
                  maxLength={3}
                  name="system"
                  size={3}
                  type="text"
                />
                <input
                  aria-label="Previous system"
                  onClick={() => navigateTo({ ...galaxy.coordinates, system: galaxy.coordinates.system - 1 })}
                  type="button"
                  value="&lt;"
                />
                <input
                  aria-label="Next system"
                  onClick={() => navigateTo({ ...galaxy.coordinates, system: galaxy.coordinates.system + 1 })}
                  type="button"
                  value="&gt;"
                />
              </th>
            </tr>
            <tr>
              <th colSpan={2}>
                <input type="submit" value="Show" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      <table className="legacy-overview-table legacy-galaxy-table" width={569}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={8}>
              Solar system {galaxy.coordinates.galaxy}:{galaxy.coordinates.system}
            </td>
          </tr>
          <tr>
            {["Coord.", "Planet", "Title (activity)", "Moon", "Debris", "Player", "Alliance", "Actions"].map((label) => (
              <td className="legacy-c" key={label}>
                {label}
              </td>
            ))}
          </tr>
          {galaxy.rows.map((row) => (
            <GalaxyTableRow key={row.position} row={row} />
          ))}
          <tr>
            <th style={{ height: 32 }}>16</th>
            <th colSpan={7}>
              <a href={fleetTargetHref(galaxy.coordinates, 16, 15)}>Outer space</a>
            </th>
          </tr>
          <tr>
            <td className="legacy-c" colSpan={6}>
              (Populated {galaxy.populated} planets)
            </td>
            <td className="legacy-c" colSpan={2}>
              <a href="#legend" onClick={(event) => event.preventDefault()}>
                Legend
              </a>
            </td>
          </tr>
        </tbody>
      </table>
      <table className="legacy-overview-table legacy-galaxy-info-table" width={569}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={2}>
              {galaxy.extra.commander ? (
                <>
                  Espionage Probes {formatLegacyNumber(galaxy.extra.spyProbes)} Recycler {formatLegacyNumber(galaxy.extra.recyclers)}{" "}
                  Interplanetary Missiles {formatLegacyNumber(galaxy.extra.missiles)}
                  <br />
                  {galaxy.extra.slots.used} of {galaxy.extra.slots.max} slots are in use
                </>
              ) : null}
              {galaxy.remoteSystemCostDue ? (
                <>
                  {galaxy.extra.commander ? <br /> : null}
                  Deuterium: {GalaxyDeuteriumCostText}
                </>
              ) : null}
            </td>
          </tr>
        </tbody>
      </table>
      <br />
      <br />
    </>
  );
}

function GalaxyTableRow({ row }: { row: GameGalaxyRow }) {
  const planet = row.planet;
  const player = planet?.player;
  const debrisCoordinates = row.planet?.coordinates ?? row.moon?.coordinates;
  return (
    <tr data-galaxy-position={row.position}>
      <th style={{ width: 30 }}>
        <a href={`#position-${row.position}`}>{row.position}</a>
      </th>
      <th style={{ width: 30 }}>
        {planet && planet.type === 1 ? (
          <a href={fleetTargetHref(planet.coordinates, planet.coordinates.position, planet.own ? 4 : 1)}>
            <img alt="" height={30} src={galaxyPlanetImagePath(planet, true)} width={30} />
          </a>
        ) : null}
      </th>
      <th className="legacy-galaxy-name" style={{ width: 130 }}>
        {planet ? (
          <>
            <span className={planet.abandoned ? "longinactive" : planet.destroyed ? "banned" : undefined}>{planet.displayName}</span>
            {planet.activityText ? <> {planet.activityText}</> : null}
          </>
        ) : null}
      </th>
      <th style={{ width: 30 }}>
        {row.moon ? (
          <a className={row.moon.destroyed ? "legacy-galaxy-destroyed-moon" : undefined} href={fleetTargetHref(row.moon.coordinates, row.moon.coordinates.position, 3)}>
            <img alt="" height={22} src={galaxyPlanetImagePath(row.moon, true)} width={22} />
          </a>
        ) : null}
      </th>
      <th style={{ width: 30 }}>
        {row.debris?.visible && debrisCoordinates ? (
          <a href={fleetTargetHref(debrisCoordinates, row.position, 8, 2)}>
            <img alt="" height={22} src={`${skinBase}/planeten/debris.jpg`} title={`${formatLegacyNumber(row.debris.metal)} / ${formatLegacyNumber(row.debris.crystal)}`} width={22} />
          </a>
        ) : null}
      </th>
      <th style={{ width: 150 }}>
        {player ? (
          <>
            <span className={player.statusClass}>{player.name}</span>
            {player.suffixes.length > 0 ? (
              <>
                (
                {player.suffixes.map((suffix, index) => (
                  <React.Fragment key={`${suffix.class}-${suffix.text}`}>
                    {index > 0 ? " " : null}
                    <span className={suffix.class}>{suffix.text}</span>
                  </React.Fragment>
                ))}
                )
              </>
            ) : null}
          </>
        ) : null}
      </th>
      <th style={{ width: 80 }}>{planet?.alliance ? <a href="#alliance">{planet.alliance.tag}</a> : null}</th>
      <th className="legacy-galaxy-actions" style={{ whiteSpace: "nowrap", width: 125 }}>
        {planet ? <GalaxyActionIcons planet={planet} /> : null}
      </th>
    </tr>
  );
}

function GalaxyActionIcons({ planet }: { planet: GameGalaxyPlanet }) {
  const actions = [
    { enabled: planet.actions.spy, icon: "e.gif", label: "Espionage", mission: 6 },
    { enabled: planet.actions.message, icon: "m.gif", label: "Write message", mission: 0 },
    { enabled: planet.actions.buddy, icon: "b.gif", label: "Buddy request", mission: 0 },
    { enabled: planet.actions.missile, icon: "r.gif", label: "Rocket attack", mission: 20 }
  ];
  return (
    <>
      {actions.map((action) =>
        action.enabled ? (
          <a href={action.mission > 0 ? fleetTargetHref(planet.coordinates, planet.coordinates.position, action.mission) : "#"} key={action.icon}>
            <img alt={action.label} src={`${skinBase}/img/${action.icon}`} title={action.label} />
          </a>
        ) : null
      )}
    </>
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

function SearchTable({ search }: { search: GameSearch }) {
  return (
    <>
      <form
        action={gameRouteURL("/game/search", window.location.search)}
        method="get"
        onSubmit={(event) => {
          event.preventDefault();
          const form = new FormData(event.currentTarget);
          const query = new URLSearchParams(window.location.search);
          for (const key of ["gid", "tid", "who", "start"]) {
            query.delete(key);
          }
          query.set("type", String(form.get("type") ?? "playername"));
          query.set("searchtext", String(form.get("searchtext") ?? ""));
          window.history.pushState({}, "", gameRouteURL("/game/search", query.toString()));
          window.dispatchEvent(new PopStateEvent("popstate"));
        }}
      >
        <table className="legacy-overview-table legacy-search-head-table" width={519}>
          <tbody>
            <tr>
              <td className="legacy-c">Search Universe</td>
            </tr>
            <tr>
              <th>
                <select name="type" defaultValue={search.type}>
                  <option value="playername">Player Name</option>
                  <option value="planetname">Planet Name</option>
                  <option value="allytag">Alliance Tag</option>
                  <option value="allyname">Alliance Name</option>
                </select>
                &nbsp;&nbsp;
                <input name="searchtext" type="text" defaultValue={search.text} />
                &nbsp;&nbsp;
                <input type="submit" value="search" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      {search.message ? <SearchMessage text={search.message} /> : null}
      {search.type === "allytag" || search.type === "allyname" ? (
        <AllianceSearchResults rows={search.allianceRows} />
      ) : (
        <PlayerSearchResults rows={search.playerRows} />
      )}
    </>
  );
}

function NotesTable({
  notes,
  onCreate,
  onDelete,
  onUpdate,
  pending
}: {
  notes: GameNotes;
  onCreate: (draft: GameNoteDraft) => void;
  onDelete: (noteIDs: number[]) => void;
  onUpdate: (noteID: number, draft: GameNoteDraft) => void;
  pending: boolean;
}) {
  if (notes.action === "create") {
    return <NoteForm mode="create" onCreate={onCreate} onUpdate={onUpdate} pending={pending} />;
  }
  if (notes.action === "edit" && notes.editNote) {
    return <NoteForm mode="edit" note={notes.editNote} onCreate={onCreate} onUpdate={onUpdate} pending={pending} />;
  }
  return (
    <form
      action={noteURL({})}
      method="post"
      onSubmit={(event) => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        const ids: number[] = [];
        form.forEach((value, key) => {
          const match = /^delmes\[(\d+)\]$/.exec(key);
          if (match && value === "y") {
            ids.push(Number(match[1]));
          }
        });
        onDelete(ids);
      }}
    >
      <table className="legacy-overview-table legacy-notes-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={4}>
              Notes
            </td>
          </tr>
          <tr>
            <th colSpan={4}>
              <a href={noteURL({ action: 1 })}>Create a new note</a>
            </th>
          </tr>
          <tr>
            <td className="legacy-c">&nbsp;</td>
            <td className="legacy-c">Date</td>
            <td className="legacy-c">Subject</td>
            <td className="legacy-c">Size</td>
          </tr>
          {notes.rows.length > 0 ? (
            notes.rows.map((note) => (
              <tr data-note-row={note.id} key={note.id}>
                <th style={{ width: 20 }}>
                  <input disabled={pending} name={`delmes[${note.id}]`} type="checkbox" value="y" />
                </th>
                <th style={{ width: 150 }}>{formatLegacyDateTime(note.date)}</th>
                <th>
                  <a href={noteURL({ action: 2, noteID: note.id })}>
                    <span style={{ color: note.priorityColor }}>{note.subject}</span>
                  </a>
                </th>
                <th align="right" style={{ width: 40 }}>
                  {note.textSize}
                </th>
              </tr>
            ))
          ) : (
            <tr>
              <th colSpan={4}>no notes recorded</th>
            </tr>
          )}
          <tr>
            <td colSpan={4}>
              <input disabled={pending} type="submit" value="Delete" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function NoteForm({
  mode,
  note,
  onCreate,
  onUpdate,
  pending
}: {
  mode: "create" | "edit";
  note?: GameNote;
  onCreate: (draft: GameNoteDraft) => void;
  onUpdate: (noteID: number, draft: GameNoteDraft) => void;
  pending: boolean;
}) {
  const editNote = mode === "edit" ? note : undefined;
  const isEdit = editNote !== undefined;
  const priority = editNote ? editNote.priority : 2;
  return (
    <form
      action={noteURL({})}
      method="post"
      onSubmit={(event) => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        const draft = {
          subject: String(form.get("betreff") ?? ""),
          text: String(form.get("text") ?? ""),
          priority: Number(form.get("u") ?? priority)
        };
        if (editNote) {
          onUpdate(editNote.id, draft);
        } else {
          onCreate(draft);
        }
      }}
    >
      <input name="s" type="hidden" value={isEdit ? 2 : 1} />
      {editNote ? <input name="n" type="hidden" value={editNote.id} /> : null}
      <table className="legacy-overview-table legacy-notes-form-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={2}>
              {isEdit ? "Edit note" : "Create note"}
            </td>
          </tr>
          <tr>
            <th>Priority</th>
            <th>
              <select disabled={pending} name="u" defaultValue={priority}>
                <option value={2}>Important</option>
                <option value={1}>Normal</option>
                <option value={0}>Unimportant</option>
              </select>
            </th>
          </tr>
          <tr>
            <th>Subject</th>
            <th>
              <input disabled={pending} maxLength={30} name="betreff" size={30} type="text" defaultValue={editNote ? editNote.subject : ""} />
            </th>
          </tr>
          <tr>
            <th>
              {isEdit ? "Note" : "Notice"} (<span id="cntChars">{editNote ? editNote.textSize : 0}</span> / 5000 characters)
            </th>
            <th>
              <textarea cols={60} disabled={pending} name="text" rows={10} defaultValue={editNote ? editNote.text : ""} />
            </th>
          </tr>
          <tr>
            <td className="legacy-c">
              <a href={noteURL({})}>Back</a>
            </td>
            <td className="legacy-c">
              {isEdit ? <input disabled={pending} type="reset" value="Reset" /> : null}
              <input disabled={pending} type="submit" value={isEdit ? "Apply" : "Save"} />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function noteURL({ action, noteID }: { action?: number; noteID?: number }): string {
  const query = new URLSearchParams(window.location.search);
  query.delete("a");
  query.delete("n");
  if (action !== undefined) {
    query.set("a", String(action));
  }
  if (noteID !== undefined) {
    query.set("n", String(noteID));
  }
  return gameRouteURL("/game/notes", query.toString());
}

function SearchMessage({ text }: { text: string }) {
  return (
    <table className="legacy-overview-table legacy-search-message-table" width={519}>
      <tbody>
        <tr>
          <th>{text}</th>
        </tr>
      </tbody>
    </table>
  );
}

function PlayerSearchResults({ rows }: { rows: GameSearchPlayerRow[] }) {
  if (rows.length === 0) {
    return null;
  }
  return (
    <table className="legacy-overview-table legacy-search-results-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c">Name</td>
          <td className="legacy-c">&nbsp;</td>
          <td className="legacy-c">Alliance</td>
          <td className="legacy-c">Planet</td>
          <td className="legacy-c">Position</td>
          <td className="legacy-c">Place</td>
        </tr>
        {rows.map((row) => (
          <tr data-search-row key={`${row.playerId}-${row.planetId}`}>
            <th>
              <span style={{ color: row.own ? "lime" : row.sameAlliance ? "#87CEEB" : undefined }}>{row.playerName}</span>
            </th>
            <th>
              {!row.own ? (
                <>
                  <a href={gameSearchMessageHref(row.playerId)}>
                    <img alt="write message" src={`${skinBase}/img/m.gif`} title="write message" />
                  </a>
                  <a href={gameSearchBuddyHref(row.playerId)}>
                    <img alt="Buddy request" src={`${skinBase}/img/b.gif`} style={{ border: 0 }} title="Buddy request" />
                  </a>
                </>
              ) : (
                <>&nbsp;</>
              )}
            </th>
            <th>
              <a href={gameRouteURL("/game/alliance", window.location.search)} target="_ally">
                {row.alliance?.tag ?? ""}
              </a>
            </th>
            <th>{row.planetName}</th>
            <th>
              <a href={gameRouteURL("/game/galaxy", galaxyTargetSearch(row.coordinates))}>{formatCoordinates(row.coordinates)}</a>
            </th>
            <th>
              <a href={gameSearchStatisticsHref(row.place)}>{formatLegacyNumber(row.place)}</a>
            </th>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function AllianceSearchResults({ rows }: { rows: GameSearchAllianceRow[] }) {
  if (rows.length === 0) {
    return null;
  }
  return (
    <table className="legacy-overview-table legacy-search-results-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c">Tag</td>
          <td className="legacy-c">Name</td>
          <td className="legacy-c">Member</td>
          <td className="legacy-c">Points</td>
        </tr>
        {rows.map((row) => (
          <tr data-search-row key={row.allianceId}>
            <th>
              <a href={gameRouteURL("/game/alliance", window.location.search)} target="_ally">
                <span style={{ color: row.own ? "lime" : undefined }}>{row.tag}</span>
              </a>
            </th>
            <th>{row.name}</th>
            <th>{formatLegacyNumber(row.members)}</th>
            <th>{formatLegacyNumber(row.displayScore)}</th>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function gameSearchMessageHref(playerID: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("messageziel", String(playerID));
  return gameRouteURL("/game/messages", search.toString());
}

function gameSearchBuddyHref(playerID: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("action", "7");
  search.set("buddy_id", String(playerID));
  return gameRouteURL("/game/buddy", search.toString());
}

function gameSearchStatisticsHref(place: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("start", String(Math.floor(place / 100) * 100 + 1));
  return gameRouteURL("/game/statistics", search.toString());
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
            <a href={gameRouteURL("/game/rename-planet", window.location.search)} title="Planet menu">
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

function RenamePlanetTable({
  overview,
  onDelete,
  onRename,
  pending
}: {
  overview: GameOverview;
  onDelete: (password: string, deleteID: number) => void;
  onRename: (name: string) => void;
  pending: boolean;
}) {
  const planet = overview.currentPlanet;
  const [showDestroyMenu, setShowDestroyMenu] = React.useState(false);
  if (showDestroyMenu) {
    return (
      <RenamePlanetDestroyMenu
        onDelete={(password, deleteID) => {
          setShowDestroyMenu(false);
          onDelete(password, deleteID);
        }}
        overview={overview}
        pending={pending}
      />
    );
  }
  return (
    <>
      <h1>Rename/leave the planet</h1>
      <form
        action={gameRouteURL("/game/rename-planet", window.location.search)}
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          const form = new FormData(event.currentTarget);
          onRename(String(form.get("newname") ?? ""));
        }}
      >
        <center>
          <table className="legacy-overview-table legacy-rename-planet-table" width={519}>
            <tbody>
              <tr>
                <td className="legacy-c" colSpan={3}>
                  Planet information
                </td>
              </tr>
              <tr>
                <th>Coordinates</th>
                <th>Name</th>
                <th>Actions</th>
              </tr>
              <tr>
                <th>{formatCoordinates(planet.coordinates)}</th>
                <th>{planet.name}</th>
                <th>
                  <input
                    disabled={pending}
                    name="aktion"
                    onClick={(event) => {
                      event.preventDefault();
                      setShowDestroyMenu(true);
                    }}
                    type="submit"
                    value="Abandon the colony"
                  />
                </th>
              </tr>
              <tr>
                <th>Rename</th>
                <th>
                  <input disabled={pending} maxLength={20} name="newname" size={25} type="text" />
                  <br />
                </th>
                <th>
                  <input disabled={pending} name="aktion" type="submit" value="Rename" />
                </th>
              </tr>
            </tbody>
          </table>
        </center>
      </form>
      <br />
      <br />
      <br />
      <br />
    </>
  );
}

function RenamePlanetDestroyMenu({
  onDelete,
  overview,
  pending
}: {
  onDelete: (password: string, deleteID: number) => void;
  overview: GameOverview;
  pending: boolean;
}) {
  const planet = overview.currentPlanet;
  return (
    <>
      <h1>Rename/leave the planet</h1>
      <form
        action={gameRouteURL("/game/rename-planet", window.location.search)}
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          const form = new FormData(event.currentTarget);
          onDelete(String(form.get("pw") ?? ""), Number(form.get("deleteid") ?? planet.id));
        }}
      >
        <center>
          <table className="legacy-overview-table legacy-rename-destroy-table" width={519}>
            <tbody>
              <tr>
                <td className="legacy-c" colSpan={3}>
                  Just in case
                </td>
              </tr>
              <tr>
                <th colSpan={3}>Destruction of the planet [{formatCoordinates(planet.coordinates)}] confirm password</th>
              </tr>
              <tr>
                <th>Password</th>
                <th>
                  <input name="deleteid" type="hidden" value={planet.id} />
                  <input disabled={pending} name="pw" type="password" />
                </th>
                <th>
                  <input disabled={pending} alt="Abandon the colony" name="aktion" type="submit" value="Delete the planet!" />
                </th>
              </tr>
            </tbody>
          </table>
        </center>
      </form>
      <br />
      <br />
      <br />
      <br />
    </>
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
  return gamePlanetSwitchURL(window.location.pathname, window.location.search, planetID);
}

function galaxyHref(coordinates: Coordinates): string {
  const search = new URLSearchParams(window.location.search);
  search.set("galaxy", String(coordinates.galaxy));
  search.set("system", String(coordinates.system));
  search.set("position", String(coordinates.position));
  return gameRouteURL("/game/galaxy", search.toString());
}

function fleetTargetHref(coordinates: Coordinates, position: number, mission: number, planetType = 1): string {
  const search = new URLSearchParams(window.location.search);
  search.set("galaxy", String(coordinates.galaxy));
  search.set("system", String(coordinates.system));
  search.set("position", String(position));
  search.set("planet", String(position));
  search.set("planettype", String(planetType));
  search.set("target_mission", String(mission));
  return gameRouteURL("/game/fleet", search.toString());
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

function galaxyPlanetImagePath(planet: GameGalaxyPlanet, small: boolean): string {
  if (planet.type === 0 || planet.type === 10003) {
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

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
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

function formatLegacyDateTime(seconds: number): string {
  const date = new Date(seconds * 1000);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(
    2,
    "0"
  )} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}:${String(
    date.getSeconds()
  ).padStart(2, "0")}`;
}

function formatFleetTimestamp(seconds: number): string {
  return formatLegacyDate(new Date(seconds * 1000));
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
