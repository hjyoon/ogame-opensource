import React from "react";
import {
  gameBuddyRequestURL,
  gameFleetTargetPrefillFromSearch,
  gameFleetTargetURL,
  gameMessageComposeURL,
  gamePlanetSwitchURL,
  gameRouteURL,
  gameRoutes,
  type GameFleetTargetPrefill,
  type GameRoute
} from "./gameRoutes";

export type GameOverviewStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  overview?: GameOverview;
};

export type GameBuildingsStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  buildings?: GameBuildings;
};

export type GameEmpireStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  empire?: GameEmpire;
};

export type GameResourcesStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  resources?: GameResourceProduction;
};

export type GameResearchStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  research?: GameResearch;
};

export type GameShipyardStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  shipyard?: GameShipyard;
};

export type GameFleetStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  fleet?: GameFleet;
};

export type GameFleetDispatchPrepare = {
  ships: Record<string, number>;
  target: Coordinates;
  targetType: number;
  mission: number;
  speed: number;
};

export type GameFleetDispatchLaunch = GameFleetDispatchPrepare & {
  resources: Record<string, number>;
  holdHours: number;
  expeditionHours: number;
  unionId: number;
};

export type GameGalaxyStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  galaxy?: GameGalaxy;
};

export type GameDefenseStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
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

export type GameBuddyStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  buddy?: GameBuddy;
};

export type GameNotesStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  notes?: GameNotes;
};

export type GameMessagesStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  messages?: GameMessages;
};

export type GameReportStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  report?: GameReport;
};

export type GameOptionsStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  options?: GameOptions;
};

export type GameLogoutStatus = {
  loggedOut: boolean;
  redirectTo: string;
};

type GameOverview = {
  commander: string;
  serverTime?: string;
  messages?: string[];
  unreadMessages: number;
  score: {
    points: number;
    rawScore: number;
    rank: number;
    universePlayers: number;
  };
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  events: GameFleetMission[];
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
  buildQueue?: GameOverviewBuildQueue;
};

type GamePlanetSummary = {
  id: number;
  name: string;
  type: number;
  coordinates: Coordinates;
  current: boolean;
  buildQueue?: GameOverviewBuildQueue;
};

type GameOverviewBuildQueue = {
  techId: number;
  name: string;
  level: number;
  destroy: boolean;
  end: number;
};

type GameEmpire = {
  commander: string;
  commanderActive: boolean;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  planetType: number;
  moonEnabled: boolean;
  hasMoons: boolean;
  planets: GameEmpirePlanet[];
  resources: GameEmpireResourceRow[];
  buildings: GameEmpireLevelRow[];
  research: GameEmpireLevelRow[];
  fleet: GameEmpireCountRow[];
  defense: GameEmpireCountRow[];
};

type GameEmpirePlanet = {
  id: number;
  name: string;
  type: number;
  coordinates: Coordinates;
  fields: number;
  maxFields: number;
};

type GameEmpireResourceRow = {
  id: number;
  name: string;
  values: GameEmpireResourceValue[];
  total: number;
  production: number;
};

type GameEmpireResourceValue = {
  planetId: number;
  amount: number;
  production: number;
};

type GameEmpireLevelRow = {
  id: number;
  name: string;
  values: GameEmpireLevelValue[];
  total: number;
  average: number;
};

type GameEmpireLevelValue = {
  planetId: number;
  level: number;
  queue?: GameEmpireBuildQueueEntry[];
};

type GameEmpireBuildQueueEntry = {
  listId: number;
  level: number;
  active: boolean;
  demolish: boolean;
};

type GameEmpireCountRow = {
  id: number;
  name: string;
  values: GameEmpireCountValue[];
  total: number;
};

type GameEmpireCountValue = {
  planetId: number;
  count: number;
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
  active?: GameResearchQueue;
  items: GameBuildingItem[];
};

type GameResearchQueue = {
  taskId: number;
  planetId: number;
  techId: number;
  level: number;
  start: number;
  end: number;
  remainingSeconds: number;
  cancelable: boolean;
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
  commanderActive: boolean;
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
  templates: GameFleetTemplates;
  dispatchDraft?: GameFleetDispatchDraft;
};

type GameFleetShipCount = {
  id: number;
  name: string;
  count: number;
};

type GameFleetDispatchDraft = {
  ships: GameFleetShipCount[];
  totalShips: number;
  target: Coordinates;
  targetType: number;
  mission: number;
  speed: number;
  cargo: number;
  distance: number;
  durationSeconds: number;
  maxSpeed: number;
  fuelConsumption: number;
  speedFactor: number;
  remainingCargo: number;
  ready: boolean;
  hasSelection: boolean;
  missionOptions: GameFleetMissionOption[];
  resources: GameFleetResourceLoad[];
  holdHours?: number[];
  expeditionHours?: number[];
};

type GameFleetMissionOption = {
  id: number;
  name: string;
  selected: boolean;
  warning?: string;
};

type GameFleetResourceLoad = {
  id: number;
  name: string;
  available: number;
  requested: number;
  loaded: number;
};

type GameFleetTemplates = {
  commanderActive: boolean;
  max: number;
  items: GameFleetTemplate[];
};

type GameFleetTemplate = {
  id: number;
  name: string;
  updatedAt: number;
  ships: GameFleetTemplateShip[];
};

type GameFleetTemplateShip = {
  id: number;
  name: string;
  count: number;
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
  ownerId: number;
  ownerName: string;
  own: boolean;
  mission: number;
  missionName: string;
  stateTitle: string;
  stateShort: string;
  ships: { id: number; name: string; count: number }[];
  totalShips: number;
  missileAmount: number;
  missileTargetId: number;
  missileTarget: string;
  unionId: number;
  groupMissions: GameFleetMission[];
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
  sameAlliance: boolean;
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

type GameBuddy = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  action: number;
  rows: GameBuddyRow[];
  target?: GameBuddyPlayer;
};

type GameBuddyRow = {
  buddyId: number;
  player: GameBuddyPlayer;
  text: string;
  status: {
    text: string;
    color: string;
  };
};

type GameBuddyPlayer = {
  playerId: number;
  name: string;
  alliance?: { id: number; tag: string; founder: boolean };
  coordinates: Coordinates;
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

type GameMessages = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  action: "inbox" | "compose";
  rows: GameMessage[];
  compose?: GameMessageCompose;
};

type GameMessage = {
  id: number;
  type: number;
  from: string;
  subject: string;
  text: string;
  date: number;
  unread: boolean;
  reportable: boolean;
};

type GameReport = {
  id: number;
  type: number;
  title: string;
  text: string;
  allowed: boolean;
};

type GameOptions = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  user: GameOptionsUser;
  universe: GameOptionsUniverse;
  settings: GameOptionsSettings;
  account: GameOptionsAccount;
  flags: GameOptionsFlags;
};

type GameOptionsUser = {
  name: string;
  nameLocked: boolean;
  email: string;
  plainEmail: string;
  validated: boolean;
  admin: number;
  feedId: string;
  commanderOn: boolean;
};

type GameOptionsUniverse = {
  language: string;
  forceLanguage: boolean;
  feedAge: number;
};

type GameOptionsSettings = {
  language: string;
  skinPath: string;
  useSkin: boolean;
  deactivateIp: boolean;
  sortBy: number;
  sortOrder: number;
  maxSpy: number;
  maxFleetMessages: number;
};

type GameOptionsAccount = {
  vacation: boolean;
  vacationUntil: number;
  deletionQueued: boolean;
  deletionAt: number;
};

type GameOptionsFlags = {
  showEspionageButton: boolean;
  showWriteMessage: boolean;
  showBuddy: boolean;
  showRocketAttack: boolean;
  showViewReport: boolean;
  doNotUseFolders: boolean;
  feedEnabled: boolean;
  feedAtom: boolean;
  hideGoEmail: boolean;
};

type GameMessageCompose = {
  target: {
    playerId: number;
    name: string;
    coordinates: Coordinates;
  };
  subject: string;
  maxChars: number;
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
  buildingsPending: boolean;
  onBuildingAction: (action: "add" | "destroy" | "remove", techID: number, listID?: number) => void;
  empireStatus: GameEmpireStatus | null;
  empireError: string | null;
  resourcesStatus: GameResourcesStatus | null;
  resourcesError: string | null;
  resourcesPending: boolean;
  onResourcesSubmit: (production: Record<string, string>) => void;
  researchStatus: GameResearchStatus | null;
  researchError: string | null;
  researchPending: boolean;
  onResearchAction: (action: "start" | "cancel", techID: number) => void;
  shipyardStatus: GameShipyardStatus | null;
  shipyardError: string | null;
  shipyardPending: boolean;
  onShipyardSubmit: (orders: Record<string, number>) => void;
  fleetStatus: GameFleetStatus | null;
  fleetError: string | null;
  fleetPending: boolean;
  onFleetPrepare: (draft: GameFleetDispatchPrepare) => void;
  onFleetLaunch: (draft: GameFleetDispatchLaunch) => void;
  onFleetRecall: (fleetID: number) => void;
  onFleetTemplateAction: (action: "save" | "delete", templateID: number, name: string, ships: Record<string, number>) => void;
  galaxyStatus: GameGalaxyStatus | null;
  galaxyError: string | null;
  defenseStatus: GameDefenseStatus | null;
  defenseError: string | null;
  defensePending: boolean;
  onDefenseSubmit: (orders: Record<string, number>) => void;
  technologyStatus: GameTechnologyStatus | null;
  technologyError: string | null;
  statisticsStatus: GameStatisticsStatus | null;
  statisticsError: string | null;
  searchStatus: GameSearchStatus | null;
  searchError: string | null;
  buddyStatus: GameBuddyStatus | null;
  buddyError: string | null;
  buddyPending: boolean;
  onBuddyAction: (action: number, buddyID: number) => void;
  onBuddyRequest: (buddyID: number, text: string) => void;
  notesStatus: GameNotesStatus | null;
  notesError: string | null;
  notesPending: boolean;
  onNotesCreate: (draft: GameNoteDraft) => void;
  onNotesUpdate: (noteID: number, draft: GameNoteDraft) => void;
  onNotesDelete: (noteIDs: number[]) => void;
  messagesStatus: GameMessagesStatus | null;
  messagesError: string | null;
  messagesPending: boolean;
  onMessagesDelete: (deleteMode: string, messageIDs: number[], reportIDs: number[]) => void;
  onMessageSend: (targetPlayerID: number, subject: string, text: string) => void;
  reportStatus: GameReportStatus | null;
  reportError: string | null;
  optionsStatus: GameOptionsStatus | null;
  optionsError: string | null;
  optionsPending: boolean;
  onOptionsSubmit: (settings: {
    language: string;
    skinPath: string;
    useSkin: boolean;
    deactivateIp: boolean;
    sortBy: number;
    sortOrder: number;
    maxSpy: number;
    maxFleetMessages: number;
    deleteAccount: boolean;
  }) => void;
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
  buildingsPending,
  onBuildingAction,
  empireStatus,
  empireError,
  resourcesStatus,
  resourcesError,
  resourcesPending,
  onResourcesSubmit,
  researchStatus,
  researchError,
  researchPending,
  onResearchAction,
  shipyardStatus,
  shipyardError,
  shipyardPending,
  onShipyardSubmit,
  fleetStatus,
  fleetError,
  fleetPending,
  onFleetPrepare,
  onFleetLaunch,
  onFleetRecall,
  onFleetTemplateAction,
  galaxyStatus,
  galaxyError,
  defenseStatus,
  defenseError,
  defensePending,
  onDefenseSubmit,
  technologyStatus,
  technologyError,
  statisticsStatus,
  statisticsError,
  searchStatus,
  searchError,
  buddyStatus,
  buddyError,
  buddyPending,
  onBuddyAction,
  onBuddyRequest,
  notesStatus,
  notesError,
  notesPending,
  onNotesCreate,
  onNotesUpdate,
  onNotesDelete,
  messagesStatus,
  messagesError,
  messagesPending,
  onMessagesDelete,
  onMessageSend,
  reportStatus,
  reportError,
  optionsStatus,
  optionsError,
  optionsPending,
  onOptionsSubmit,
  logoutStatus,
  logoutError
}: LegacyGameOverviewProps) {
  const overview = status?.authenticated ? status.overview : undefined;
  const issue = status && !status.authenticated ? status.issues[0]?.message ?? "Session is invalid." : null;
  const buildings = buildingsStatus?.authenticated ? buildingsStatus.buildings : undefined;
  const buildingsIssue =
    buildingsStatus && !buildingsStatus.authenticated ? buildingsStatus.issues[0]?.message ?? "Session is invalid." : null;
  const empire = empireStatus?.authenticated ? empireStatus.empire : undefined;
  const empireIssue =
    empireStatus && !empireStatus.authenticated ? empireStatus.issues[0]?.message ?? "Session is invalid." : null;
  const empireActionIssue = empireStatus?.authenticated ? empireStatus.actionIssue : undefined;
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
  const fleetActionIssue = fleetStatus?.authenticated ? fleetStatus.actionIssue : undefined;
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
  const buddy = buddyStatus?.authenticated ? buddyStatus.buddy : undefined;
  const buddyIssue = buddyStatus && !buddyStatus.authenticated ? buddyStatus.issues[0]?.message ?? "Session is invalid." : null;
  const notes = notesStatus?.authenticated ? notesStatus.notes : undefined;
  const notesIssue = notesStatus && !notesStatus.authenticated ? notesStatus.issues[0]?.message ?? "Session is invalid." : null;
  const messages = messagesStatus?.authenticated ? messagesStatus.messages : undefined;
  const messagesIssue =
    messagesStatus && !messagesStatus.authenticated ? messagesStatus.issues[0]?.message ?? "Session is invalid." : null;
  const report = reportStatus?.authenticated ? reportStatus.report : undefined;
  const reportIssue = reportStatus && !reportStatus.authenticated ? reportStatus.issues[0]?.message ?? "Session is invalid." : null;
  const options = optionsStatus?.authenticated ? optionsStatus.options : undefined;
  const optionsIssue =
    optionsStatus && !optionsStatus.authenticated ? optionsStatus.issues[0]?.message ?? "Session is invalid." : null;
  const messagesActionIssue = messagesStatus?.authenticated ? messagesStatus.actionIssue : undefined;
  const messagesActionTone =
    messagesActionIssue?.code === "sent" || messagesActionIssue?.code === "reported" ? "neutral" : "error";
  const optionsActionIssue = optionsStatus?.authenticated ? optionsStatus.actionIssue : undefined;
  const hasHeader = route.key !== "notes" && route.key !== "galaxy" && route.key !== "report";
  const hasMenu = route.key !== "notes" && route.key !== "report";
  const contentClassName =
    route.key === "overview"
      ? "legacy-content legacy-content-overview"
      : route.key === "galaxy"
        ? "legacy-content legacy-content-noheader"
      : route.key === "notes" || route.key === "report"
        ? "legacy-content legacy-content-popup"
        : "legacy-content";

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
      {hasHeader ? (
        <header className="legacy-header-top" id="header_top">
          {overview ? <LegacyResourceHeader overview={overview} /> : <div className="legacy-header-placeholder">OGame</div>}
        </header>
      ) : null}
      {hasMenu ? <LegacyLeftMenu activeRoute={route} /> : null}
      {hasHeader && overview && route.key === "overview" && overview.messages && overview.messages.length > 0 ? (
        <LegacyPageMessage messages={overview.messages} />
      ) : null}
      <section className={contentClassName} id="content">
        {error ? <LegacyMessage tone="error" text={error} /> : null}
        {!error && issue ? <LegacyMessage tone="error" text={issue} /> : null}
        {!error && !issue && !overview && route.key !== "logout" && route.key !== "report" ? (
          <LegacyMessage tone="neutral" text="Loading overview..." />
        ) : null}
        {route.key === "logout" ? <LogoutTable error={logoutError} status={logoutStatus} /> : null}
        {route.key === "buildings" && buildingsError ? <LegacyMessage tone="error" text={buildingsError} /> : null}
        {route.key === "buildings" && !buildingsError && buildingsIssue ? (
          <LegacyMessage tone="error" text={buildingsIssue} />
        ) : null}
        {route.key === "empire" && empireError ? <LegacyMessage tone="error" text={empireError} /> : null}
        {route.key === "empire" && !empireError && empireActionIssue ? (
          <LegacyMessage tone="error" text={empireActionIssue.message} />
        ) : null}
        {route.key === "empire" && !empireError && !empireActionIssue && empireIssue ? (
          <LegacyMessage tone="error" text={empireIssue} />
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
        {route.key === "buddy" && buddyError ? <LegacyMessage tone="error" text={buddyError} /> : null}
        {route.key === "buddy" && !buddyError && buddyIssue ? <LegacyMessage tone="error" text={buddyIssue} /> : null}
        {route.key === "messages" && messagesError ? <LegacyMessage tone="error" text={messagesError} /> : null}
        {route.key === "messages" && !messagesError && messagesActionIssue ? (
          <LegacyMessage tone={messagesActionTone} text={messagesActionIssue.message} />
        ) : null}
        {route.key === "messages" && !messagesError && !messagesActionIssue && messagesIssue ? (
          <LegacyMessage tone="error" text={messagesIssue} />
        ) : null}
        {route.key === "notes" && notesError ? <LegacyMessage tone="error" text={notesError} /> : null}
        {route.key === "notes" && !notesError && notesIssue ? <LegacyMessage tone="error" text={notesIssue} /> : null}
        {route.key === "report" && reportError ? <LegacyMessage tone="error" text={reportError} /> : null}
        {route.key === "report" && !reportError && reportIssue ? <LegacyMessage tone="error" text={reportIssue} /> : null}
        {route.key === "report" && !report && !reportError && !reportIssue ? (
          <LegacyMessage tone="neutral" text="Loading report..." />
        ) : null}
        {route.key === "options" && optionsError ? <LegacyMessage tone="error" text={optionsError} /> : null}
        {route.key === "options" && !optionsError && optionsActionIssue ? (
          <LegacyMessage tone="neutral" text={optionsActionIssue.message} />
        ) : null}
        {route.key === "options" && !optionsError && !optionsActionIssue && optionsIssue ? (
          <LegacyMessage tone="error" text={optionsIssue} />
        ) : null}
        {report && route.key === "report" ? <ReportTable report={report} /> : null}
        {overview && route.key === "overview" ? <OverviewTable overview={overview} /> : null}
        {overview && route.key === "renamePlanet" ? (
          <RenamePlanetTable onDelete={onPlanetDelete} onRename={onPlanetRename} overview={overview} pending={overviewPending} />
        ) : null}
        {overview && route.key === "buildings" && !buildings && !buildingsError && !buildingsIssue ? (
          <LegacyMessage tone="neutral" text="Loading buildings..." />
        ) : null}
        {buildings && route.key === "buildings" ? (
          <BuildingsTable buildings={buildings} onAction={onBuildingAction} pending={buildingsPending} />
        ) : null}
        {overview && route.key === "empire" && !empire && !empireError && !empireIssue && !empireActionIssue ? (
          <LegacyMessage tone="neutral" text="Loading empire..." />
        ) : null}
        {empire && route.key === "empire" ? <EmpireTable empire={empire} /> : null}
        {overview && route.key === "resources" && !resources && !resourcesError && !resourcesIssue ? (
          <LegacyMessage tone="neutral" text="Loading resources..." />
        ) : null}
        {resources && route.key === "resources" ? (
          <ResourcesTable onSubmit={onResourcesSubmit} pending={resourcesPending} resources={resources} />
        ) : null}
        {overview && route.key === "research" && !research && !researchError && !researchIssue ? (
          <LegacyMessage tone="neutral" text="Loading research..." />
        ) : null}
        {research && route.key === "research" ? (
          <ResearchTable onAction={onResearchAction} pending={researchPending} research={research} />
        ) : null}
        {overview && route.key === "shipyard" && !shipyard && !shipyardError && !shipyardIssue ? (
          <LegacyMessage tone="neutral" text="Loading shipyard..." />
        ) : null}
        {shipyard && route.key === "shipyard" ? (
          <ShipyardTable onSubmit={onShipyardSubmit} pending={shipyardPending} shipyard={shipyard} />
        ) : null}
        {overview && (route.key === "fleet" || route.key === "fleetTemplates") && !fleet && !fleetError && !fleetIssue ? (
          <LegacyMessage tone="neutral" text="Loading fleet..." />
        ) : null}
        {route.key === "fleet" && !fleetError && fleetActionIssue ? (
          <LegacyMessage tone="error" text={fleetActionIssue.message} />
        ) : null}
        {fleet && route.key === "fleet" ? (
          <FleetTable fleet={fleet} onLaunch={onFleetLaunch} onPrepare={onFleetPrepare} onRecall={onFleetRecall} pending={fleetPending} />
        ) : null}
        {fleet && route.key === "fleetTemplates" ? (
          <FleetTemplatesTable fleet={fleet} onAction={onFleetTemplateAction} pending={fleetPending} />
        ) : null}
        {overview && route.key === "galaxy" && !galaxy && !galaxyError && !galaxyIssue ? (
          <LegacyMessage tone="neutral" text="Loading galaxy..." />
        ) : null}
        {galaxy && route.key === "galaxy" ? <GalaxyTable galaxy={galaxy} /> : null}
        {overview && route.key === "defense" && !defense && !defenseError && !defenseIssue ? (
          <LegacyMessage tone="neutral" text="Loading defense..." />
        ) : null}
        {defense && route.key === "defense" ? (
          <DefenseTable defense={defense} onSubmit={onDefenseSubmit} pending={defensePending} />
        ) : null}
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
        {overview && route.key === "buddy" && !buddy && !buddyError && !buddyIssue ? (
          <LegacyMessage tone="neutral" text="Loading buddy list..." />
        ) : null}
        {buddy && route.key === "buddy" ? (
          <BuddyTable buddy={buddy} onAction={onBuddyAction} onRequest={onBuddyRequest} pending={buddyPending} />
        ) : null}
        {overview && route.key === "messages" && !messages && !messagesError && !messagesIssue ? (
          <LegacyMessage tone="neutral" text="Loading messages..." />
        ) : null}
        {messages && route.key === "messages" ? (
          <MessagesTable
            messages={messages}
            onDelete={onMessagesDelete}
            onSend={onMessageSend}
            pending={messagesPending}
          />
        ) : null}
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
        {overview && route.key === "options" && !options && !optionsError && !optionsIssue ? (
          <LegacyMessage tone="neutral" text="Loading options..." />
        ) : null}
        {options && route.key === "options" ? (
          <OptionsTable onSubmit={onOptionsSubmit} options={options} pending={optionsPending} />
        ) : null}
        {overview &&
        route.key !== "overview" &&
        route.key !== "renamePlanet" &&
        route.key !== "buildings" &&
        route.key !== "empire" &&
        route.key !== "resources" &&
        route.key !== "research" &&
        route.key !== "shipyard" &&
        route.key !== "fleet" &&
        route.key !== "galaxy" &&
        route.key !== "defense" &&
        route.key !== "technology" &&
        route.key !== "statistics" &&
        route.key !== "search" &&
        route.key !== "buddy" &&
        route.key !== "messages" &&
        route.key !== "report" &&
        route.key !== "notes" &&
        route.key !== "options" &&
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
          </td>
          <td className="legacy-header-cell">
            <table className="legacy-officer-table">
              <tbody>
                <tr>
                  {officers.map((officer) => (
                    <td className="legacy-header-cell" key={officer}>
                      <img alt="" height={32} src={`${gameImageBase}/${officer}_ikon_un.gif`} width={32} />
                    </td>
                  ))}
                  <td className="legacy-header-cell" />
                </tr>
              </tbody>
            </table>
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
        className="legacy-statistics-form"
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
          query.set("sort_per_member", String(form.get("sort_per_member") ?? "0"));
          window.history.pushState({}, "", gameRouteURL("/game/statistics", query.toString()));
          window.dispatchEvent(new PopStateEvent("popstate"));
        }}
      >
        <table className="legacy-overview-table legacy-statistics-head-table" width={525}>
          <tbody>
            <tr>
              <td className="legacy-c">Statistics (as of: {formatLegacyStatisticsDateTime(statistics.generatedAt)})</td>
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
                <input id="sort_per_member" name="sort_per_member" type="hidden" value={statisticsSortValue()} readOnly />
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
    <table className="legacy-overview-table legacy-statistics-table legacy-statistics-player-table" width={525}>
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
                style={{ color: row.own ? "lime" : row.sameAlliance ? "#87CEEB" : "#FFFFFF" }}
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
    <table className="legacy-overview-table legacy-statistics-table legacy-statistics-alliance-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c" width={30}>
            Place
          </td>
          <td className="legacy-c">Alliance</td>
          <td className="legacy-c">&nbsp;</td>
          <td className="legacy-c">Num.</td>
          <td className="legacy-c">
            <a href={statisticsSortURL(0)}>Thousand points</a>
          </td>
          <td className="legacy-c">
            <a href={statisticsSortURL(1)}>Per person</a>
          </td>
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

function statisticsSortURL(sortPerMember: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("sort_per_member", String(sortPerMember));
  return gameRouteURL("/game/statistics", search.toString());
}

function statisticsSortValue(): string {
  return new URLSearchParams(window.location.search).get("sort_per_member") ?? "0";
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

function BuildingsTable({
  buildings,
  onAction,
  pending
}: {
  buildings: GameBuildings;
  onAction: (action: "add" | "destroy" | "remove", techID: number, listID?: number) => void;
  pending: boolean;
}) {
  return (
    <table className="legacy-overview-table legacy-buildings-table" width={530}>
      <tbody>
        {buildings.items.map((item) => {
          const actionContent = (
            <>
              {item.action}
              {item.action === "Build level" ? (
                <>
                  <br />
                  level {item.nextLevel}
                </>
              ) : null}
            </>
          );
          return (
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
                {item.canBuild ? (
                  <a
                    className="legacy-build-ok"
                    href={buildingActionURL("add", item.id)}
                    onClick={(event) => {
                      event.preventDefault();
                      if (!pending) {
                        onAction("add", item.id);
                      }
                    }}
                  >
                    {actionContent}
                  </a>
                ) : (
                  <span className="legacy-build-blocked">{actionContent}</span>
                )}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function buildingActionURL(action: "add" | "destroy" | "remove", techID: number, listID?: number) {
  const query = new URLSearchParams(window.location.search);
  query.set("modus", action);
  if (action === "add" || action === "destroy") {
    query.set("techid", String(techID));
  }
  if (listID !== undefined) {
    query.set("listid", String(listID));
  }
  return gameRouteURL("/game/buildings", `?${query.toString()}`);
}

function ResearchTable({
  onAction,
  pending,
  research
}: {
  onAction: (action: "start" | "cancel", techID: number) => void;
  pending: boolean;
  research: GameResearch;
}) {
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
        {research.items.map((item) => {
          const active = research.active?.techId === item.id ? research.active : undefined;
          const actionContent =
            item.action === "Cancel" && active ? (
              <>
                {formatLegacyDuration(active.remainingSeconds)}
                <br />
                Cancel
              </>
            ) : (
              <>
                {item.action}
                {item.action === "Research level" ? (
                  <>
                    <br />
                    level {item.nextLevel}
                  </>
                ) : null}
              </>
            );
          const action = item.action === "Cancel" ? "cancel" : "start";
          return (
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
                {item.canBuild ? (
                  <a
                    className="legacy-build-ok"
                    href={researchActionURL(action, item.id)}
                    onClick={(event) => {
                      event.preventDefault();
                      if (!pending) {
                        onAction(action, item.id);
                      }
                    }}
                  >
                    {actionContent}
                  </a>
                ) : (
                  <span className="legacy-build-blocked">{actionContent}</span>
                )}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function researchActionURL(action: "start" | "cancel", techID: number) {
  const query = new URLSearchParams(window.location.search);
  if (action === "start") {
    query.set("bau", String(techID));
  } else {
    query.set("unbau", String(techID));
  }
  return gameRouteURL("/game/research", `?${query.toString()}`);
}

function collectLegacyUnitOrders(form: HTMLFormElement): Record<string, number> {
  const orders: Record<string, number> = {};
  const formData = new FormData(form);
  for (const [key, value] of formData.entries()) {
    const match = /^fmenge\[(\d+)\]$/.exec(key);
    if (!match || typeof value !== "string") {
      continue;
    }
    const amount = Number.parseInt(value, 10);
    if (Number.isFinite(amount) && amount > 0) {
      orders[match[1]] = amount;
    }
  }
  return orders;
}

function setLegacyUnitOrderMax(anchor: HTMLAnchorElement, itemID: number, maximum: number) {
  const form = anchor.closest("form");
  const input = form?.elements.namedItem(`fmenge[${itemID}]`);
  if (input instanceof HTMLInputElement) {
    input.value = String(maximum);
  }
}

function ShipyardTable({
  onSubmit,
  pending,
  shipyard
}: {
  onSubmit: (orders: Record<string, number>) => void;
  pending: boolean;
  shipyard: GameShipyard;
}) {
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
    <form
      className="legacy-shipyard-form"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(collectLegacyUnitOrders(event.currentTarget));
      }}
    >
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
                    <input
                      aria-label={item.name}
                      defaultValue={0}
                      disabled={pending}
                      maxLength={6}
                      name={`fmenge[${item.id}]`}
                      size={6}
                      type="text"
                    />
                    {item.maxBuild > 0 ? (
                      <>
                        <br />
                        <a
                          href="#max"
                          onClick={(event) => {
                            event.preventDefault();
                            setLegacyUnitOrderMax(event.currentTarget, item.id, item.maxBuild);
                          }}
                        >
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
              <input disabled={pending} type="submit" value="Build" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function FleetTable({
  fleet,
  onPrepare,
  onLaunch,
  onRecall,
  pending
}: {
  fleet: GameFleet;
  onPrepare: (draft: GameFleetDispatchPrepare) => void;
  onLaunch: (draft: GameFleetDispatchLaunch) => void;
  onRecall: (fleetID: number) => void;
  pending: boolean;
}) {
  const targetPrefill = gameFleetTargetPrefillFromSearch(window.location.search);
  const dispatchTarget = targetPrefill
    ? {
        galaxy: targetPrefill.targetGalaxy,
        system: targetPrefill.targetSystem,
        position: targetPrefill.targetPlanet
      }
    : fleet.currentPlanet.coordinates;
  const dispatchTargetType = targetPrefill?.targetPlanetType ?? 1;
  const dispatchMission = targetPrefill?.targetMission ?? 0;
  const submitDispatchDraft = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onPrepare({
      ships: collectLegacyFleetShips(event.currentTarget),
      target: dispatchTarget,
      targetType: dispatchTargetType,
      mission: dispatchMission,
      speed: 10
    });
  };
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
                    <form
                      onSubmit={(event) => {
                        event.preventDefault();
                        onRecall(mission.id);
                      }}
                    >
                      <input name="order_return" type="hidden" value={mission.id} />
                      <input disabled={pending} type="submit" value="Recall" />
                    </form>
                  ) : null}
                </th>
              </tr>
            ))
          )}
        </tbody>
      </table>

      <form className="legacy-fleet-form" onSubmit={submitDispatchDraft}>
        {targetPrefill ? <FleetTargetPrefillInputs prefill={targetPrefill} /> : null}
        <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-fleet-select-table" width={519}>
          <tbody>
            {fleet.slots.used >= fleet.slots.max ? (
              <tr style={{ height: 20 }}>
                <th colSpan={4}>
                  <span style={{ color: "red" }}>Maximum fleet size has been reached!</span>
                </th>
              </tr>
            ) : null}
            {fleet.templates.commanderActive ? (
              <tr style={{ height: 20 }}>
                <td className="legacy-c" colSpan={4}>
                  <a href={gameRouteURL("/game/fleet-templates", window.location.search)}>Standard fleets</a>
                  {fleet.templates.items.length > 0 ? (
                    <>
                      {" "}
                      {fleet.templates.items.map((template) => (
                        <React.Fragment key={template.id}>
                          <a
                            href={fleetTemplateJavascriptHref(template)}
                            onClick={(event) => {
                              event.preventDefault();
                              setLegacyFleetTemplateShips(template);
                            }}
                          >
                            {template.name}
                          </a>{" "}
                        </React.Fragment>
                      ))}
                    </>
                  ) : null}
                </td>
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
                      <a
                        href="#max-ship"
                        onClick={(event) => {
                          event.preventDefault();
                          setLegacyFleetShipAmount(event.currentTarget, ship.id, ship.count);
                        }}
                      >
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
                <a
                  href="#clear-ships"
                  onClick={(event) => {
                    event.preventDefault();
                    setLegacyFleetShips(event.currentTarget, fleet.ships, "none");
                  }}
                >
                  no ships
                </a>
              </th>
              <th colSpan={2}>
                <a
                  href="#all-ships"
                  onClick={(event) => {
                    event.preventDefault();
                    setLegacyFleetShips(event.currentTarget, fleet.ships, "all");
                  }}
                >
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
      {fleet.dispatchDraft?.hasSelection ? <FleetDispatchPreviewTable draft={fleet.dispatchDraft} fleet={fleet} onLaunch={onLaunch} pending={pending} /> : null}
      <br />
      <br />
      <br />
      <br />
    </>
  );
}

function FleetDispatchPreviewTable({
  draft,
  fleet,
  onLaunch,
  pending
}: {
  draft: GameFleetDispatchDraft;
  fleet: GameFleet;
  onLaunch: (draft: GameFleetDispatchLaunch) => void;
  pending: boolean;
}) {
  const submitLaunch = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onLaunch({
      ships: fleetDraftShipsPayload(draft),
      resources: collectLegacyFleetResources(event.currentTarget),
      target: draft.target,
      targetType: draft.targetType,
      mission: legacyFormInt(form.get("order"), draft.mission),
      speed: draft.speed,
      holdHours: legacyFormInt(form.get("holdingtime"), 0),
      expeditionHours: legacyFormInt(form.get("expeditiontime"), 0),
      unionId: 0
    });
  };
  return (
    <form className="legacy-fleet-dispatch-form" data-dispatch-action="launch-dispatch" onSubmit={submitLaunch}>
      <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-fleet-dispatch-table" width={519}>
        <tbody>
          <tr style={{ height: 20, textAlign: "left" }}>
            <td className="legacy-c" colSpan={2}>
              {formatCoordinates(draft.target)} - {fleetPlanetTypeName(draft.targetType)}
            </td>
          </tr>
          <tr style={{ textAlign: "left", verticalAlign: "top" }}>
            <th style={{ width: "50%" }}>
              <FleetDispatchMissionTable draft={draft} />
            </th>
            <th>
              <FleetDispatchResourcesTable draft={draft} fleet={fleet} />
            </th>
          </tr>
          <tr style={{ height: 20 }}>
            <th colSpan={2}>
              <input disabled={pending} type="submit" value="Next" />
            </th>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function FleetDispatchMissionTable({ draft }: { draft: GameFleetDispatchDraft }) {
  return (
    <table border={0} cellPadding={0} cellSpacing={0} width={259}>
      <tbody>
        <tr style={{ height: 20 }}>
          <td className="legacy-c" colSpan={2}>
            Mission
          </td>
        </tr>
        {draft.missionOptions.length === 0 ? (
          <tr style={{ height: 20 }}>
            <th>
              <span style={{ color: "red" }}>No suitable missions</span>
            </th>
          </tr>
        ) : (
          draft.missionOptions.map((mission) => (
            <tr key={mission.id} style={{ height: 20 }}>
              <th>
                <input defaultChecked={mission.selected} name="order" type="radio" value={mission.id} />
                {mission.name}
                {mission.warning ? (
                  <>
                    <br />
                    <br />
                    <span style={{ color: "red" }}>{mission.warning}</span>
                  </>
                ) : null}
              </th>
            </tr>
          ))
        )}
      </tbody>
    </table>
  );
}

function FleetDispatchResourcesTable({ draft, fleet }: { draft: GameFleetDispatchDraft; fleet: GameFleet }) {
  const expeditionSelected = draft.missionOptions.some((mission) => mission.id === 15);
  return (
    <table border={0} cellPadding={0} cellSpacing={0} width={259}>
      <tbody>
        <tr style={{ height: 20 }}>
          <td className="legacy-c" colSpan={3}>
            Resources
          </td>
        </tr>
        {draft.resources.map((resource, index) => (
          <tr key={resource.id} style={{ height: 20 }}>
            <th>{resource.name}</th>
            <th>
              <a
                href="#max-resource"
                onClick={(event) => {
                  event.preventDefault();
                  setLegacyFleetResourceAmount(event.currentTarget, index + 1, resource.available);
                }}
              >
                max
              </a>
            </th>
            <th>
              <input
                aria-label={resource.name}
                data-resource-id={resource.id}
                defaultValue={0}
                name={`resource${index + 1}`}
                size={10}
                title={`${resource.name} ${formatLegacyNumber(resource.available)}`}
                type="text"
              />
            </th>
          </tr>
        ))}
        <tr style={{ height: 20 }}>
          <th>Residue</th>
          <th colSpan={2}>
            <div id="remainingresources">-</div>
          </th>
        </tr>
        <tr style={{ height: 20 }}>
          <th colSpan={3}>
            <a
              href="#max-resources"
              onClick={(event) => {
                event.preventDefault();
                setLegacyFleetAllResources(event.currentTarget, draft.resources);
              }}
            >
              All resources
            </a>
          </th>
        </tr>
        {draft.holdHours && draft.holdHours.length > 0 ? (
          <>
            <tr style={{ height: 20 }}>
              <td className="legacy-c" colSpan={3}>
                Hold time
              </td>
            </tr>
            <tr style={{ height: 20 }}>
              <th colSpan={3}>
                <select name="holdingtime" defaultValue={1}>
                  {draft.holdHours.map((hour) => (
                    <option key={hour} value={hour}>
                      {hour}
                    </option>
                  ))}
                </select>{" "}
                Time in hours
              </th>
            </tr>
          </>
        ) : null}
        {expeditionSelected && draft.expeditionHours && draft.expeditionHours.length > 0 ? (
          <>
            <tr style={{ height: 20 }}>
              <td className="legacy-c" colSpan={3}>
                Hold time
              </td>
            </tr>
            <tr style={{ height: 20 }}>
              <th colSpan={3}>
                <select name="expeditiontime">
                  {draft.expeditionHours.map((hour) => (
                    <option key={hour} value={hour}>
                      {hour}
                    </option>
                  ))}
                </select>{" "}
                Time in hours
              </th>
            </tr>
          </>
        ) : null}
        <tr style={{ height: 20 }}>
          <th colSpan={3}>
            {draft.ships.map((ship) => `${ship.name}: ${formatLegacyNumber(ship.count)}`).join(", ")}
            {draft.ships.length > 0 ? <br /> : null}
            {formatLegacyNumber(draft.totalShips)} ships, {formatLegacyNumber(draft.cargo)} capacity from {fleet.currentPlanet.name}
            <br />
            <span className="legacy-fleet-flight-math">
              Distance: <span id="distance">{formatLegacyNumber(draft.distance)}</span>, Duration:{" "}
              <span id="duration">{formatLegacyDuration(draft.durationSeconds)}</span>, Fuel consumption:{" "}
              <span id="consumption">{formatLegacyNumber(draft.fuelConsumption)}</span>, Max speed:{" "}
              <span id="maxspeed">{formatLegacyNumber(draft.maxSpeed)}</span>
            </span>
          </th>
        </tr>
      </tbody>
    </table>
  );
}

function collectLegacyFleetShips(form: HTMLFormElement): Record<string, number> {
  const ships: Record<string, number> = {};
  const formData = new FormData(form);
  for (const [key, value] of formData.entries()) {
    const match = /^ship(\d+)$/.exec(key);
    if (!match || typeof value !== "string") {
      continue;
    }
    const amount = Number.parseInt(value, 10);
    if (Number.isFinite(amount) && amount > 0) {
      ships[match[1]] = amount;
    }
  }
  return ships;
}

function collectLegacyFleetResources(form: HTMLFormElement): Record<string, number> {
  const resources: Record<string, number> = {};
  const formData = new FormData(form);
  for (const [key, value] of formData.entries()) {
    const match = /^resource\d+$/.exec(key);
    if (!match || typeof value !== "string") {
      continue;
    }
    const input = form.elements.namedItem(key);
    const resourceID = input instanceof HTMLInputElement ? input.dataset.resourceId : "";
    const amount = Number.parseInt(value, 10);
    if (resourceID && Number.isFinite(amount) && amount > 0) {
      resources[resourceID] = amount;
    }
  }
  return resources;
}

function fleetDraftShipsPayload(draft: GameFleetDispatchDraft): Record<string, number> {
  const ships: Record<string, number> = {};
  draft.ships.forEach((ship) => {
    ships[String(ship.id)] = ship.count;
  });
  return ships;
}

function setLegacyFleetResourceAmount(anchor: HTMLAnchorElement, resourceIndex: number, amount: number) {
  const form = anchor.closest("form");
  const input = form?.elements.namedItem(`resource${resourceIndex}`);
  if (input instanceof HTMLInputElement) {
    input.value = String(amount);
  }
}

function setLegacyFleetAllResources(anchor: HTMLAnchorElement, resources: GameFleetResourceLoad[]) {
  const form = anchor.closest("form");
  if (!form) {
    return;
  }
  resources.forEach((resource, index) => {
    const input = form.elements.namedItem(`resource${index + 1}`);
    if (input instanceof HTMLInputElement) {
      input.value = String(resource.available);
    }
  });
}

function setLegacyFleetShipAmount(anchor: HTMLAnchorElement, shipID: number, amount: number) {
  const form = anchor.closest("form");
  const input = form?.elements.namedItem(`ship${shipID}`);
  if (input instanceof HTMLInputElement) {
    input.value = String(amount);
  }
}

function setLegacyFleetShips(anchor: HTMLAnchorElement, ships: GameFleetShip[], mode: "all" | "none") {
  const form = anchor.closest("form");
  if (!form) {
    return;
  }
  for (const ship of ships) {
    const input = form.elements.namedItem(`ship${ship.id}`);
    if (input instanceof HTMLInputElement) {
      input.value = mode === "all" ? String(ship.count) : "0";
    }
  }
}

function fleetPlanetTypeName(type: number): string {
  switch (type) {
    case 2:
      return "Debris";
    case 3:
      return "Moon";
    default:
      return "Planet";
  }
}

function FleetTargetPrefillInputs({ prefill }: { prefill: GameFleetTargetPrefill }) {
  return (
    <>
      <input name="target_galaxy" type="hidden" value={prefill.targetGalaxy} />
      <input name="target_system" type="hidden" value={prefill.targetSystem} />
      <input name="target_planet" type="hidden" value={prefill.targetPlanet} />
      <input name="target_planettype" type="hidden" value={prefill.targetPlanetType} />
      <input name="target_mission" type="hidden" value={prefill.targetMission} />
    </>
  );
}

function FleetTemplatesTable({
  fleet,
  onAction,
  pending
}: {
  fleet: GameFleet;
  onAction: (action: "save" | "delete", templateID: number, name: string, ships: Record<string, number>) => void;
  pending: boolean;
}) {
  const selectableShips = fleet.ships.filter((ship) => ship.selectable && ship.id !== 212);
  const emptyDraft = React.useMemo<Record<string, number>>(
    () => Object.fromEntries(selectableShips.map((ship) => [String(ship.id), 0])),
    [selectableShips]
  );
  const [templateID, setTemplateID] = React.useState(0);
  const [name, setName] = React.useState("");
  const [ships, setShips] = React.useState<Record<string, number>>(emptyDraft);

  const editTemplate = (template: GameFleetTemplate) => {
    const next = { ...emptyDraft };
    for (const ship of template.ships) {
      next[String(ship.id)] = ship.count;
    }
    setTemplateID(template.id);
    setName(template.name);
    setShips(next);
  };

  return (
    <>
      <form
        action={gameRouteURL("/game/fleet-templates", window.location.search)}
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          onAction("save", templateID, name, ships);
        }}
      >
        <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-fleet-templates-table" width={519}>
          <tbody>
            <tr style={{ height: 20 }}>
              <td className="legacy-c" colSpan={4}>
                Standard fleets {fleet.templates.items.length} / {fleet.templates.max}
              </td>
            </tr>
            {!fleet.templates.commanderActive ? (
              <tr style={{ height: 20 }}>
                <th colSpan={4}>Commander is required</th>
              </tr>
            ) : null}
            {fleet.templates.items.map((template) => (
              <tr data-fleet-template-row={template.id} key={template.id} style={{ height: 20 }}>
                <th>{template.name}</th>
                <th>{template.ships.map((ship) => `${ship.name}: ${formatLegacyNumber(ship.count)}`).join(", ") || "-"}</th>
                <th>
                  <a
                    href="#edit-template"
                    onClick={(event) => {
                      event.preventDefault();
                      editTemplate(template);
                    }}
                  >
                    O
                  </a>
                </th>
                <th>
                  <button
                    disabled={pending}
                    onClick={(event) => {
                      event.preventDefault();
                      onAction("delete", template.id, template.name, {});
                    }}
                    type="button"
                  >
                    X
                  </button>
                </th>
              </tr>
            ))}
            <tr style={{ height: 20 }}>
              <td className="legacy-c" colSpan={4}>
                {templateID > 0 ? "Edit standard fleet" : "Create standard fleet"}
              </td>
            </tr>
            <tr style={{ height: 20 }}>
              <th>Name</th>
              <th colSpan={3}>
                <input name="template_id" type="hidden" value={templateID} />
                <input maxLength={20} name="template_name" onChange={(event) => setName(event.target.value)} size={30} type="text" value={name} />
              </th>
            </tr>
            {selectableShips.map((ship) => (
              <tr data-fleet-template-ship-row={ship.id} key={ship.id} style={{ height: 20 }}>
                <th>{ship.name}</th>
                <th>{formatLegacyNumber(ship.count)}</th>
                <th colSpan={2}>
                  <input
                    aria-label={ship.name}
                    max={ship.count}
                    min={0}
                    name={`ship[${ship.id}]`}
                    onChange={(event) => setShips((current) => ({ ...current, [String(ship.id)]: Number(event.target.value || 0) }))}
                    size={10}
                    type="number"
                    value={ships[String(ship.id)] ?? 0}
                  />
                </th>
              </tr>
            ))}
            <tr style={{ height: 20 }}>
              <th colSpan={2}>
                <input disabled={pending || !fleet.templates.commanderActive} type="submit" value="Save" />
              </th>
              <th colSpan={2}>
                <input
                  disabled={pending}
                  onClick={() => {
                    setTemplateID(0);
                    setName("");
                    setShips(emptyDraft);
                  }}
                  type="button"
                  value="Clear"
                />
              </th>
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

function fleetTemplateJavascriptHref(template: GameFleetTemplate): string {
  const args = template.ships.flatMap((ship) => [String(ship.id), String(ship.count)]).join(",");
  return `javascript:setShips(${args})`;
}

function setLegacyFleetTemplateShips(template: GameFleetTemplate) {
  for (const ship of template.ships) {
    const input = document.querySelector<HTMLInputElement>(`input[name="ship${ship.id}"]`);
    if (input) {
      input.value = String(ship.count);
    }
  }
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
        <table className="legacy-galaxy-nav-table legacy-header-table" id="t1">
          <tbody>
            <tr>
              <td className="legacy-header-cell">
                <table className="legacy-header-table" id="t2">
                  <tbody>
                    <tr>
                      <td className="legacy-c" colSpan={3}>
                        Galaxy
                      </td>
                    </tr>
                    <tr>
                      <td className="legacy-l">
                        <input
                          aria-label="Previous galaxy"
                          name="galaxyLeft"
                          onClick={() => navigateTo({ ...galaxy.coordinates, galaxy: galaxy.coordinates.galaxy - 1 })}
                          type="button"
                          value="<-"
                        />
                      </td>
                      <td className="legacy-l">
                        <input
                          aria-label="Galaxy"
                          defaultValue={galaxy.coordinates.galaxy}
                          maxLength={3}
                          name="galaxy"
                          size={5}
                          tabIndex={1}
                          type="text"
                        />
                      </td>
                      <td className="legacy-l">
                        <input
                          aria-label="Next galaxy"
                          name="galaxyRight"
                          onClick={() => navigateTo({ ...galaxy.coordinates, galaxy: galaxy.coordinates.galaxy + 1 })}
                          type="button"
                          value="->"
                        />
                      </td>
                    </tr>
                  </tbody>
                </table>
              </td>
              <td className="legacy-header-cell">
                <table className="legacy-header-table" id="t3">
                  <tbody>
                    <tr>
                      <td className="legacy-c" colSpan={3}>
                        Solar system
                      </td>
                    </tr>
                    <tr>
                      <td className="legacy-l">
                        <input
                          aria-label="Previous system"
                          name="systemLeft"
                          onClick={() => navigateTo({ ...galaxy.coordinates, system: galaxy.coordinates.system - 1 })}
                          type="button"
                          value="<-"
                        />
                      </td>
                      <td className="legacy-l">
                        <input
                          aria-label="Solar system"
                          defaultValue={galaxy.coordinates.system}
                          maxLength={3}
                          name="system"
                          size={5}
                          tabIndex={2}
                          type="text"
                        />
                      </td>
                      <td className="legacy-l">
                        <input
                          aria-label="Next system"
                          name="systemRight"
                          onClick={() => navigateTo({ ...galaxy.coordinates, system: galaxy.coordinates.system + 1 })}
                          type="button"
                          value="->"
                        />
                      </td>
                    </tr>
                  </tbody>
                </table>
              </td>
            </tr>
            <tr>
              <td className="legacy-header-cell legacy-galaxy-show-cell" colSpan={2}>
                <input type="submit" value="Show" />
              </td>
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
          <a className={row.moon.destroyed ? "legacy-galaxy-destroyed-moon" : undefined} href={fleetTargetHref(row.moon.coordinates, row.moon.coordinates.position, 3, 3)}>
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
  const playerID = planet.player?.id ?? 0;
  const actions = [
    { enabled: planet.actions.spy, href: fleetTargetHref(planet.coordinates, planet.coordinates.position, 6), icon: "e.gif", label: "Espionage" },
    { enabled: planet.actions.message && playerID > 0, href: gameMessageComposeURL(playerID, window.location.search), icon: "m.gif", label: "Write message" },
    { enabled: planet.actions.buddy && playerID > 0, href: gameBuddyRequestURL(playerID, window.location.search), icon: "b.gif", label: "Buddy request" },
    { enabled: planet.actions.missile, href: fleetTargetHref(planet.coordinates, planet.coordinates.position, 20), icon: "r.gif", label: "Rocket attack" }
  ];
  return (
    <>
      {actions.map((action) =>
        action.enabled ? (
          <a data-galaxy-action={action.label} href={action.href} key={action.icon}>
            <img alt={action.label} src={`${skinBase}/img/${action.icon}`} title={action.label} />
          </a>
        ) : null
      )}
    </>
  );
}

function DefenseTable({
  defense,
  onSubmit,
  pending
}: {
  defense: GameDefense;
  onSubmit: (orders: Record<string, number>) => void;
  pending: boolean;
}) {
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
    <form
      className="legacy-defense-form"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(collectLegacyUnitOrders(event.currentTarget));
      }}
    >
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
                    <input
                      aria-label={item.name}
                      defaultValue={0}
                      disabled={pending}
                      maxLength={6}
                      name={`fmenge[${item.id}]`}
                      size={6}
                      type="text"
                    />
                    {item.maxBuild > 0 ? (
                      <>
                        <br />
                        <a
                          href="#max"
                          onClick={(event) => {
                            event.preventDefault();
                            setLegacyUnitOrderMax(event.currentTarget, item.id, item.maxBuild);
                          }}
                        >
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
              <input disabled={pending} type="submit" value="Build" />
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

function BuddyTable({
  buddy,
  onAction,
  onRequest,
  pending
}: {
  buddy: GameBuddy;
  onAction: (action: number, buddyID: number) => void;
  onRequest: (buddyID: number, text: string) => void;
  pending: boolean;
}) {
  if (buddy.action === 7) {
    return <BuddyRequestTable buddy={buddy} onRequest={onRequest} pending={pending} />;
  }
  if (buddy.action === 5 || buddy.action === 6) {
    return <BuddyRequestsTable buddy={buddy} onAction={onAction} pending={pending} />;
  }
  return (
    <table className="legacy-overview-table legacy-buddy-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c" colSpan={6}>
            Buddylist
          </td>
        </tr>
        <tr>
          <th colSpan={6}>
            <a href={buddyURL({ action: 5 })}>Request</a>
          </th>
        </tr>
        <tr>
          <th colSpan={6}>
            <a href={buddyURL({ action: 6 })}>Your requests</a>
          </th>
        </tr>
        <tr>
          <td className="legacy-c">&nbsp;</td>
          <td className="legacy-c">Name</td>
          <td className="legacy-c">Alliance</td>
          <td className="legacy-c">Coords</td>
          <td className="legacy-c">Status</td>
          <td className="legacy-c">&nbsp;</td>
        </tr>
        {buddy.rows.length > 0 ? (
          buddy.rows.map((row, index) => (
            <tr data-buddy-row={row.buddyId} key={row.buddyId}>
              <th style={{ width: 20 }}>{index + 1}</th>
              <th>
                <a href={buddyMessageURL(row.player.playerId)}>{row.player.name}</a>
              </th>
              <th>{buddyAllianceLink(row.player)}</th>
              <th>
                <a href={buddyGalaxyURL(row.player.coordinates)}>{formatCoordinates(row.player.coordinates)}</a>
              </th>
              <th>
                <span style={{ color: row.status.color }}>{row.status.text}</span>
              </th>
              <th>
                <a
                  href={buddyURL({ action: 8, buddyID: row.buddyId })}
                  onClick={(event) => {
                    event.preventDefault();
                    if (!pending) {
                      onAction(8, row.buddyId);
                    }
                  }}
                >
                  delete
                </a>
              </th>
            </tr>
          ))
        ) : (
          <tr>
            <th colSpan={6}>No buddies found</th>
          </tr>
        )}
      </tbody>
    </table>
  );
}

function BuddyRequestsTable({
  buddy,
  onAction,
  pending
}: {
  buddy: GameBuddy;
  onAction: (action: number, buddyID: number) => void;
  pending: boolean;
}) {
  const incoming = buddy.action === 5;
  return (
    <table className="legacy-overview-table legacy-buddy-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c" colSpan={6}>
            {incoming ? "Request" : "Your requests"}
          </td>
        </tr>
        {buddy.rows.length > 0 ? (
          <>
            <tr>
              <th>&nbsp;</th>
              <th>User</th>
              <th>Alliance</th>
              <th>Coords</th>
              <th>Text</th>
              <th>&nbsp;</th>
            </tr>
            {buddy.rows.map((row, index) => (
              <tr data-buddy-row={row.buddyId} key={row.buddyId}>
                <th style={{ width: 20 }}>{index + 1}</th>
                <th>
                  <a href={buddyMessageURL(row.player.playerId)}>{row.player.name}</a>
                </th>
                <th>{buddyAllianceLink(row.player)}</th>
                <th>
                  <a href={buddyGalaxyURL(row.player.coordinates)}>{formatCoordinates(row.player.coordinates)}</a>
                </th>
                <th>{row.text}</th>
                <th style={{ width: 100 }}>
                  {incoming ? (
                    <>
                      <a
                        href={buddyURL({ action: 2, buddyID: row.buddyId })}
                        onClick={(event) => {
                          event.preventDefault();
                          if (!pending) {
                            onAction(2, row.buddyId);
                          }
                        }}
                      >
                        accept
                      </a>{" "}
                      <a
                        href={buddyURL({ action: 3, buddyID: row.buddyId })}
                        onClick={(event) => {
                          event.preventDefault();
                          if (!pending) {
                            onAction(3, row.buddyId);
                          }
                        }}
                      >
                        decline
                      </a>
                    </>
                  ) : (
                    <a
                      href={buddyURL({ action: 4, buddyID: row.buddyId })}
                      onClick={(event) => {
                        event.preventDefault();
                        if (!pending) {
                          onAction(4, row.buddyId);
                        }
                      }}
                    >
                      withdraw request
                    </a>
                  )}
                </th>
              </tr>
            ))}
          </>
        ) : (
          <tr>
            <th colSpan={6}>no entries</th>
          </tr>
        )}
        <tr>
          <td className="legacy-c" colSpan={6}>
            <a href={buddyURL({})}>back</a>
          </td>
        </tr>
      </tbody>
    </table>
  );
}

function BuddyRequestTable({
  buddy,
  onRequest,
  pending
}: {
  buddy: GameBuddy;
  onRequest: (buddyID: number, text: string) => void;
  pending: boolean;
}) {
  const target = buddy.target;
  if (!target) {
    return (
      <table className="legacy-overview-table legacy-buddy-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={2}>
              Buddy request
            </td>
          </tr>
          <tr>
            <th colSpan={2}>Player not found</th>
          </tr>
          <tr>
            <td className="legacy-c" colSpan={2}>
              <a href={buddyURL({})}>back</a>
            </td>
          </tr>
        </tbody>
      </table>
    );
  }
  return (
    <form
      action={buddyURL({ action: 1, buddyID: target.playerId })}
      method="post"
      onSubmit={(event) => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        onRequest(target.playerId, String(form.get("text") ?? ""));
      }}
    >
      <table className="legacy-overview-table legacy-buddy-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={2}>
              Buddy request
            </td>
          </tr>
          <tr>
            <th>Player</th>
            <th>{target.name}</th>
          </tr>
          <tr>
            <th>
              Request text(<span id="cntChars">0</span> / 5000 characters)
            </th>
            <th>
              <textarea cols={60} disabled={pending} name="text" rows={10} />
            </th>
          </tr>
          <tr>
            <td className="legacy-c">
              <a href={buddyURL({})}>back</a>
            </td>
            <td className="legacy-c">
              <input disabled={pending} type="submit" value="send" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function buddyAllianceLink(player: GameBuddyPlayer) {
  if (!player.alliance) {
    return <a href={gameRouteURL("/game/alliance", window.location.search)}>&nbsp;</a>;
  }
  return (
    <a href={gameRouteURL("/game/alliance", window.location.search)} target="_ally">
      {player.alliance.tag}
      {player.alliance.founder ? "  (G)" : ""}
    </a>
  );
}

function buddyURL({ action, buddyID }: { action?: number; buddyID?: number }): string {
  const query = new URLSearchParams(window.location.search);
  query.delete("action");
  query.delete("buddy_id");
  if (action !== undefined) {
    query.set("action", String(action));
  }
  if (buddyID !== undefined) {
    query.set("buddy_id", String(buddyID));
  }
  return gameRouteURL("/game/buddy", query.toString());
}

function buddyMessageURL(playerID: number): string {
  const query = new URLSearchParams(window.location.search);
  query.set("messageziel", String(playerID));
  return gameRouteURL("/game/messages", query.toString());
}

function buddyGalaxyURL(coordinates: Coordinates): string {
  return gameRouteURL("/game/galaxy", galaxyTargetSearch(coordinates));
}

function MessagesTable({
  messages,
  onDelete,
  onSend,
  pending
}: {
  messages: GameMessages;
  onDelete: (deleteMode: string, messageIDs: number[], reportIDs: number[]) => void;
  onSend: (targetPlayerID: number, subject: string, text: string) => void;
  pending: boolean;
}) {
  if (messages.action === "compose" && messages.compose) {
    return <MessageComposeTable compose={messages.compose} onSend={onSend} pending={pending} />;
  }
  const submitMessages = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const nativeEvent = event.nativeEvent as SubmitEvent;
    const submitter = nativeEvent.submitter instanceof HTMLInputElement ? nativeEvent.submitter : null;
    const deleteMode =
      submitter?.dataset.deleteMode ?? (submitter?.name === "deletemessages" ? submitter.value : String(data.get("deletemessages") ?? ""));
    const messageIDs: number[] = [];
    const reportIDs: number[] = [];
    for (const [key] of data) {
      const deleteMatch = /^delmes(\d+)$/.exec(key);
      if (deleteMatch) {
        messageIDs.push(Number(deleteMatch[1]));
      }
      const reportMatch = /^sneak(\d+)$/.exec(key);
      if (reportMatch) {
        reportIDs.push(Number(reportMatch[1]));
      }
    }
    onDelete(deleteMode, messageIDs, reportIDs);
  };
  return (
    <form
      action={gameRouteURL("/game/messages", window.location.search)}
      method="post"
      onSubmit={submitMessages}
    >
      <table className="legacy-overview-table legacy-messages-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={4}>
              Messages
            </td>
          </tr>
          <tr>
            <td className="legacy-c" width={20}>
              Action
            </td>
            <td className="legacy-c" width={150}>
              Date
            </td>
            <td className="legacy-c" width={129}>
              From
            </td>
            <td className="legacy-c" width={220}>
              Subject
            </td>
          </tr>
          {messages.rows.length > 0 ? (
            messages.rows.map((message) => (
              <React.Fragment key={message.id}>
                <tr data-message-row={message.id}>
                  <th>
                    <input disabled={pending} name={`delmes${message.id}`} type="checkbox" value="on" />
                  </th>
                  <th className={message.unread ? "legacy-message-unread" : undefined}>{formatLegacyMessageDate(message.date)}</th>
                  <th>
                    <LegacyMessageHTML html={message.from} />
                  </th>
                  <th>
                    <LegacyMessageHTML html={message.subject} />
                  </th>
                </tr>
                {message.text !== "" ? (
                  <tr>
                    <th className="legacy-message-text" colSpan={4}>
                      <LegacyMessageHTML html={message.text} />
                    </th>
                  </tr>
                ) : null}
                {message.reportable ? (
                  <tr>
                    <th colSpan={4}>
                      <input disabled={pending} name={`sneak${message.id}`} type="checkbox" />
                      <input disabled={pending} type="submit" value="Report" />
                    </th>
                  </tr>
                ) : null}
              </React.Fragment>
            ))
          ) : (
            <tr>
              <th colSpan={4}>There are no messages.</th>
            </tr>
          )}
          <tr>
            <th colSpan={4}>
              <select defaultValue="deletemarked" disabled={pending} name="deletemessages">
                <option value="deletemarked">delete marked messages</option>
                <option value="deletenonmarked">delete unmarked messages</option>
                <option value="deleteallshown">delete all shown messages</option>
              </select>
              <input disabled={pending} type="submit" value="Delete" />
              <input data-delete-mode="deleteall" disabled={pending} type="submit" value="Delete all messages" />
            </th>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function MessageComposeTable({
  compose,
  onSend,
  pending
}: {
  compose: GameMessageCompose;
  onSend: (targetPlayerID: number, subject: string, text: string) => void;
  pending: boolean;
}) {
  const targetText = `${compose.target.name} [${formatCoordinates(compose.target.coordinates)}]`;
  const submitMessage = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    onSend(compose.target.playerId, String(data.get("betreff") ?? ""), String(data.get("text") ?? ""));
  };
  return (
    <form
      action={gameRouteURL("/game/messages", window.location.search)}
      method="post"
      onSubmit={submitMessage}
    >
      <table className="legacy-overview-table legacy-messages-compose-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={2}>
              Write message
            </td>
          </tr>
          <tr>
            <th>Recipient</th>
            <th>
              <input name="to" readOnly size={40} type="text" value={targetText} />
            </th>
          </tr>
          <tr>
            <th>Subject</th>
            <th>
              <input defaultValue={compose.subject} disabled={pending} maxLength={40} name="betreff" size={40} type="text" />
            </th>
          </tr>
          <tr>
            <th colSpan={2}>
              <textarea cols={40} disabled={pending} maxLength={compose.maxChars} name="text" rows={10} />
            </th>
          </tr>
          <tr>
            <th colSpan={2}>
              <input name="messageziel" type="hidden" value={compose.target.playerId} />
              <input disabled={pending} type="submit" value="Send" />
            </th>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function LegacyMessageHTML({ html }: { html: string }) {
  return <span dangerouslySetInnerHTML={{ __html: sanitizeLegacyMessageHTML(html) }} />;
}

function ReportTable({ report }: { report: GameReport }) {
  return (
    <>
      <div id="overDiv" style={{ position: "absolute", visibility: "hidden", zIndex: 1000 }} />
      <table className="legacy-report-table" width="99%">
        <tbody>
          <tr>
            <td>
              {report.allowed && report.text !== "" ? <LegacyReportHTML html={report.text} /> : null}
            </td>
          </tr>
        </tbody>
      </table>
    </>
  );
}

function OptionsTable({
  onSubmit,
  options,
  pending
}: {
  onSubmit: (settings: {
    language: string;
    skinPath: string;
    useSkin: boolean;
    deactivateIp: boolean;
    sortBy: number;
    sortOrder: number;
    maxSpy: number;
    maxFleetMessages: number;
    deleteAccount: boolean;
  }) => void;
  options: GameOptions;
  pending: boolean;
}) {
  const submitOptions = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onSubmit({
      language: String(form.get("lang") ?? options.settings.language),
      skinPath: String(form.get("dpath") ?? ""),
      useSkin: form.get("design") === "on",
      deactivateIp: form.get("noipcheck") === "on",
      sortBy: legacyFormInt(form.get("settings_sort"), 0),
      sortOrder: legacyFormInt(form.get("settings_order"), 0),
      maxSpy: legacyFormInt(form.get("spio_anz"), 1),
      maxFleetMessages: legacyFormInt(form.get("settings_fleetactions"), 3),
      deleteAccount: form.get("db_deaktjava") === "on"
    });
  };

  return (
    <form action={gameRouteURL("/game/options", window.location.search)} method="POST" onSubmit={submitOptions}>
      <table className="legacy-overview-table legacy-options-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c" colSpan={2}>
              User Data
            </td>
          </tr>
          <tr>
            <th>{options.user.nameLocked ? <a title="The username can only be changed once every seven days.">Username</a> : "Username"}</th>
            <th>
              {options.user.nameLocked ? (
                options.user.name
              ) : (
                <input disabled name="db_character" readOnly size={20} type="text" value={options.user.name} />
              )}
              <br />
            </th>
          </tr>
          <tr>
            <th>Old password</th>
            <th>
              <input disabled name="db_password" size={20} type="password" />
            </th>
          </tr>
          <tr>
            <th>New password (min. 8 characters)</th>
            <th>
              <input disabled maxLength={40} name="newpass1" size={20} type="password" />
            </th>
          </tr>
          <tr>
            <th>New password (repeat)</th>
            <th>
              <input disabled maxLength={40} name="newpass2" size={20} type="password" />
            </th>
          </tr>
          <tr>
            <th>
              <a title="You can change this email address at any time. This will be entered as a permanent address after 7 days without changes.">
                Email address
              </a>
            </th>
            <th>
              <input disabled maxLength={100} name="db_email" readOnly size={20} type="text" value={options.user.email} />
            </th>
          </tr>
          <tr>
            <th>Permanent Address</th>
            <th>{options.user.plainEmail}</th>
          </tr>
          <tr>
            <th colSpan={2} />
          </tr>
          <tr>
            <td className="legacy-c" colSpan={2}>
              General Options
            </td>
          </tr>
          {!options.universe.forceLanguage ? (
            <tr>
              <th>Language:</th>
              <th>
                <select defaultValue={options.settings.language} name="lang">
                  {legacyLanguageOptions.map((language) => (
                    <option key={language.value} value={language.value}>
                      {language.label}
                    </option>
                  ))}
                </select>
              </th>
            </tr>
          ) : null}
          <tr>
            <th>Sort planets by:</th>
            <th>
              <select defaultValue={String(options.settings.sortBy)} name="settings_sort">
                <option value="0">Order of emergence</option>
                <option value="1">Coordinates</option>
                <option value="2">Alphabet</option>
              </select>
            </th>
          </tr>
          <tr>
            <th>Assortment sequence:</th>
            <th>
              <select defaultValue={String(options.settings.sortOrder)} name="settings_order">
                <option value="0">ascending</option>
                <option value="1">descending</option>
              </select>
            </th>
          </tr>
          <tr>
            <th>
              Skin path (e.g. C:/ogame/skin/)
              <br />{" "}
              <a href="/download/" rel="noreferrer" target="_blank">
                download
              </a>
            </th>
            <th>
              <input defaultValue={options.settings.skinPath} maxLength={80} name="dpath" size={40} type="text" />
              <br />
            </th>
          </tr>
          <tr>
            <th>Display skin</th>
            <th>
              <input defaultChecked={options.settings.useSkin} name="design" type="checkbox" />
            </th>
          </tr>
          <tr>
            <th>
              <a title="IP check means that a security logout occurs automatically when the IP changes or two people are logged into an account from different IPs. Disabling the IP check may represent a security risk!">
                Disable IP Check - GameOperator Authorization Required
              </a>
            </th>
            <th>
              <input defaultChecked={options.settings.deactivateIp} name="noipcheck" type="checkbox" />
            </th>
          </tr>
          <tr>
            <td className="legacy-c" colSpan={2}>
              Galaxy View Options
            </td>
          </tr>
          <tr>
            <th>
              <a title="Number of espionage probes that can be sent directly from each scan in the Galaxy menu.">
                Number of espionage probes
              </a>
            </th>
            <th>
              <input defaultValue={options.settings.maxSpy} maxLength={2} name="spio_anz" size={2} type="text" />
            </th>
          </tr>
          <tr>
            <th>Maximum fleet messages</th>
            <th>
              <input defaultValue={options.settings.maxFleetMessages} maxLength={2} name="settings_fleetactions" size={2} type="text" />
            </th>
          </tr>
          {options.user.commanderOn ? (
            <>
              <tr>
                <th>Action shortcuts</th>
                <th>Confirm</th>
              </tr>
              {legacyFlagRows(options.flags).map((row) => (
                <tr key={row.name}>
                  <th>
                    <img alt="" src={`${skinBase}/img/${row.icon}`} /> {row.label}
                  </th>
                  <th>
                    <input defaultChecked={row.checked} disabled name={row.name} type="checkbox" />
                  </th>
                </tr>
              ))}
              <tr>
                <td className="legacy-c" colSpan={2}>
                  Message Options
                </td>
              </tr>
              <tr>
                <th>No folder sorting</th>
                <th>
                  <input defaultChecked={options.flags.doNotUseFolders} disabled name="settings_folders" type="checkbox" />
                </th>
              </tr>
              <tr>
                <td className="legacy-c" colSpan={2}>
                  <span style={{ color: "#ff8900" }}>Newsfeed</span>
                </td>
              </tr>
              <tr>
                <th>{options.flags.feedEnabled ? "Activated" : "Activate"}</th>
                <th>
                  <input defaultChecked={options.flags.feedEnabled} disabled name="feed_activated" type="checkbox" />
                </th>
              </tr>
            </>
          ) : null}
          {options.user.admin === 1 ? (
            <>
              <tr>
                <td className="legacy-c" colSpan={2}>
                  Operator settings
                </td>
              </tr>
              <tr>
                <th>Hide Email on message page for players</th>
                <th>
                  <input defaultChecked={options.flags.hideGoEmail} disabled name="hide_go_email" type="checkbox" />
                </th>
              </tr>
            </>
          ) : null}
          <tr>
            <td className="legacy-c" colSpan={2}>
              Vacation mode / Delete account
            </td>
          </tr>
          <tr>
            <th>
              <a title="Vacation mode will protect you during long absences. It can only be activated if nothing is being built, researched, or flown.">
                Enable vacation mode
              </a>
            </th>
            <th>
              <input checked={options.account.vacation} disabled name="urlaubs_modus" readOnly type="checkbox" />
              {options.account.vacationUntil > 0 ? ` until ${formatLegacyTimestamp(options.account.vacationUntil)}` : null}
            </th>
          </tr>
          <tr>
            <th>
              <a title="If you mark this box, your account will be deleted automatically after 7 days.">Delete account</a>
            </th>
            <th>
              <input defaultChecked={options.account.deletionQueued} name="db_deaktjava" type="checkbox" />
              {options.account.deletionQueued && options.account.deletionAt > 0
                ? ` am: ${formatLegacyTimestamp(options.account.deletionAt)}`
                : null}
            </th>
          </tr>
          <tr>
            <th colSpan={2}>
              <input disabled={pending} type="submit" value="save changes" />
            </th>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

const legacyLanguageOptions = [
  { value: "de", label: "Deutsch" },
  { value: "en", label: "English" },
  { value: "es", label: "Español" },
  { value: "fr", label: "Français" },
  { value: "it", label: "Italiano" },
  { value: "jp", label: "日本語" },
  { value: "ru", label: "Русский" }
];

function legacyFlagRows(flags: GameOptionsFlags) {
  return [
    { name: "settings_esp", icon: "e.gif", label: "Espionage", checked: flags.showEspionageButton },
    { name: "settings_wri", icon: "m.gif", label: "Write message", checked: flags.showWriteMessage },
    { name: "settings_bud", icon: "b.gif", label: "Buddy request", checked: flags.showBuddy },
    { name: "settings_mis", icon: "r.gif", label: "Missile attack", checked: flags.showRocketAttack },
    { name: "settings_rep", icon: "s.gif", label: "View report", checked: flags.showViewReport }
  ];
}

function legacyFormInt(value: FormDataEntryValue | null, fallback: number): number {
  if (typeof value !== "string") {
    return fallback;
  }
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function formatLegacyTimestamp(unixSeconds: number): string {
  if (unixSeconds <= 0) {
    return "";
  }
  const date = new Date(unixSeconds * 1000);
  const pad = (value: number) => value.toString().padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function LegacyReportHTML({ html }: { html: string }) {
  return <div dangerouslySetInnerHTML={{ __html: sanitizeLegacyMessageHTML(html) }} />;
}

function sanitizeLegacyMessageHTML(value: string): string {
  if (typeof DOMParser === "undefined") {
    return value;
  }
  const doc = new DOMParser().parseFromString(`<div>${value}</div>`, "text/html");
  doc.querySelectorAll("script,style,iframe,object,embed,meta,link").forEach((node) => node.remove());
  doc.body.querySelectorAll("*").forEach((element) => {
    const reportHref = legacyReportHrefFromOnClick(element.getAttribute("onclick") ?? "");
    if (reportHref && element instanceof HTMLAnchorElement) {
      element.href = reportHref;
      element.removeAttribute("target");
    }
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase();
      const rawValue = attribute.value.trim().toLowerCase();
      if (name.startsWith("on") || ((name === "href" || name === "src" || name === "xlink:href") && rawValue.startsWith("javascript:"))) {
        element.removeAttribute(attribute.name);
      }
    }
  });
  return doc.body.innerHTML;
}

function legacyReportHrefFromOnClick(value: string): string | null {
  if (!value.toLowerCase().includes("page=bericht")) {
    return null;
  }
  const normalized = value.replace(/\\'/g, "'").replace(/\\"/g, '"').replace(/&amp;/g, "&").replace(/&#039;/g, "'");
  const match = /(?:index\.php\?)?page=bericht[^'")\s]*/i.exec(normalized);
  if (!match) {
    return null;
  }
  const rawQuery = match[0].includes("?") ? match[0].slice(match[0].indexOf("?") + 1) : match[0];
  const source = new URLSearchParams(rawQuery);
  const reportID = source.get("bericht");
  if (!reportID) {
    return null;
  }
  const query = new URLSearchParams(typeof window === "undefined" ? "" : window.location.search);
  query.set("bericht", reportID);
  const session = source.get("session") ?? query.get("session");
  if (session) {
    query.set("session", session);
  }
  return gameRouteURL("/game/report", query.toString());
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
            <td align="center" className="legacy-c legacy-technology-details-title" style={{ whiteSpace: "nowrap" }}>
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

function EmpireTable({ empire }: { empire: GameEmpire }) {
  const planets = empire.planets;
  const colSpan = planets.length + 2;
  const sumFields = planets.reduce((sum, planet) => sum + planet.fields, 0);
  const sumMaxFields = planets.reduce((sum, planet) => sum + planet.maxFields, 0);
  const avgFields = planets.length > 0 ? Math.ceil(sumFields / planets.length) : 0;
  const avgMaxFields = planets.length > 0 ? Math.ceil(sumMaxFields / planets.length) : 0;

  return (
    <div className="legacy-center">
      <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-empire-table" width={750}>
        <tbody>
          <tr style={{ height: 20 }}>
            <td className="legacy-c" colSpan={colSpan}>
              Empire Overview
            </td>
          </tr>
          {empire.moonEnabled && empire.hasMoons ? (
            <tr style={{ height: 20 }}>
              <th colSpan={Math.ceil(planets.length / 2)}>
                <a href={empirePlanetTypeURL(1)}>Planets</a>
              </th>
              <th colSpan={Math.ceil(planets.length / 2) + (1 - (planets.length % 2))}>
                <a href={empirePlanetTypeURL(3)}>Moons</a>
              </th>
              <th>&nbsp;</th>
            </tr>
          ) : null}
          <tr style={{ height: 75 }}>
            <th style={{ width: 75 }}></th>
            {planets.map((planet) => (
              <th key={planet.id} style={{ padding: 20, width: 75 }}>
                <a href={planetHref(planet.id)}>
                  <img alt="" height={71} src={planetImagePath(planet, false)} width={75} />
                </a>
              </th>
            ))}
            <th style={{ width: 75 }}>Sum</th>
          </tr>
          <tr style={{ height: 20 }}>
            <th style={{ width: 75 }}>Name</th>
            {planets.map((planet) => (
              <th key={planet.id} style={{ width: 75 }}>
                {planet.name}
              </th>
            ))}
            <th style={{ width: 75 }}>&nbsp;</th>
          </tr>
          <tr style={{ height: 20 }}>
            <th style={{ width: 75 }}>Coordinates</th>
            {planets.map((planet) => (
              <th key={planet.id} style={{ width: 75 }}>
                <a href={galaxyHref(planet.coordinates)}>[{formatCoordinates(planet.coordinates)}]</a>
              </th>
            ))}
            <th style={{ width: 75 }}>&nbsp;</th>
          </tr>
          <tr style={{ height: 20 }}>
            <th style={{ width: 75 }}>Fields</th>
            {planets.map((planet) => (
              <th key={planet.id} style={{ width: 75 }}>
                {planet.fields}/{planet.maxFields}
              </th>
            ))}
            <th style={{ width: 75 }}>
              {formatLegacyNumber(sumFields)} ({formatLegacyNumber(avgFields)}) / {formatLegacyNumber(sumMaxFields)} (
              {formatLegacyNumber(avgMaxFields)})
            </th>
          </tr>
          <EmpireSectionTitle colSpan={colSpan} title="Resources" />
          {empire.resources.map((row) => (
            <EmpireResourceRow key={row.id} planets={planets} row={row} />
          ))}
          <EmpireSectionTitle colSpan={colSpan} title="Buildings" />
          {empire.buildings.map((row) => (
            <EmpireLevelRow key={row.id} planets={planets} row={row} />
          ))}
          <EmpireSectionTitle colSpan={colSpan} title="Research" />
          {empire.research.map((row) => (
            <EmpireLevelRow key={row.id} planets={planets} row={row} />
          ))}
          <EmpireSectionTitle colSpan={colSpan} title="Fleet" />
          {empire.fleet.map((row) => (
            <EmpireCountRow key={row.id} planets={planets} row={row} />
          ))}
          <EmpireSectionTitle colSpan={colSpan} title="Defense" />
          {empire.defense.map((row) => (
            <EmpireCountRow key={row.id} planets={planets} row={row} />
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

function EmpireSectionTitle({ colSpan, title }: { colSpan: number; title: string }) {
  return (
    <tr style={{ height: 20 }}>
      <td align="left" className="legacy-c" colSpan={colSpan}>
        {title}
      </td>
    </tr>
  );
}

function EmpireResourceRow({ planets, row }: { planets: GameEmpirePlanet[]; row: GameEmpireResourceRow }) {
  const energy = row.id === 703;
  return (
    <tr data-empire-resource-row={row.id} style={{ height: 20 }}>
      <th style={{ width: 75 }}>{row.name}</th>
      {planets.map((planet) => {
        const value = empireResourceValue(row, planet.id);
        return (
          <th key={planet.id} style={{ width: 75 }}>
            {energy ? (
              <>
                <span style={{ color: value.amount < 0 ? "red" : undefined }}>{formatLegacyPlainNumber(value.amount)}</span> /{" "}
                {formatLegacyPlainNumber(value.production)}
              </>
            ) : (
              <a href={gameRouteURL("/game/resources", withPlanetSearch(planet.id))}>
                {formatLegacyPlainNumber(value.amount)} / {formatLegacyPlainNumber(value.production)}
              </a>
            )}
          </th>
        );
      })}
      <th style={{ width: 75 }}>
        {formatLegacyPlainNumber(row.total)} / {formatLegacyPlainNumber(row.production)}
      </th>
    </tr>
  );
}

function EmpireLevelRow({ planets, row }: { planets: GameEmpirePlanet[]; row: GameEmpireLevelRow }) {
  return (
    <tr data-empire-level-row={row.id} style={{ height: 20 }}>
      <th style={{ width: 75 }}>
        <a href={technologyInfoURL(row.id)}>{row.name}</a>
      </th>
      {planets.map((planet) => {
        const value = empireLevelValue(row, planet.id);
        return (
          <th key={planet.id} style={{ width: 75 }}>
            {value.level > 0 ? (
              <>
                <a href={gameRouteURL("/game/buildings", withPlanetSearch(planet.id))}>
                  <span style={{ color: "lime" }}>{formatLegacyPlainNumber(value.level)}</span>
                </a>
                <EmpireBuildQueueLinks planetID={planet.id} queue={value.queue ?? []} />
              </>
            ) : (
              <span style={{ color: "white" }}>-</span>
            )}
          </th>
        );
      })}
      <th style={{ width: 75 }}>
        {formatLegacyPlainNumber(row.total)} ({formatEmpireAverage(row.average)})
      </th>
    </tr>
  );
}

function EmpireBuildQueueLinks({ planetID, queue }: { planetID: number; queue: GameEmpireBuildQueueEntry[] }) {
  if (queue.length === 0) {
    return null;
  }
  return (
    <>
      {queue.map((entry) =>
        entry.active ? (
          <React.Fragment key={entry.listId}>
            {" "}
            <a href={empireQueueRemoveURL(planetID, entry.listId)}>
              <span style={{ color: "magenta" }}>{formatLegacyPlainNumber(entry.level)}</span>
            </a>
          </React.Fragment>
        ) : (
          <span key={entry.listId} style={{ color: "sandybrown" }}>
            {" "}
            (
            <a href={empireQueueRemoveURL(planetID, entry.listId)}>
              <span style={{ color: "sandybrown" }}>{formatLegacyPlainNumber(entry.level)}</span>
            </a>
            )
          </span>
        )
      )}
    </>
  );
}

function EmpireCountRow({ planets, row }: { planets: GameEmpirePlanet[]; row: GameEmpireCountRow }) {
  return (
    <tr data-empire-count-row={row.id} style={{ height: 20 }}>
      <th style={{ width: 75 }}>
        <a href={technologyInfoURL(row.id)}>{row.name}</a>
      </th>
      {planets.map((planet) => {
        const value = empireCountValue(row, planet.id);
        return (
          <th key={planet.id} style={{ width: 75 }}>
            {value.count > 0 ? (
              <a href={gameRouteURL("/game/shipyard", withPlanetSearch(planet.id))}>
                <span style={{ color: "lime" }}>{formatLegacyPlainNumber(value.count)}</span>
              </a>
            ) : (
              <span style={{ color: "white" }}>-</span>
            )}
          </th>
        );
      })}
      <th style={{ width: 75 }}>{formatLegacyPlainNumber(row.total)}</th>
    </tr>
  );
}

function empirePlanetTypeURL(planetType: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("planettype", String(planetType));
  return gameRouteURL("/game/empire", search.toString());
}

function withPlanetSearch(planetID: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("cp", String(planetID));
  return search.toString();
}

function empireQueueRemoveURL(planetID: number, listID: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("planet", String(planetID));
  search.set("modus", "remove");
  search.set("listid", String(listID));
  return gameRouteURL("/game/empire", search.toString());
}

function empireResourceValue(row: GameEmpireResourceRow, planetID: number): GameEmpireResourceValue {
  return row.values.find((value) => value.planetId === planetID) ?? { planetId: planetID, amount: 0, production: 0 };
}

function empireLevelValue(row: GameEmpireLevelRow, planetID: number): GameEmpireLevelValue {
  return row.values.find((value) => value.planetId === planetID) ?? { planetId: planetID, level: 0, queue: [] };
}

function empireCountValue(row: GameEmpireCountRow, planetID: number): GameEmpireCountValue {
  return row.values.find((value) => value.planetId === planetID) ?? { planetId: planetID, count: 0 };
}

function formatEmpireAverage(value: number): string {
  if (Number.isInteger(value)) {
    return formatLegacyPlainNumber(value);
  }
  return value.toFixed(2).replace(/0+$/, "").replace(/\.$/, "");
}

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
                <input disabled={pending} name="action" type="submit" value="Recalculate" />
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
    <table className="legacy-overview-table legacy-overview-main-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c" colSpan={4}>
            <a href={gameRouteURL("/game/rename-planet", window.location.search)} title="Planet menu">
              Planet "{planet.name}"
            </a>{" "}
            ({overview.commander})
          </td>
        </tr>
        {overview.unreadMessages > 0 ? (
          <tr>
            <th colSpan={4}>
              <a href={gameRouteURL("/game/messages", window.location.search)}>
                {overviewUnreadMessageText(overview.unreadMessages)}
              </a>
            </th>
          </tr>
        ) : null}
        <tr>
          <th>Server time</th>
          <th colSpan={3}>{overview.serverTime || formatLegacyDate(new Date())}</th>
        </tr>
        <tr>
          <td className="legacy-c" colSpan={4}>
            Events
          </td>
        </tr>
        <OverviewEventRows events={overview.events ?? []} />
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
            <div className="legacy-center">{overviewBuildQueueText(planet.buildQueue, true)}</div>
            <br />
          </th>
          <th className="legacy-s">
            <table className="legacy-planet-list">
              <tbody>
                {otherPlanets.length === 0
                  ? null
                  : rowsOfTwo(otherPlanets).map((row, index) => (
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
                          <div className="legacy-center">{overviewBuildQueueText(item.buildQueue, false)}</div>
                        </th>
                      ))}
                    </tr>
                  ))}
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
            <a className="legacy-overview-position-link" href={galaxyHref(planet.coordinates)}>
              [{formatCoordinates(planet.coordinates)}]
            </a>
          </th>
        </tr>
        <tr>
          <th>Points</th>
          <th colSpan={3}>
            {formatLegacyNumber(overview.score.points)} (Rank{" "}
            <a className="legacy-overview-rank-link" href={overviewRankHref(overview.score.rank)}>
              {formatLegacyNumber(overview.score.rank)}
            </a>{" "}
            of {formatLegacyNumber(overview.score.universePlayers)}
            )
          </th>
        </tr>
      </tbody>
    </table>
  );
}

function OverviewEventRows({ events }: { events: GameFleetMission[] }) {
  if (events.length === 0) {
    return null;
  }
  const now = Math.floor(Date.now() / 1000);
  return (
    <>
      {events.map((event) => {
        const remaining = Math.max(0, event.arrivalAt - now);
        const groupMissions = overviewEventGroupMissions(event);
        return (
          <tr className={overviewEventRowClass(event)} key={event.id}>
            <th>
              <div title={String(remaining)}>{formatLegacyDuration(remaining)}</div>
            </th>
            <th colSpan={3}>
              {groupMissions.map((groupEvent, index) => (
                <React.Fragment key={groupEvent.id}>
                  {index > 0 ? (
                    <>
                      <br />
                      <br />
                    </>
                  ) : null}
                  <span className={overviewEventMissionClass(groupEvent)}>
                    <OverviewEventBody event={groupEvent} />
                  </span>
                </React.Fragment>
              ))}
            </th>
          </tr>
        );
      })}
    </>
  );
}

function overviewEventGroupMissions(event: GameFleetMission): GameFleetMission[] {
  return event.groupMissions.length > 0 ? event.groupMissions : [event];
}

function OverviewEventBody({ event }: { event: GameFleetMission }) {
  if (overviewEventBaseMission(event.mission) === 20) {
    return <OverviewMissileEventBody event={event} />;
  }
  return (
    <>
      <a title={overviewEventShipTitle(event)}>{overviewEventFleetLabel(event)}</a> {overviewEventDirectionText(event)}{" "}
      <a href={galaxyHref(event.origin)}>[{formatCoordinates(event.origin)}]</a> {overviewEventTargetText(event)}{" "}
      <a href={galaxyHref(event.target)}>[{formatCoordinates(event.target)}]</a>. Mission: {event.missionName}
    </>
  );
}

function OverviewMissileEventBody({ event }: { event: GameFleetMission }) {
  return (
    <>
      Rocket Attack ({formatLegacyNumber(event.missileAmount)}) from{" "}
      <a href={galaxyHref(event.origin)}>[{formatCoordinates(event.origin)}]</a> to{" "}
      <a href={galaxyHref(event.target)}>[{formatCoordinates(event.target)}]</a>
      {event.missileTargetId > 0 ? ` Main target ${event.missileTarget || `NAME_${event.missileTargetId}`}` : ""}
    </>
  );
}

function overviewEventRowClass(event: GameFleetMission): string {
  if (event.groupMissions.length > 0) {
    return "";
  }
  if (event.mission >= 200) {
    return "holding";
  }
  if (event.mission >= 100) {
    return "return";
  }
  return "flight";
}

function overviewEventMissionClass(event: GameFleetMission): string {
  const own = event.own !== false;
  switch (overviewEventBaseMission(event.mission)) {
    case 1:
    case 21:
      return own ? "ownattack" : "attack";
    case 2:
      return own ? "ownfederation" : "federation";
    case 4:
      return own ? "owndeploy" : "deploy";
    case 5:
      return own ? "ownhold" : "hold";
    case 6:
      return own ? "ownespionage" : "espionage";
    case 7:
      return own ? "owncolony" : "colony";
    case 8:
      return own ? "ownharvest" : "harvest";
    case 9:
      return own ? "owndestroy" : "destroy";
    case 20:
      return own ? "ownmissile" : "missile";
    default:
      return own ? "owntransport" : "transport";
  }
}

function overviewEventBaseMission(mission: number): number {
  if (mission >= 200) {
    return mission - 200;
  }
  if (mission >= 100) {
    return mission - 100;
  }
  return mission;
}

function overviewEventDirectionText(event: GameFleetMission): string {
  if (event.mission >= 100 && event.mission < 200) {
    return "returns from";
  }
  if (event.mission >= 200) {
    return "holds from";
  }
  return "from";
}

function overviewEventTargetText(event: GameFleetMission): string {
  if (event.mission >= 100 && event.mission < 200) {
    return "to";
  }
  if (event.mission >= 200) {
    return "onto";
  }
  return "sent to";
}

function overviewEventFleetLabel(event: GameFleetMission): string {
  const count = formatLegacyNumber(event.totalShips);
  if (event.own !== false) {
    return `Your ${count} fleet`;
  }
  const owner = event.ownerName.trim() || "Enemy";
  return `${owner}'s ${count} fleet`;
}

function overviewEventShipTitle(event: GameFleetMission): string {
  return event.ships.map((ship) => `${ship.name}: ${formatLegacyNumber(ship.count)}`).join("\n");
}

function overviewBuildQueueText(queue: GameOverviewBuildQueue | undefined, includeLevel: boolean): string {
  if (!queue) {
    return "free";
  }
  if (!includeLevel) {
    return queue.name;
  }
  const level = queue.destroy ? queue.level + 1 : queue.level;
  return `${queue.name}${queue.destroy ? " Снести" : ""} (${level})`;
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

function overviewRankHref(rank: number): string {
  const search = new URLSearchParams(window.location.search);
  const start = Math.floor(Math.max(0, rank) / 100) * 100 + 1;
  search.set("start", String(start));
  return gameRouteURL("/game/statistics", search.toString());
}

function galaxyHref(coordinates: Coordinates): string {
  const search = new URLSearchParams(window.location.search);
  search.set("galaxy", String(coordinates.galaxy));
  search.set("system", String(coordinates.system));
  search.set("position", String(coordinates.position));
  return gameRouteURL("/game/galaxy", search.toString());
}

function fleetTargetHref(coordinates: Coordinates, position: number, mission: number, planetType = 1): string {
  return gameFleetTargetURL(
    {
      galaxy: coordinates.galaxy,
      system: coordinates.system,
      position,
      mission,
      planetType
    },
    window.location.search
  );
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

function planetImagePath(planet: Pick<GamePlanetOverview, "id" | "type" | "coordinates">, small: boolean): string {
  if (planet.type === 0) {
    return `${skinBase}/planeten/${small ? "small/s_" : ""}mond.jpg`;
  }
  const imageID = (planet.id % 7) + 1;
  const category = planetCategory(planet.coordinates.position);
  const filename = `${category}${String(imageID).padStart(2, "0")}.jpg`;
  return `${skinBase}/planeten/${small ? "small/s_" : ""}${filename}`;
}

function overviewUnreadMessageText(count: number): string {
  return `You have ${count} new message${count > 1 ? "s" : ""}`;
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
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(
    date.getUTCDate()
  ).padStart(2, "0")} ${String(date.getUTCHours()).padStart(2, "0")}:${String(date.getUTCMinutes()).padStart(
    2,
    "0"
  )}:${String(date.getUTCSeconds()).padStart(2, "0")}`;
}

function formatLegacyMessageDate(seconds: number): string {
  const date = new Date(seconds * 1000);
  return `${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")} ${String(
    date.getUTCHours()
  ).padStart(2, "0")}:${String(date.getUTCMinutes()).padStart(2, "0")}:${String(date.getUTCSeconds()).padStart(2, "0")}`;
}

function formatLegacyStatisticsDateTime(seconds: number): string {
  return formatLegacyDateTime(seconds).replace(" ", ", ");
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
