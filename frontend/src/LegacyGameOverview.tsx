import React from "react";
import {
  gameBuddyRequestURL,
  gameFleetTargetPrefillFromSearch,
  gameFleetTargetURL,
  gameGalaxyMissileURL,
  gameMenuRouteURL,
  gameMessageComposeURL,
  gamePlanetSwitchURL,
  gameRouteURL,
  gameRoutes,
  type GameFleetTargetPrefill,
  type GameRoute
} from "./gameRoutes";
import { legacyChangelogRows } from "./legacyChangelogData";

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

export type GameMerchantStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  merchant?: GameMerchant;
};

export type GameMerchantTradeValues = {
  metal: number;
  crystal: number;
  deuterium: number;
};

export type GameOfficersStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  officers?: GameOfficers;
};

export type GameOfficerRecruitment = {
  officerId: number;
  days: number;
};

export type GameAllianceStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  alliance?: GameAlliance;
};

export type GameAdminStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  actionIssue?: { code: string; message: string };
  admin?: GameAdmin;
};

export type GameAdminAction =
  | {
      action: "ban";
      targetIds: number[];
      banMode: number;
      days: number;
      hours: number;
      reason: string;
    }
  | {
      action: "settings";
      values: Record<string, number>;
    }
  | {
      action: "broadcast_send";
      category: number;
      subject: string;
      text: string;
    }
  | {
      action: "reports_delete";
      reportIds: number[];
      deleteMode: string;
    }
  | {
      action: "queue_end" | "queue_remove" | "queue_freeze" | "queue_unfreeze";
      taskId: number;
    }
  | {
      action: "fleetlogs_2min" | "fleetlogs_end" | "fleetlogs_return";
      taskId: number;
    }
  | {
      action: "add_one";
      amount: number;
    }
  | {
      action: "remove_one" | "remove_date";
      itemId: number;
    }
  | {
      action: "add_date";
      amount: number;
      dayMonth: string;
      hourMinute: string;
      inactiveDays: number;
      ingameDays: number;
      periodicDays: number;
    };

export type GameAllianceAction =
  | { action: "create"; tag: string; name: string }
  | { action: "search"; text: string }
  | { action: "apply"; allianceId: number; text: string }
  | { action: "withdraw"; applicationId: number }
  | { action: "accept"; applicationId: number }
  | { action: "reject"; applicationId: number; text: string }
  | { action: "leave" }
  | { action: "save_text"; textKind: number; text: string; insertApp: boolean }
  | { action: "save_settings"; homepage: string; imageLogo: string; open: boolean; founderRankName: string }
  | { action: "add_rank"; rankName: string }
  | { action: "save_ranks"; rankRights: { id: number; rights: number }[] }
  | { action: "delete_rank"; rankId: number }
  | { action: "assign_rank"; targetPlayerId: number; targetRankId: number }
  | { action: "kick_member"; targetPlayerId: number }
  | { action: "send_circular"; circularRankId: number; text: string };

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
  actionIssue?: { code: string; message: string };
  galaxy?: GameGalaxy;
};

export type GameGalaxyMissileLaunch = {
  targetPlanetId: number;
  amount: number;
  targetDefenseId: number;
};

export type GameGalaxyInstantDispatch = {
  action: "dispatch-spy" | "dispatch-recycle";
  target: Coordinates;
  targetType: number;
  amount: number;
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

export type GamePhalanxStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  phalanx?: GamePhalanx;
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
  adminLevel: number;
  serverTime?: string;
  officers?: {
    commander: boolean;
    admiral: boolean;
    engineer: boolean;
    geologist: boolean;
    technocrat: boolean;
  };
  news?: GameOverviewNews;
  menuLinks?: GameOverviewMenuLinks;
  messages?: string[];
  errors?: string[];
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

type GameOverviewNews = {
  url: string;
  start: string;
  end: string;
};

type GameOverviewMenuLinks = {
  boardUrl?: string;
  discordUrl?: string;
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
  darkMatter: number;
  energy: number;
  energyCapacity: number;
  metalCapacity: number;
  crystalCapacity: number;
  deuteriumCapacity: number;
};

type GameBuildings = {
  commander: string;
  commanderActive: boolean;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  queue: GameBuildingQueueEntry[];
  items: GameBuildingItem[];
};

type GameBuildingQueueEntry = {
  listId: number;
  techId: number;
  name: string;
  level: number;
  destroy: boolean;
  start: number;
  end: number;
  remainingSeconds: number;
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
  commanderActive: boolean;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  hasShipyard: boolean;
  busy: boolean;
  queue: GameShipyardQueueEntry[];
  items: GameShipyardItem[];
};

type GameShipyardQueueEntry = {
  taskId: number;
  unitId: number;
  name: string;
  count: number;
  start: number;
  end: number;
  remainingSeconds: number;
};

type GameDefense = {
  commander: string;
  commanderActive: boolean;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  hasShipyard: boolean;
  busy: boolean;
  queue: GameShipyardQueueEntry[];
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
    maxSpy: number;
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
  loadedResources?: Record<string, number>;
  missileAmount: number;
  missileTargetId: number;
  missileTarget: string;
  unionId: number;
  unionName?: string;
  unionPlayers?: { id: number; name: string }[];
  groupMissions: GameFleetMission[];
  origin: Coordinates;
  originName: string;
  target: Coordinates;
  targetName: string;
  targetType: number;
  targetOwnerName: string;
  departureAt: number;
  arrivalAt: number;
  canRecall: boolean;
  canCreateUnion: boolean;
};

type GamePhalanx = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  source: GamePhalanxPlanet;
  target: GamePhalanxPlanet;
  events: GameFleetMission[];
  actionIssue?: { code: string; message: string };
  cost: number;
  remainingDeuterium: number;
  reportHeading: string;
  eventsHeading: string;
};

type GamePhalanxPlanet = {
  id: number;
  ownerId: number;
  name: string;
  type: number;
  coordinates: Coordinates;
  phalanxLevel: number;
  deuterium: number;
};

type GameStatistics = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  viewerAllianceId: number;
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
  speed: number;
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
  info?: GameTechnologyInfo;
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
  demolish?: GameTechnologyDemolish;
};

type GameTechnologyDetailsLevel = {
  step: number;
  requirements: GameTechnologyRequirement[];
};

type GameTechnologyDemolish = {
  level: number;
  cost: BuildingCost;
  durationSeconds: number;
};

type GameTechnologyInfo = {
  id: number;
  name: string;
  description: string;
  level: number;
  kind: "mine" | "solar" | "fusion" | "storage" | "description";
  rows: GameTechnologyInfoRow[];
};

type GameTechnologyInfoRow = {
  level: number;
  current: boolean;
  production: number;
  productionDifference: number;
  energy: number;
  energyDifference: number;
  storage: number;
  storageDifference: number;
  deuteriumConsumption: number;
  deuteriumDifference: number;
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

type GameMerchant = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  user: {
    paidDarkMatter: number;
    freeDarkMatter: number;
  };
  activeOfferId: number;
  rates: {
    metal: number;
    crystal: number;
    deuterium: number;
  };
  rows: GameMerchantResourceRow[];
};

type GameMerchantResourceRow = {
  id: number;
  name: string;
  offered: boolean;
  value: number;
  freeStorage: number;
  rate: number;
};

type GameOfficers = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  user: {
    paidDarkMatter: number;
    freeDarkMatter: number;
  };
  rows: GameOfficerRow[];
};

type GameOfficerRow = {
  id: number;
  key: string;
  name: string;
  description: string;
  note: string;
  image: string;
  icon: string;
  active: boolean;
  until: number;
  daysLeft: number;
  weekCost: number;
  threeMonthCost: number;
};

type GameAlliance = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  view: string;
  viewer: GameAllianceViewer;
  own?: GameAllianceInfo;
  target?: GameAllianceInfo;
  pending?: GameAllianceApplication;
  searchText: string;
  textKind: number;
  searchResults: GameAllianceSearchResult[];
  applications: GameAllianceApplication[];
  selectedApp?: GameAllianceApplication;
  members: GameAllianceMember[];
  ranks: GameAllianceRank[];
  circularResult?: { recipients: string[] };
};

type GameAllianceViewer = {
  playerId: number;
  name: string;
  validated: boolean;
  allianceId: number;
  rankId: number;
  rankName: string;
  rankRights: number;
  founder: boolean;
};

type GameAllianceInfo = {
  id: number;
  tag: string;
  name: string;
  ownerId: number;
  homepage: string;
  imageLogo: string;
  open: boolean;
  insertApp: boolean;
  externalText: string;
  internalText: string;
  applicationText: string;
  oldTag: string;
  oldName: string;
  tagUntil: number;
  nameUntil: number;
  memberCount: number;
  applicationCount: number;
};

type GameAllianceSearchResult = {
  id: number;
  tag: string;
  name: string;
  memberCount: number;
};

type GameAllianceApplication = {
  id: number;
  allianceId: number;
  playerId: number;
  playerName: string;
  text: string;
  date: number;
};

type GameAllianceMember = {
  playerId: number;
  name: string;
  rankId: number;
  rankName: string;
  score: number;
  joinedAt: number;
  lastClick: number;
  galaxy: number;
  system: number;
  position: number;
};

type GameAllianceRank = {
  id: number;
  name: string;
  rights: number;
};

type GameAdmin = {
  commander: string;
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
  viewer: GameAdminViewer;
  mode: string;
  menu: GameAdminMenuItem[];
  messageRows?: GameAdminMessageRow[];
  userLogRows?: GameAdminUserLogRow[];
  userRows?: GameAdminUserRow[];
  activeUsers?: GameAdminUserRow[];
  selectedUser?: GameAdminUserDetail;
  planetRows?: GameAdminPlanetRow[];
  selectedPlanet?: GameAdminPlanetDetail;
  reportRows?: GameAdminReportRow[];
  universe?: GameAdminUniverseSettings;
  expedition?: Record<string, number>;
  fleetLogRows?: GameAdminFleetLogRow[];
  queueRows?: GameAdminQueueRow[];
  battleReports?: GameAdminBattleReportRow[];
  checksumGroups?: GameAdminChecksumGroup[];
  databaseBackups?: GameAdminDatabaseBackup[];
  botStrategies?: GameAdminBotStrategy[];
  couponRows?: GameAdminCouponRow[];
  couponQueueRows?: GameAdminCouponQueueRow[];
};

type GameAdminViewer = {
  playerId: number;
  name: string;
  level: number;
};

type GameAdminMenuItem = {
  mode: string;
  label: string;
  image: string;
};

type GameAdminMessageRow = {
  id: number;
  ownerId: number;
  ownerName: string;
  ip: string;
  agent: string;
  text: string;
  date: number;
};

type GameAdminUserLogRow = {
  id: number;
  ownerId: number;
  ownerName: string;
  lastClick: number;
  vacation: boolean;
  banned: boolean;
  noAttack: boolean;
  disable: boolean;
  type: string;
  text: string;
  date: number;
};

type GameAdminUserRow = {
  playerId: number;
  name: string;
  regDate: number;
  lastClick: number;
  vacation: boolean;
  banned: boolean;
  noAttack: boolean;
  disable: boolean;
  homePlanet?: GameAdminUserPlanet;
};

type GameAdminUserPlanet = {
  id: number;
  name: string;
  coordinates: Coordinates;
};

type GameAdminUserDetail = GameAdminUserRow & {
  permanentEmail: string;
  email: string;
  alliance: string;
  joinDate: number;
  disableUntil: number;
  vacationUntil: number;
  bannedUntil: number;
  noAttackUntil: number;
  lastLogin: number;
  ipAddress: string;
  validated: boolean;
  adminLevel: number;
  sniff: boolean;
  debug: boolean;
  sortBy: number;
  sortOrder: number;
  skin: string;
  useSkin: boolean;
  deactivateIP: boolean;
  maxSpy: number;
  maxFleetMsg: number;
  oldScore1: number;
  oldPlace1: number;
  oldScore2: number;
  oldPlace2: number;
  oldScore3: number;
  oldPlace3: number;
  score1: number;
  place1: number;
  score2: number;
  place2: number;
  score3: number;
  place3: number;
  scoreDate: number;
  darkMatterFree: number;
  darkMatter: number;
  research: Record<string, number>;
  activePlanet?: GameAdminUserPlanet;
  planets: GameAdminPlanetRow[];
};

type GameAdminPlanetRow = {
  id: number;
  name: string;
  date: number;
  coordinates: Coordinates;
  owner?: GameAdminUserRow;
};

type GameAdminTechnologyValue = {
  id: number;
  name: string;
  value: number;
  percent: number;
};

type GameAdminPlanetDetail = GameAdminPlanetRow & {
  type: number;
  diameter: number;
  temperature: number;
  fields: number;
  maxFields: number;
  removeDate: number;
  lastActivity: number;
  lastUpdate: number;
  gateUntil: number;
  score: {
    points: number;
    fleetPoints: number;
    defensePoints: number;
  };
  resources: Resources;
  energyBalance: number;
  energyCapacity: number;
  productionFactor: number;
  buildings: GameAdminTechnologyValue[];
  fleet: GameAdminTechnologyValue[];
  defense: GameAdminTechnologyValue[];
  buildQueue: GameOverviewBuildQueue[];
  moon?: GameAdminPlanetRow;
  debris?: GameAdminPlanetRow;
};

type GameAdminReportRow = {
  id: number;
  ownerId: number;
  ownerName: string;
  messageId: number;
  from: string;
  subject: string;
  text: string;
  date: number;
};

type GameAdminFleetLogRow = {
  taskId: number;
  number: number;
  mission: number;
  start: number;
  end: number;
  flightTime: number;
  fuel: number;
  unionId: number;
  origin: GameAdminFleetLogPlanet;
  target: GameAdminFleetLogPlanet;
  ships: GameFleetShipCount[];
  cargo: GameFleetResourceLoad[];
};

type GameAdminFleetLogPlanet = {
  id: number;
  name: string;
  ownerId: number;
  ownerName: string;
  coordinates: Coordinates;
  type: number;
};

type GameAdminUniverseSettings = {
  number: number;
  speed: number;
  fleetSpeed: number;
  galaxies: number;
  systems: number;
  maxUsers: number;
  acs: number;
  fleetDebris: number;
  defenseDebris: number;
  rapidFire: boolean;
  moons: boolean;
  defenseRepair: number;
  defenseDelta: number;
  userCount: number;
  freeze: boolean;
  news1: string;
  news2: string;
  newsUntil: number;
  startDate: number;
  battleEngine: string;
  language: string;
  hacks: number;
  extBoard: string;
  extDiscord: string;
  extTutorial: string;
  extRules: string;
  extImpressum: string;
  phpBattle: boolean;
  battleMax: number;
  forceLanguage: boolean;
  startDarkMatter: number;
  maxShipyard: number;
  feedAge: number;
};

type GameAdminQueueRow = {
  id: number;
  ownerId: number;
  ownerName: string;
  type: string;
  description: string;
  priority: number;
  start: number;
  end: number;
  freeze: boolean;
  frozen: number;
};

type GameAdminBattleReportRow = {
  id: number;
  date: number;
  title: string;
};

type GameAdminChecksumGroup = {
  title: string;
  rows: GameAdminChecksumRow[];
};

type GameAdminChecksumRow = {
  path: string;
  checksum: string;
  status: string;
};

type GameAdminDatabaseBackup = {
  fileName: string;
};

type GameAdminBotStrategy = {
  id: number;
  name: string;
};

type GameAdminCouponRow = {
  id: number;
  code: string;
  amount: number;
  used: boolean;
  userUniverse: number;
  userId: number;
  userName: string;
};

type GameAdminCouponQueueRow = {
  id: number;
  amount: number;
  inactiveDays: number;
  ingameDays: number;
  periodicDays: number;
  start: number;
  end: number;
  priority: number;
};

type ResourceProductionRow = {
  id: number;
  name: string;
  level: number;
  percent: number;
  values: ResourceProductionValues;
  bonusIcons?: ResourceProductionBonusIcon[];
};

type ResourceProductionBonusIcon = {
  image: string;
  alt: string;
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
  onOverviewRefresh: () => void;
  onPlanetDelete: (password: string, deleteID: number) => void;
  onPlanetRename: (name: string) => void;
  buildingsStatus: GameBuildingsStatus | null;
  buildingsError: string | null;
  buildingsPending: boolean;
  onBuildingAction: (action: "add" | "destroy" | "remove", techID: number, listID?: number) => void;
  onBuildingsRefresh: () => void;
  empireStatus: GameEmpireStatus | null;
  empireError: string | null;
  resourcesStatus: GameResourcesStatus | null;
  resourcesError: string | null;
  resourcesPending: boolean;
  onResourcesSubmit: (production: Record<string, string>) => void;
  merchantStatus: GameMerchantStatus | null;
  merchantError: string | null;
  merchantPending: boolean;
  onMerchantCall: (offerID: number) => void;
  onMerchantTrade: (values: GameMerchantTradeValues) => void;
  officersStatus: GameOfficersStatus | null;
  officersError: string | null;
  officersPending: boolean;
  onOfficerRecruit: (draft: GameOfficerRecruitment) => void;
  allianceStatus: GameAllianceStatus | null;
  allianceError: string | null;
  alliancePending: boolean;
  onAllianceAction: (action: GameAllianceAction) => void;
  adminStatus: GameAdminStatus | null;
  adminError: string | null;
  onAdminAction: (action: GameAdminAction) => void;
  researchStatus: GameResearchStatus | null;
  researchError: string | null;
  researchPending: boolean;
  onResearchAction: (action: "start" | "cancel", techID: number) => void;
  shipyardStatus: GameShipyardStatus | null;
  shipyardError: string | null;
  shipyardPending: boolean;
  onShipyardSubmit: (orders: Record<string, number>) => void;
  onShipyardRefresh: () => void;
  fleetStatus: GameFleetStatus | null;
  fleetError: string | null;
  fleetPending: boolean;
  onFleetPrepare: (draft: GameFleetDispatchPrepare) => void;
  onFleetLaunch: (draft: GameFleetDispatchLaunch) => void;
  onFleetRecall: (fleetID: number) => void;
  onFleetTemplateAction: (action: "save" | "delete", templateID: number, name: string, ships: Record<string, number>) => void;
  galaxyStatus: GameGalaxyStatus | null;
  galaxyError: string | null;
  galaxyPending: boolean;
  onGalaxyMissileLaunch: (draft: GameGalaxyMissileLaunch) => void;
  onGalaxyInstantDispatch: (draft: GameGalaxyInstantDispatch) => void;
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
  phalanxStatus: GamePhalanxStatus | null;
  phalanxError: string | null;
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
    oldPassword: string;
    newPassword: string;
    newPasswordRepeat: string;
    email: string;
    vacationMode: boolean;
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
  | { type: "external"; key: "board" | "discord"; label: string }
  | { type: "route"; color?: string; id?: string; key: GameRoute["key"] };

const skinBase = "/public-assets/evolution";
const gameImageBase = "/public-assets/game-img";
const GalaxyDeuteriumCostText = "10";
const gameRouteByKey = new Map(gameRoutes.map((route) => [route.key, route]));
const legacyMenuEntries: LegacyMenuEntry[] = [
  { type: "image", height: 40, src: `${skinBase}/gfx/ogame-produktion.jpg`, width: 110 },
  { type: "route", key: "overview" },
  { type: "route", key: "empire" },
  { type: "route", key: "admin" },
  { type: "route", key: "buildings" },
  { type: "route", key: "resources" },
  { type: "route", color: "#FF8900", key: "merchant" },
  { type: "route", key: "research" },
  { type: "route", key: "shipyard" },
  { type: "route", key: "fleet" },
  { type: "route", key: "technology" },
  { type: "route", key: "galaxy" },
  { type: "route", key: "defense" },
  { type: "image", height: 19, src: `${skinBase}/gfx/info-help.jpg`, width: 110 },
  { type: "route", key: "alliance" },
  { type: "external", key: "board", label: "Board" },
  { type: "external", key: "discord", label: "Discord" },
  { type: "route", id: "darkmatter2", key: "officers" },
  { type: "route", key: "statistics" },
  { type: "route", key: "search" },
  { type: "image", height: 35, src: `${skinBase}/gfx/user-menu.jpg`, width: 110 },
  { type: "route", key: "messages" },
  { type: "route", key: "notes" },
  { type: "route", key: "buddy" },
  { type: "route", key: "options" },
  { type: "route", key: "logout" }
];

function LegacyCenter({ children }: { children: React.ReactNode }) {
  return React.createElement("center", null, children);
}

function LegacyFont({ children, color }: { children: React.ReactNode; color?: string }) {
  return React.createElement("font", color ? ({ color } as React.HTMLAttributes<HTMLElement> & { color: string }) : null, children);
}

export function LegacyGameOverview({
  status,
  error,
  route,
  overviewPending,
  onOverviewRefresh,
  onPlanetDelete,
  onPlanetRename,
  buildingsStatus,
  buildingsError,
  buildingsPending,
  onBuildingAction,
  onBuildingsRefresh,
  empireStatus,
  empireError,
  resourcesStatus,
  resourcesError,
  resourcesPending,
  onResourcesSubmit,
  merchantStatus,
  merchantError,
  merchantPending,
  onMerchantCall,
  onMerchantTrade,
  officersStatus,
  officersError,
  officersPending,
  onOfficerRecruit,
  allianceStatus,
  allianceError,
  alliancePending,
  onAllianceAction,
  adminStatus,
  adminError,
  onAdminAction,
  researchStatus,
  researchError,
  researchPending,
  onResearchAction,
  shipyardStatus,
  shipyardError,
  shipyardPending,
  onShipyardSubmit,
  onShipyardRefresh,
  fleetStatus,
  fleetError,
  fleetPending,
  onFleetPrepare,
  onFleetLaunch,
  onFleetRecall,
  onFleetTemplateAction,
  galaxyStatus,
  galaxyError,
  galaxyPending,
  onGalaxyMissileLaunch,
  onGalaxyInstantDispatch,
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
  phalanxStatus,
  phalanxError,
  optionsStatus,
  optionsError,
  optionsPending,
  onOptionsSubmit,
  logoutStatus,
  logoutError
}: LegacyGameOverviewProps) {
  const overview = status?.authenticated ? status.overview : undefined;
  const issue = status && !status.authenticated ? status.issues[0]?.message ?? "Session is invalid." : null;
  React.useEffect(() => {
    const legacyWindow = window as Window & { showGalaxy?: (galaxy: number, system: number, planet: number) => void };
    legacyWindow.showGalaxy = (galaxy: number, system: number, planet: number) => {
      const search = overviewSessionSearch();
      search.set("galaxy", String(galaxy));
      search.set("system", String(system));
      search.set("position", String(planet));
      window.location.href = gameRouteURL("/game/galaxy", search.toString());
    };
    return () => {
      delete legacyWindow.showGalaxy;
    };
  }, []);
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
  const merchant = merchantStatus?.authenticated ? merchantStatus.merchant : undefined;
  const merchantIssue =
    merchantStatus && !merchantStatus.authenticated ? merchantStatus.issues[0]?.message ?? "Session is invalid." : null;
  const merchantActionIssue = merchantStatus?.authenticated ? merchantStatus.actionIssue : undefined;
  const officers = officersStatus?.authenticated ? officersStatus.officers : undefined;
  const officersIssue =
    officersStatus && !officersStatus.authenticated ? officersStatus.issues[0]?.message ?? "Session is invalid." : null;
  const officersActionIssue = officersStatus?.authenticated ? officersStatus.actionIssue : undefined;
  const officersActionTone = officersActionIssue?.code === "recruited" ? "neutral" : "error";
  const alliance = allianceStatus?.authenticated ? allianceStatus.alliance : undefined;
  const allianceIssue =
    allianceStatus && !allianceStatus.authenticated ? allianceStatus.issues[0]?.message ?? "Session is invalid." : null;
  const allianceActionIssue = allianceStatus?.authenticated ? allianceStatus.actionIssue : undefined;
  const allianceActionTone =
    allianceActionIssue &&
    ["created", "applied", "withdrawn", "accepted", "rejected", "left"].includes(allianceActionIssue.code)
      ? "neutral"
      : "error";
  const isAllianceInfoPopup = route.key === "alliance" && alliance?.view === "info";
  const admin = adminStatus?.authenticated ? adminStatus.admin : undefined;
  const adminIssue = adminStatus && !adminStatus.authenticated ? adminStatus.issues[0]?.message ?? "Session is invalid." : null;
  const adminActionIssue = adminStatus?.authenticated ? adminStatus.actionIssue : undefined;
  const adminAccessDenied = adminActionIssue?.code === "access_denied";
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
  const galaxyActionIssue = galaxyStatus?.authenticated ? galaxyStatus.actionIssue : undefined;
  const galaxyActionTone = galaxyActionIssue?.code === "rocket_launched" ? "neutral" : "error";
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
  const phalanx = phalanxStatus?.authenticated ? phalanxStatus.phalanx : undefined;
  const phalanxIssue =
    phalanxStatus && !phalanxStatus.authenticated ? phalanxStatus.issues[0]?.message ?? "Session is invalid." : null;
  const phalanxActionIssue = phalanx?.actionIssue;
  const options = optionsStatus?.authenticated ? optionsStatus.options : undefined;
  const optionsIssue =
    optionsStatus && !optionsStatus.authenticated ? optionsStatus.issues[0]?.message ?? "Session is invalid." : null;
  const messagesActionIssue = messagesStatus?.authenticated ? messagesStatus.actionIssue : undefined;
  const messagesActionTone =
    messagesActionIssue?.code === "sent" || messagesActionIssue?.code === "reported" ? "neutral" : "error";
  const optionsActionIssue = optionsStatus?.authenticated ? optionsStatus.actionIssue : undefined;
  const hasHeader =
    !isAllianceInfoPopup &&
    route.key !== "notes" &&
    route.key !== "galaxy" &&
    route.key !== "report" &&
    route.key !== "phalanx" &&
    route.key !== "admin" &&
    route.key !== "empire";
  const hasMenu = !isAllianceInfoPopup && route.key !== "notes" && route.key !== "report" && route.key !== "phalanx";
  const hasOverviewPageMessage =
    hasHeader && Boolean(overview && route.key === "overview" && overview.messages && overview.messages.length > 0);
  const hasOverviewPageError =
    hasHeader && Boolean(overview && route.key === "overview" && overview.errors && overview.errors.length > 0);
  const searchPageMessage =
    route.key === "search" && search?.message && !isSearchPageErrorMessage(search.message) ? search.message : "";
  const searchPageError =
    route.key === "search" && search?.message && isSearchPageErrorMessage(search.message) ? search.message : "";
  const hasSearchPageFooter = Boolean(searchPageMessage || searchPageError);
  const pageMessageRef = React.useRef<HTMLDivElement | null>(null);
  const pageErrorRef = React.useRef<HTMLDivElement | null>(null);
  const searchMessageRef = React.useRef<HTMLDivElement | null>(null);
  const searchErrorRef = React.useRef<HTMLDivElement | null>(null);
  const [overviewContentLayout, setOverviewContentLayout] = React.useState<{ height: string; top: number; errorTop: number } | null>(
    null
  );
  const [searchContentLayout, setSearchContentLayout] = React.useState<{ height: string; top: number; errorTop: number } | null>(
    null
  );
  React.useLayoutEffect(() => {
    if (route.key !== "overview") {
      setOverviewContentLayout(null);
      return;
    }
    const updateOverviewContentLayout = () => {
      const headerHeight = 81;
      const messageHeight = pageMessageRef.current?.offsetHeight ?? 0;
      const errorHeight = pageErrorRef.current?.offsetHeight ?? 0;
      const errorTop = headerHeight + messageHeight + 5;
      const top = headerHeight + errorHeight + messageHeight + 10;
      const height = `${Math.max(0, window.innerHeight - messageHeight - errorHeight - headerHeight - 20)}px`;
      setOverviewContentLayout((current) =>
        current?.top === top && current.height === height && current.errorTop === errorTop ? current : { height, top, errorTop }
      );
    };
    updateOverviewContentLayout();
    window.addEventListener("resize", updateOverviewContentLayout);
    return () => window.removeEventListener("resize", updateOverviewContentLayout);
  }, [hasOverviewPageError, hasOverviewPageMessage, route.key]);
  React.useLayoutEffect(() => {
    if (route.key !== "search" || !hasSearchPageFooter) {
      setSearchContentLayout(null);
      return;
    }
    const updateSearchContentLayout = () => {
      const headerHeight = 81;
      const messageHeight = searchMessageRef.current?.offsetHeight ?? 0;
      const errorHeight = searchErrorRef.current?.offsetHeight ?? 0;
      const top = headerHeight + errorHeight + messageHeight + 10;
      const height = `${Math.max(0, window.innerHeight - messageHeight - errorHeight - headerHeight - 20)}px`;
      const errorTop = headerHeight + messageHeight + 5;
      setSearchContentLayout((current) =>
        current?.top === top && current.height === height && current.errorTop === errorTop ? current : { height, top, errorTop }
      );
    };
    updateSearchContentLayout();
    window.addEventListener("resize", updateSearchContentLayout);
    return () => window.removeEventListener("resize", updateSearchContentLayout);
  }, [hasSearchPageFooter, route.key, searchPageError, searchPageMessage]);
  React.useEffect(() => {
    if (route.key !== "fleetTemplates" || !fleet || fleet.templates.commanderActive) {
      return;
    }
    const search = overviewSessionSearch();
    window.history.replaceState({}, "", gameRouteURL("/game/overview", search.toString()));
    window.dispatchEvent(new PopStateEvent("popstate"));
  }, [fleet, route.key]);
  const contentClassName =
    route.key === "overview"
      ? "legacy-content legacy-content-overview"
      : route.key === "galaxy" || route.key === "admin" || route.key === "empire"
        ? "legacy-content legacy-content-noheader"
      : route.key === "notes" || route.key === "report" || route.key === "phalanx" || isAllianceInfoPopup
        ? "legacy-content legacy-content-popup"
        : "legacy-content";
  const contentStyle: React.CSSProperties =
    route.key === "overview"
      ? overviewContentLayout
        ? { height: overviewContentLayout.height, top: `${overviewContentLayout.top}px` }
        : { height: "calc(100vh - 124px)" }
      : hasSearchPageFooter
        ? searchContentLayout
          ? { height: searchContentLayout.height, top: `${searchContentLayout.top}px` }
          : { height: "calc(100vh - 130px)", top: "120px" }
      : route.key === "galaxy" ||
          route.key === "admin" ||
          route.key === "empire" ||
          route.key === "notes" ||
          route.key === "report" ||
          route.key === "phalanx" ||
          isAllianceInfoPopup
        ? { height: "calc(100vh - 20px)" }
        : { height: "calc(100vh - 101px)" };

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
        <div className="legacy-header-top" id="header_top">
          <LegacyCenter>{overview ? <LegacyResourceHeader overview={overview} /> : <div className="legacy-header-placeholder">OGame</div>}</LegacyCenter>
        </div>
      ) : null}
      {hasMenu ? (
        <LegacyLeftMenu
          activeRoute={route}
          adminLevel={overview?.adminLevel ?? 0}
          commanderActive={overview?.officers?.commander ?? false}
          menuLinks={overview?.menuLinks}
        />
      ) : null}
      {hasOverviewPageMessage && overview?.messages ? (
        <LegacyPageMessage ref={pageMessageRef} messages={overview.messages} />
      ) : null}
      {hasOverviewPageError && overview?.errors ? (
        <LegacyPageError
          ref={pageErrorRef}
          messages={overview.errors}
          style={{ top: overviewContentLayout && hasOverviewPageMessage ? `${overviewContentLayout.errorTop}px` : "86px" }}
        />
      ) : null}
      {searchPageMessage ? <LegacyPageMessage ref={searchMessageRef} messages={[searchPageMessage]} /> : null}
      {searchPageError ? (
        <LegacyPageError
          ref={searchErrorRef}
          messages={[searchPageError]}
          style={{ top: searchContentLayout && searchPageMessage ? `${searchContentLayout.errorTop}px` : "86px" }}
        />
      ) : null}
      <section className={contentClassName} id="content" style={contentStyle}>
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
        {route.key === "merchant" && merchantError ? <LegacyMessage tone="error" text={merchantError} /> : null}
        {route.key === "merchant" && !merchantError && merchantActionIssue ? (
          <LegacyMessage tone="error" text={merchantActionIssue.message} />
        ) : null}
        {route.key === "merchant" && !merchantError && !merchantActionIssue && merchantIssue ? (
          <LegacyMessage tone="error" text={merchantIssue} />
        ) : null}
        {route.key === "officers" && officersError ? <LegacyMessage tone="error" text={officersError} /> : null}
        {route.key === "officers" && !officersError && officersActionIssue ? (
          <LegacyMessage tone={officersActionTone} text={officersActionIssue.message} />
        ) : null}
        {route.key === "officers" && !officersError && !officersActionIssue && officersIssue ? (
          <LegacyMessage tone="error" text={officersIssue} />
        ) : null}
        {route.key === "alliance" && allianceError ? <LegacyMessage tone="error" text={allianceError} /> : null}
        {route.key === "alliance" && !allianceError && allianceActionIssue ? (
          <LegacyMessage tone={allianceActionTone} text={allianceActionIssue.message} />
        ) : null}
        {route.key === "alliance" && !allianceError && !allianceActionIssue && allianceIssue ? (
          <LegacyMessage tone="error" text={allianceIssue} />
        ) : null}
        {route.key === "admin" && adminError ? <LegacyMessage tone="error" text={adminError} /> : null}
        {route.key === "admin" && !adminError && adminActionIssue ? (
          <LegacyMessage tone="error" text={adminActionIssue.message} />
        ) : null}
        {route.key === "admin" && !adminError && !adminActionIssue && adminIssue ? (
          <LegacyMessage tone="error" text={adminIssue} />
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
        {route.key === "galaxy" && !galaxyError && galaxyActionIssue ? (
          <LegacyMessage tone={galaxyActionTone} text={galaxyActionIssue.message} />
        ) : null}
        {route.key === "galaxy" && !galaxyError && !galaxyActionIssue && galaxyIssue ? (
          <LegacyMessage tone="error" text={galaxyIssue} />
        ) : null}
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
        {route.key === "phalanx" && phalanxError ? <LegacyMessage tone="error" text={phalanxError} /> : null}
        {route.key === "phalanx" && !phalanxError && phalanxIssue ? (
          <LegacyMessage tone="error" text={phalanxIssue} />
        ) : null}
        {route.key === "options" && optionsError ? <LegacyMessage tone="error" text={optionsError} /> : null}
        {route.key === "options" && !optionsError && optionsActionIssue ? (
          <LegacyMessage tone="neutral" text={optionsActionIssue.message} />
        ) : null}
        {route.key === "options" && !optionsError && !optionsActionIssue && optionsIssue ? (
          <LegacyMessage tone="error" text={optionsIssue} />
        ) : null}
        {report && route.key === "report" ? <ReportTable report={report} /> : null}
        {overview && route.key === "phalanx" && !phalanx && !phalanxError && !phalanxIssue ? (
          <LegacyMessage tone="neutral" text="Loading phalanx..." />
        ) : null}
        {phalanx && route.key === "phalanx" ? <PhalanxTable phalanx={phalanx} /> : null}
        {overview && route.key === "overview" ? <OverviewPage onBuildQueueComplete={onOverviewRefresh} overview={overview} /> : null}
        {overview && route.key === "renamePlanet" ? (
          <RenamePlanetTable onDelete={onPlanetDelete} onRename={onPlanetRename} overview={overview} pending={overviewPending} />
        ) : null}
        {overview && route.key === "buildings" && !buildings && !buildingsError && !buildingsIssue ? (
          <LegacyMessage tone="neutral" text="Loading buildings..." />
        ) : null}
        {buildings && route.key === "buildings" ? (
          <BuildingsTable buildings={buildings} onAction={onBuildingAction} onComplete={onBuildingsRefresh} pending={buildingsPending} />
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
        {overview && route.key === "merchant" && !merchant && !merchantError && !merchantIssue && !merchantActionIssue ? (
          <LegacyMessage tone="neutral" text="Loading merchant..." />
        ) : null}
        {merchant && route.key === "merchant" ? (
          <MerchantTable
            merchant={merchant}
            onCall={onMerchantCall}
            onTrade={onMerchantTrade}
            pending={merchantPending}
          />
        ) : null}
        {overview && route.key === "officers" && !officers && !officersError && !officersIssue && !officersActionIssue ? (
          <LegacyMessage tone="neutral" text="Loading officers..." />
        ) : null}
        {officers && route.key === "officers" ? (
          <OfficersTable officers={officers} onRecruit={onOfficerRecruit} pending={officersPending} />
        ) : null}
        {overview && route.key === "alliance" && !alliance && !allianceError && !allianceIssue && !allianceActionIssue ? (
          <LegacyMessage tone="neutral" text="Loading alliance..." />
        ) : null}
        {alliance && route.key === "alliance" ? (
          <AllianceTable alliance={alliance} onAction={onAllianceAction} pending={alliancePending} />
        ) : null}
        {overview && route.key === "admin" && !admin && !adminError && !adminIssue && !adminActionIssue ? (
          <LegacyMessage tone="neutral" text="Loading admin..." />
        ) : null}
        {admin && route.key === "admin" && !adminAccessDenied ? <AdminTable admin={admin} onAdminAction={onAdminAction} /> : null}
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
          <ShipyardTable onComplete={onShipyardRefresh} onSubmit={onShipyardSubmit} pending={shipyardPending} shipyard={shipyard} />
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
        {galaxy && route.key === "galaxy" ? (
          <GalaxyTable
            adminLevel={overview?.adminLevel ?? 0}
            galaxy={galaxy}
            onInstantDispatch={onGalaxyInstantDispatch}
            onMissileLaunch={onGalaxyMissileLaunch}
            pending={galaxyPending}
          />
        ) : null}
        {overview && route.key === "defense" && !defense && !defenseError && !defenseIssue ? (
          <LegacyMessage tone="neutral" text="Loading defense..." />
        ) : null}
        {defense && route.key === "defense" ? (
          <DefenseTable defense={defense} onSubmit={onDefenseSubmit} pending={defensePending} />
        ) : null}
        {overview && route.key === "technology" && !technology && !technologyError && !technologyIssue ? (
          <LegacyMessage tone="neutral" text="Loading technology..." />
        ) : null}
        {technology && route.key === "technology" ? <TechnologyTable onBuildingAction={onBuildingAction} technology={technology} /> : null}
        {overview && route.key === "changelog" ? <ChangelogTable /> : null}
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
            actionIssue={messagesActionIssue}
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
        route.key !== "merchant" &&
        route.key !== "officers" &&
        route.key !== "alliance" &&
        route.key !== "admin" &&
        route.key !== "research" &&
        route.key !== "shipyard" &&
        route.key !== "fleet" &&
        route.key !== "galaxy" &&
        route.key !== "defense" &&
        route.key !== "technology" &&
        route.key !== "changelog" &&
        route.key !== "statistics" &&
        route.key !== "search" &&
        route.key !== "buddy" &&
        route.key !== "messages" &&
        route.key !== "report" &&
        route.key !== "phalanx" &&
        route.key !== "notes" &&
        route.key !== "options" &&
        route.key !== "logout" ? (
          <MigrationPendingGameTable route={route} />
        ) : null}
      </section>
    </main>
  );
}

const LegacyPageMessage = React.forwardRef<HTMLDivElement, { messages: string[] }>(function LegacyPageMessage(
  { messages },
  ref
) {
  return (
    <div className="legacy-page-messagebox" id="messagebox" ref={ref} style={{ display: "block" }}>
      <center>
        {messages.map((message, index) => (
          <React.Fragment key={`${message}-${index}`}>
            {message}
            <br />
            {"\n"}
          </React.Fragment>
        ))}
      </center>
    </div>
  );
});

const LegacyPageError = React.forwardRef<HTMLDivElement, { messages: string[]; style?: React.CSSProperties }>(function LegacyPageError(
  { messages, style },
  ref
) {
  return (
    <div className="legacy-page-errorbox" id="errorbox" ref={ref} style={{ display: "block", ...style }}>
      <center>
        {messages.map((message, index) => (
          <React.Fragment key={`${message}-${index}`}>
            {message}
            <br />
            {"\n"}
          </React.Fragment>
        ))}
      </center>
    </div>
  );
});

function isSearchPageErrorMessage(message: string): boolean {
  return message.startsWith("Too few characters!");
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
    { name: "Dark Matter", value: planet.resources.darkMatter, color: "#FFFFFF", img: `${gameImageBase}/dm_klein_2.jpg` },
    {
      name: "Energy",
      value: planet.resources.energy,
      secondary: planet.resources.energyCapacity,
      color: planet.resources.energy < 0 ? "#ff0000" : undefined,
      signed: true,
      img: `${skinBase}/images/energie.gif`
    }
  ];
  const officers = [
    { active: overview.officers?.commander ?? false, alt: "Commander", image: "commander" },
    { active: overview.officers?.admiral ?? false, alt: "Admiral", image: "admiral" },
    { active: overview.officers?.engineer ?? false, alt: "Engineer", image: "ingenieur" },
    { active: overview.officers?.geologist ?? false, alt: "Geologist", image: "geologe" },
    { active: overview.officers?.technocrat ?? false, alt: "Technocrat", image: "technokrat" }
  ];
  const selectedPlanetHref = planetHref(planet.id);

  return (
    <table className="legacy-header-table header">
      <tbody>
        <tr className="header">
          <td className="legacy-header-cell header" style={{ width: 5 }}>
            <table className="legacy-header-table header">
              <tbody>
                <tr className="header">
                  <td className="legacy-header-cell header">
                    <img alt="" height={50} src={planetImagePath(planet, true)} width={50} />
                  </td>
                  <td className="legacy-header-cell header">
                    <select
                      aria-label="Planet selector"
                      size={1}
                      onChange={(event) => {
                        window.history.pushState({}, "", event.currentTarget.value);
                        window.dispatchEvent(new PopStateEvent("popstate"));
                      }}
                      value={selectedPlanetHref}
                    >
                      {overview.planetSwitcher.map((item) => (
                        <option key={item.id} value={planetHref(item.id)}>
                          {item.name}  [{formatCoordinates(item.coordinates)}]
                        </option>
                      ))}
                    </select>
                  </td>
                </tr>
              </tbody>
            </table>
          </td>
          <td className="legacy-header-cell header">
            <table cellPadding={0} cellSpacing={0} className="legacy-resource-table header" id="resources">
              <tbody>
                <tr className="header">
                  {resources.map((resource) => (
                    <td align="center" className="legacy-header-cell header" key={resource.name} width={85}>
                      {resource.name === "Dark Matter" ? (
                        <a href={gameMenuRouteURL("/game/merchant", window.location.search)}>
                          <img alt="" height={22} src={resource.img} title="Dark Matter" width={42} />
                        </a>
                      ) : (
                        <img alt="" height={22} src={resource.img} width={42} />
                      )}
                    </td>
                  ))}
                </tr>
                <tr className="header">
                  {resources.map((resource) => (
                    <td align="center" className="legacy-header-cell legacy-resource-name header" key={resource.name} width={85}>
                      <i>
                        <b>
                          <LegacyFont color="#ffffff">{resource.name}</LegacyFont>
                        </b>
                      </i>
                    </td>
                  ))}
                </tr>
                <tr className="header">
                  {resources.map((resource) => (
                    <td align="center" className="legacy-header-cell header" key={resource.name} width={90}>
                      <LegacyFont color={resource.color ?? (resource.capacity !== undefined && resource.value >= resource.capacity ? "#ff0000" : undefined)}>
                        {resource.signed ? formatLegacySignedNumber(resource.value) : formatLegacyNumber(resource.value)}
                      </LegacyFont>
                      {resource.secondary !== undefined ? `/${formatLegacyNumber(resource.secondary)}` : null}
                    </td>
                  ))}
                </tr>
              </tbody>
            </table>
          </td>
          <td className="legacy-header-cell header">
            <table align="left" className="legacy-officer-table header">
              <tbody>
                <tr className="header">
                  {officers.map((officer) => (
                    <td align="center" className="legacy-header-cell header" key={officer.image} width={35}>
                      <a accessKey="i" href={gameMenuRouteURL("/game/officers", window.location.search)}>
                        <img
                          alt={officer.alt}
                          height={32}
                          src={`${gameImageBase}/${officer.image}_ikon${officer.active ? "" : "_un"}.gif`}
                          width={32}
                        />
                      </a>
                    </td>
                  ))}
                  <td align="center" className="legacy-header-cell header" />
                </tr>
              </tbody>
            </table>
          </td>
        </tr>
      </tbody>
    </table>
  );
}

function LegacyLeftMenu({
  activeRoute,
  adminLevel,
  commanderActive,
  menuLinks
}: {
  activeRoute: GameRoute;
  adminLevel: number;
  commanderActive: boolean;
  menuLinks?: GameOverviewMenuLinks;
}) {
  return (
    <aside className="legacy-leftmenu" id="leftmenu">
      <div className="legacy-center">
        <div className="legacy-menu" id="menu">
          <p>
            <span className="legacy-nowrap">
              Universe 1 (<a href={gameRouteURL("/game/changelog", window.location.search)}>v 0.84</a>)
            </span>
          </p>
          <table cellPadding={0} cellSpacing={0} width={110}>
            <tbody>
              {legacyMenuEntries.map((entry, index) => {
                if (entry.type === "image") {
                  return (
                    <tr key={`${entry.src}-${index}`}>
                      <td>
                        <img alt="" height={entry.height} src={entry.src} width={entry.width} />
                      </td>
                    </tr>
                  );
                }
                if (entry.key === "admin" && adminLevel <= 0) {
                  return null;
                }
                if (entry.key === "empire" && !commanderActive) {
                  return null;
                }
                if (entry.type === "external") {
                  return <LegacyMenuExternal entry={entry} key={entry.key} menuLinks={menuLinks} />;
                }
                return <LegacyMenuRoute activeRoute={activeRoute} entry={entry} key={entry.key} />;
              })}
            </tbody>
          </table>
        </div>
      </div>
    </aside>
  );
}

function LegacyMenuExternal({ entry, menuLinks }: { entry: Extract<LegacyMenuEntry, { type: "external" }>; menuLinks?: GameOverviewMenuLinks }) {
  const url = entry.key === "board" ? menuLinks?.boardUrl : menuLinks?.discordUrl;
  if (!url) {
    return null;
  }
  return (
    <tr>
      <td>
        <div className="legacy-center">
          <a href={url} target="_blank">
            {entry.label}
          </a>
        </div>
      </td>
    </tr>
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
          <a
            aria-current={route.key === activeRoute.key ? "page" : undefined}
            href={gameMenuRouteURL(route.path, window.location.search)}
            id={entry.id}
            style={entry.id === "darkmatter2" ? { cursor: "pointer", width: 110 } : undefined}
          >
            {entry.color ? <span style={{ color: entry.color }}>{route.label}</span> : route.label}
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
          <td className="legacy-c c">{route.label}</td>
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
          <td className="legacy-c c">Logout</td>
        </tr>
        <tr>
          <th>{text}</th>
        </tr>
      </tbody>
    </table>
  );
}

function StatisticsTable({ statistics }: { statistics: GameStatistics }) {
  const navigateStatisticsForm = React.useCallback((form: HTMLFormElement) => {
    const query = new URLSearchParams();
    for (const [key, value] of new FormData(form).entries()) {
      query.set(key, String(value));
    }
    window.history.pushState({}, "", gameRouteURL("/game/statistics", query.toString()));
    window.dispatchEvent(new PopStateEvent("popstate"));
  }, []);
  const submitStatistics = React.useCallback(
    (event: React.FormEvent<HTMLElement>) => {
      const form = event.target as HTMLFormElement;
      if (!(form instanceof HTMLFormElement) || !form.classList.contains("legacy-statistics-form")) {
        return;
      }
      event.preventDefault();
      navigateStatisticsForm(form);
    },
    [navigateStatisticsForm]
  );
  const clickStatistics = React.useCallback(
    (event: React.MouseEvent<HTMLElement>) => {
      if (!(event.target instanceof Element)) {
        return;
      }
      const sortLink = event.target.closest<HTMLElement>("[data-statistics-sort]");
      if (!sortLink) {
        return;
      }
      event.preventDefault();
      const form = sortLink.closest("center")?.querySelector<HTMLFormElement>("form.legacy-statistics-form");
      const sortInput = form?.querySelector<HTMLInputElement>("#sort_per_member");
      const sort = sortLink.dataset.statisticsSort ?? "0";
      if (form && sortInput) {
        sortInput.value = sort;
        navigateStatisticsForm(form);
      }
    },
    [navigateStatisticsForm]
  );
  return React.createElement("center", {
    dangerouslySetInnerHTML: { __html: statisticsHTML(statistics) },
    onClick: clickStatistics,
    onSubmit: submitStatistics
  });
}

function statisticsHTML(statistics: GameStatistics): string {
  const windows = statisticsWindows(statistics.total, statistics.start);
  const action = legacyHTMLAttribute(gameRouteURL("/game/statistics", window.location.search));
  const who = statistics.who === "ally" ? "ally" : "player";
  const type = statistics.type || "ressources";
  const displayStart = new URLSearchParams(window.location.search).has("start") ? statistics.start : -1;
  if (displayStart !== statistics.start) {
    statistics = { ...statistics, start: displayStart };
  }
  let html = `<!-- begin header form --> \n<form class="legacy-statistics-form" method="post" action="${action}" > \n  \n  <!-- begin head table --> \n  <table class="legacy-statistics-head-table" width="525"> \n    <tr> \n      <td class="c">Statistics (as of: ${formatLegacyStatisticsDateTime(statistics.generatedAt)})</td> \n    </tr> \n    <tr> \n      <th> \n        \n \n        What kind of&nbsp;\n          \n        <select name="who"> \n          <option value="player" ${who === "player" ? "selected" : ""}>Player</option> \n          <option value="ally" ${who === "ally" ? "selected" : ""}>Alliance</option> \n        </select> \n          \n        &nbsp;by&nbsp;\n              \n        <select name="type"> \n          <option value="ressources" ${type === "ressources" ? "selected" : ""}>Points</option> \n          <option value="fleet" ${type === "fleet" ? "selected" : ""}>Fleets</option> \n          <option value="research" ${type === "research" ? "selected" : ""}>Research</option> \n        </select> \n          \n        &nbsp;in place        <select name="start"> \n          <option value="-1" ${statistics.start === -1 ? "selected" : ""}>[Own position]</option> \n`;
  html += windows
    .map((start) => `          <option value="${start}" ${statistics.start === start ? "selected" : ""}>${start}-${start + 99}</option> \n`)
    .join("");
  html += `        </select> \n          \n${statisticsHiddenInputsHTML()}        <input type="hidden" id="sort_per_member" name="sort_per_member" value="${legacyHTMLAttribute(statisticsSortValue())}" /> \n        <input type=submit value="Show"> \n      </th> \n    </tr> \n  </table> \n  <!-- end head table --> \n    \n</form> \n<!-- end header form --> \n\n<!-- begin statistic data --> \n`;
  html += who === "ally" ? allianceStatisticsHTML(statistics) : playerStatisticsHTML(statistics);
  html += "\n<!-- end statistic data --><br><br><br><br>";
  return html;
}

function playerStatisticsHTML(statistics: GameStatistics): string {
  let html = `<!-- begin user --> \n<table class="legacy-statistics-table legacy-statistics-player-table" width="525"> \n  <tr> \n    <td class="c" width="30">Place</td> \n    <td class="c">Player</td> \n    <td class="c">&nbsp;</td> \n    <td class="c">Alliance</td> \n    <td class="c">Points</td> \n  </tr>\n`;
  for (const row of statistics.rows) {
    const playerColor = row.own ? "lime" : row.sameAlliance ? "87CEEB" : "FFFFFF";
    const playerHref = row.own || !isNavigableCoordinates(row.coordinates) ? "#" : gameRouteURL("/game/galaxy", galaxyTargetSearch(row.coordinates));
    const message = row.own
      ? ""
      : `      <a href="${legacyHTMLAttribute(gameMessageComposeURL(row.player.id, window.location.search))}"> \n        <img src="${skinBase}/img/m.gif" border="0" alt="Write message" /> \n      </a> \n`;
    const alliance =
      row.alliance && row.sameAlliance
        ? ` \t  <a href="${legacyHTMLAttribute(gameRouteURL("/game/alliance", window.location.search))}">\n        ${legacyHTMLText(row.alliance.tag)}      </a>\n`
        : row.alliance
          ? `   \t  <a href="${legacyHTMLAttribute(allianceInfoURL(row.alliance.id))}" target="_ally">\n        ${legacyHTMLText(row.alliance.tag)}      </a>\n`
          : `      <a href="${legacyHTMLAttribute(gameRouteURL("/game/alliance", window.location.search))}"> \n              </a> \n`;
    html += `  <tr data-statistics-row="${row.place}"> \n    <!-- rank --> \n    <th> \n      ${row.place}&nbsp;&nbsp;\n\n      ${statisticsDeltaHTML(row)} \n    </th> \n\n    <!-- nick --> \n    <th> \n       <a href="${legacyHTMLAttribute(playerHref)}" style='color:${playerColor}' >      \n\n${legacyHTMLText(row.player.name)}</a> \n    </th> \n\n    <!--  message-icon --> \n    <th> \n${message}    &nbsp;\n    </th> \n\n    <!--  ally --> \n    <th> \n${alliance}    </th> \n\n    <!-- points --> \n    <th> \n      ${formatLegacyNumber(row.displayScore)}    </th> \n\n  </tr> \n`;
  }
  html += "</table>\n<!-- end user -->";
  return html;
}

function statisticsSortValue(): string {
  return new URLSearchParams(window.location.search).get("sort_per_member") ?? "0";
}

function statisticsHiddenInputsHTML(): string {
  const search = new URLSearchParams(window.location.search);
  return ["session", "cp"]
    .map((name) => {
      const value = search.get(name);
      return value ? `        <input type="hidden" name="${name}" value="${legacyHTMLAttribute(value)}" /> \n` : "";
    })
    .join("");
}

function allianceInfoURL(allianceID: number): string {
  return allianceURL({ allyid: String(allianceID) });
}

function allianceStatisticsHTML(statistics: GameStatistics): string {
  let html = `<!-- begin ally -->\n<table class="legacy-statistics-table legacy-statistics-alliance-table" width="519">\n  <tr>\n    <td class ="c" width="30">Place</td>\n    <td class ="c">Alliance</td>\n    <td class="c">&nbsp;</td>\n    <td class ="c">Num.</td>\n    <td class ="c"><a href="#" data-statistics-sort="0">Thousand points</a></td>\n    <td class ="c"><a href="#" data-statistics-sort="1">Per person</a></td>\n  </tr>\n`;
  for (const row of statistics.rows) {
    const tag = row.alliance?.tag ?? "";
    const allyHref = row.own ? "#" : allianceInfoURL(row.alliance?.id ?? 0);
    const applyHref = allianceURL({ a: "2", allyid: String(row.alliance?.id ?? 0) });
    const applicationIcon =
      statistics.viewerAllianceId === 0
        ? `      <a href="${legacyHTMLAttribute(applyHref)}">\n        <img src="${skinBase}/img/m.gif" border="0" alt="Write message" />\n      </a>\n`
        : "";
    html += `  <tr data-statistics-row="${row.place}">\n  \n    <!-- rank -->\n    <th>\n      ${row.place}&nbsp;&nbsp;\n\n      ${statisticsDeltaHTML(row)} \n    </th>\n    \n    <!--  name -->\n    <th>\n      <a href="${legacyHTMLAttribute(allyHref)}"${row.own ? " style='color:lime;'" : " target='_ally'"}>      \n \n      ${legacyHTMLText(tag)}    </a>\n    </th>\n    \n    <!-- bewerben -->\n    <th>\n${applicationIcon}      &nbsp;\n    </th>\n    \n    <!-- amount members -->\n    <th>\n      ${formatLegacyNumber(row.members)} </th>\n    \n    <!-- points -->\n    <th>\n      ${formatLegacyNumber(row.displayScore)}     \n      \n    </th>\n    \n    <!-- points per member -->\n    <th>\n      \n      ${formatLegacyNumber(row.perMember)}\n              \n    </th>\n    \n  </tr>\n  \n  <tr>\n`;
  }
  html += "</table>\n<!-- end ally -->";
  return html;
}

function statisticsDeltaHTML(row: GameStatisticsRow): string {
  const title = `From ${formatLegacyDateTime(row.scoreDate)}`;
  let color = "87CEEB";
  let marker = "*";
  let tooltip = "*";
  if (row.delta < 0) {
    color = "lime";
    marker = "+";
    tooltip = `+${Math.abs(row.delta)}`;
  } else if (row.delta > 0) {
    color = "red";
    marker = "-";
    tooltip = `-${Math.abs(row.delta)}`;
  }
  return `<span class="legacy-statistics-delta" tabindex="0"><font color="${color}">${marker}</font><span class="legacy-statistics-tooltip"><font color="${color}">${legacyHTMLText(
    tooltip
  )}</font><br/><font color="white">${legacyHTMLText(title)}</font></span></span>`;
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

function isNavigableCoordinates(coordinates: Coordinates): boolean {
  return coordinates.galaxy > 0 && coordinates.system > 0 && coordinates.position > 0;
}

function BuildingsTable({
  buildings,
  onAction,
  onComplete,
  pending
}: {
  buildings: GameBuildings;
  onAction: (action: "add" | "destroy" | "remove", techID: number, listID?: number) => void;
  onComplete: () => void;
  pending: boolean;
}) {
  const activeQueue = buildings.queue[0];
  return (
    <table className="legacy-overview-table legacy-buildings-table" width={530}>
      <tbody>
        {buildings.commanderActive
          ? buildings.queue.map((entry, index) => (
              <tr key={`queue-${entry.listId}`}>
                <td className="l" colSpan={2}>
                  {index + 1}.: {entry.name}
                  {entry.level > 0 ? ` , level ${entry.destroy ? entry.level + 1 : entry.level}` : ""}
                  {entry.destroy ? " demolish" : ""}
                </td>
                <td className="k">
                  {index === 0 ? (
                    <BuildingQueueCountdown entry={entry} onComplete={onComplete} onRemove={onAction} pending={pending} />
                  ) : (
                    <a
                      href={buildingActionURL("remove", entry.techId, entry.listId)}
                      onClick={(event) => {
                        event.preventDefault();
                        if (!pending) {
                          onAction("remove", entry.techId, entry.listId);
                        }
                      }}
                    >
                      <span style={{ color: "red" }}>Cancel</span>
                    </a>
                  )}
                </td>
              </tr>
            ))
          : null}
        {buildings.items.map((item) => {
          const actionCell = buildingActionCell(buildings, item, activeQueue);
          return (
            <tr data-building-row={item.id} key={item.id}>
              <td className="legacy-l l legacy-building-image" dangerouslySetInnerHTML={{ __html: buildingImageHTML(item) }} />
              <td
                className="legacy-l l legacy-building-description"
                dangerouslySetInnerHTML={{ __html: buildingDescriptionHTML(item) }}
              />
              {actionCell.countdown ? (
                <td className={`${actionCell.className} legacy-building-action`}>
                  <BuildingQueueCountdown
                    entry={actionCell.countdown}
                    onComplete={onComplete}
                    onRemove={onAction}
                    pending={pending}
                  />
                </td>
              ) : (
                <td
                  className={`${actionCell.className} legacy-building-action`}
                  dangerouslySetInnerHTML={{ __html: actionCell.html }}
                  onClick={(event) => {
                    if (!actionCell.clickable || pending || !(event.target instanceof HTMLElement) || !event.target.closest("a")) {
                      return;
                    }
                    event.preventDefault();
                    onAction("add", item.id);
                  }}
                />
              )}
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function buildingActionCell(
  buildings: GameBuildings,
  item: GameBuildingItem,
  activeQueue: GameBuildingQueueEntry | undefined
): { className: string; html: string; clickable: boolean; countdown?: GameBuildingQueueEntry } {
  if (buildings.queue.length > 0) {
    if (buildings.commanderActive) {
      if (buildings.queue.length >= 5) {
        return { className: "k", html: "", clickable: false };
      }
      return { className: "k", html: buildingEnqueueHTML(item), clickable: true };
    }
    if (activeQueue?.techId === item.id) {
      return { className: "k", html: "", clickable: false, countdown: activeQueue };
    }
    return { className: "k", html: "", clickable: false };
  }
  return { className: "l", html: buildingActionHTML(item), clickable: item.canBuild };
}

function BuildingQueueCountdown({
  entry,
  onComplete,
  onRemove,
  pending
}: {
  entry: GameBuildingQueueEntry;
  onComplete: () => void;
  onRemove: (action: "add" | "destroy" | "remove", techID: number, listID?: number) => void;
  pending: boolean;
}) {
  const [now, setNow] = React.useState(() => Math.floor(Date.now() / 1000));
  const [refreshQueued, setRefreshQueued] = React.useState(false);
  React.useEffect(() => {
    const id = window.setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => window.clearInterval(id);
  }, []);
  React.useEffect(() => {
    setRefreshQueued(false);
  }, [entry.end, entry.listId, entry.techId]);
  const remaining = Math.max(0, entry.end - now);
  const completeBuildQueue = React.useCallback(() => {
    setRefreshQueued(true);
    onComplete();
  }, [onComplete]);
  React.useEffect(() => {
    if (remaining > 0 || refreshQueued) {
      return undefined;
    }
    const id = window.setTimeout(() => {
      completeBuildQueue();
    }, 2000);
    return () => window.clearTimeout(id);
  }, [completeBuildQueue, refreshQueued, remaining]);
  if (remaining <= 0) {
    return (
      <div className="z" id="bxx" title="0">
        Done
        <br />
        <a
          href={buildingNextURL()}
          onClick={(event) => {
            event.preventDefault();
            completeBuildQueue();
          }}
        >
          Next
        </a>
      </div>
    );
  }
  return (
    <div className="z" id="bxx" title={String(remaining)}>
      {formatLegacyCountdown(remaining)}
      <br />
      <a
        href={buildingActionURL("remove", entry.techId, entry.listId)}
        onClick={(event) => {
          event.preventDefault();
          if (!pending) {
            onRemove("remove", entry.techId, entry.listId);
          }
        }}
      >
        Cancel
      </a>
    </div>
  );
}

function OverviewBuildQueue({
  queue,
  includeLevel,
  onComplete
}: {
  queue: GameOverviewBuildQueue | undefined;
  includeLevel: boolean;
  onComplete: () => void;
}) {
  if (!queue) {
    return <>free</>;
  }
  return (
    <>
      {overviewBuildQueueText(queue, includeLevel)}
      {includeLevel ? <OverviewBuildQueueCountdown onComplete={onComplete} queue={queue} /> : null}
    </>
  );
}

function OverviewBuildQueueCountdown({ queue, onComplete }: { queue: GameOverviewBuildQueue; onComplete: () => void }) {
  const [now, setNow] = React.useState(() => Math.floor(Date.now() / 1000));
  const [refreshQueued, setRefreshQueued] = React.useState(false);
  React.useEffect(() => {
    const id = window.setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => window.clearInterval(id);
  }, []);
  React.useEffect(() => {
    setRefreshQueued(false);
  }, [queue.end, queue.techId]);
  const remaining = Math.max(0, queue.end - now);
  React.useEffect(() => {
    if (remaining > 0 || refreshQueued) {
      return undefined;
    }
    const id = window.setTimeout(() => {
      setRefreshQueued(true);
      onComplete();
    }, 1500);
    return () => window.clearTimeout(id);
  }, [onComplete, refreshQueued, remaining]);
  return (
    <div className="z" id="bxx" title={String(queue.end)}>
      {remaining <= 0 ? "--" : formatLegacyCountdown(remaining)}
    </div>
  );
}

function buildingNextURL() {
  const query = new URLSearchParams(window.location.search);
  query.delete("modus");
  query.delete("techid");
  query.delete("listid");
  return gameRouteURL("/game/buildings", `?${query.toString()}`);
}

function buildingImageHTML(item: GameBuildingItem): string {
  const href = legacyHTMLAttribute(technologyInfoURL(item.id));
  return `<a href="${href}"><img border="0" src="${skinBase}/gebaeude/${item.id}.gif" align="top" width="120" height="120"></a>`;
}

function buildingDescriptionHTML(item: GameBuildingItem): string {
  const href = legacyHTMLAttribute(technologyInfoURL(item.id));
  const level = item.level > 0 ? ` (level ${item.level})` : "";
  const costs = costParts(item.cost)
    .map((part) => ` ${legacyHTMLText(part.name)}: <b>${formatLegacyPlainNumber(part.value)}</b>`)
    .join("");
  return `<a href="${href}">${legacyHTMLText(item.name)}</a>${level}<br>${legacyHTMLText(item.description)}<br>Cost:${costs}<br>Duration: ${formatLegacyDuration(item.durationSeconds)}<br>`;
}

function buildingActionHTML(item: GameBuildingItem): string {
  const action = item.action === "Build level" ? `Build <br> level ${item.nextLevel}` : legacyHTMLText(item.action);
  const color = item.canBuild ? "#00FF00" : "#FF0000";
  const content = `<font color="${color}">${action}</font>`;
  if (!item.canBuild) {
    return content;
  }
  const href = legacyHTMLAttribute(buildingActionURL("add", item.id));
  return `<a href="${href}">${content}</a> `;
}

function buildingEnqueueHTML(item: GameBuildingItem): string {
  const href = legacyHTMLAttribute(buildingActionURL("add", item.id));
  return `<a href="${href}">In the queue for construction</a>`;
}

function legacyHTMLText(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function legacyHTMLAttribute(value: string): string {
  return legacyHTMLText(value);
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

function OfficersTable({
  officers,
  onRecruit,
  pending
}: {
  officers: GameOfficers;
  onRecruit: (draft: GameOfficerRecruitment) => void;
  pending: boolean;
}) {
  return (
    <LegacyCenter>
      <div
        id="header"
        style={{
          backgroundImage: `url('${gameImageBase}/kasino_600x120.jpg')`,
          height: 120,
          width: 600
        }}
      >
        <div
          id="headtext1"
          style={{ color: "f3d2b1", fontSize: 18, fontWeight: "bold", left: -160, position: "relative", top: 25 }}
        >
          To the wise lord ...
        </div>
        <div
          id="headtext2"
          style={{ color: "#c2f1fd", float: "right", fontSize: 13, fontWeight: "bold", left: -240, position: "relative", top: 23 }}
        >
          ... need smarts{" "}
          <b>
            {React.createElement("font", { size: 4 }, "advisors.")}
          </b>
        </div>
      </div>
      <table className="legacy-officers-table" width={600}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={3}>
              Dark Matter
            </td>
          </tr>
          <tr>
            <td className="legacy-l l">
              <img height={120} src={`${gameImageBase}/DMaterie.jpg`} style={{ border: 0, verticalAlign: "top" }} width={120} />
            </td>
            <td className="legacy-l l">
              <strong>Dark Matter</strong>
              <br />
              Dark matter is a substance that can only be stored for a few standard years at great expense. It can be used to
              extract incredible amounts of energy. The method of its extraction is very difficult and dangerous, so it is very
              highly valued.
              <div style={{ margin: "4px 4px" }}>
                <table>
                  <tbody>
                    <tr>
                      <td>
                        <img height={32} src={`${gameImageBase}/dm_klein_1.jpg`} style={{ verticalAlign: "middle" }} width={32} />
                      </td>
                      <td style={{ backgroundColor: "transparent" }}>
                        <strong style={{ color: "skyblue", verticalAlign: "middle" }}>
                          With this substance, officers and commanders can be hired.
                        </strong>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </td>
            <td className="legacy-l l" style={{ textAlign: "center", verticalAlign: "middle", width: 90 }}>
              <a
                href={gameRouteURL("/game/officers", window.location.search)}
                id="darkmatter2"
                style={{ cursor: "pointer", height: 60, textAlign: "center", width: 100 }}
              >
                <br />
                <b>
                  <div id="darkmatter2">Get the dark matter</div>
                </b>
              </a>
            </td>
          </tr>
          <tr>
            <td className="legacy-c c" colSpan={3}>
              Officers
            </td>
          </tr>
          {officers.rows.map((officer) => (
            <React.Fragment key={officer.id}>
              <tr data-officer-row={officer.id}>
                <td className="legacy-l l" rowSpan={2}>
                  <img height={120} src={`${gameImageBase}/${officer.image}`} style={{ border: 0, verticalAlign: "top" }} width={120} />
                </td>
                <td className="legacy-l l" rowSpan={2}>
                  {officer.key === "admiral" ? (
                    <>
                      <b>{officer.name}</b>(
                      <b>
                        {" "}
                        {officerStatus(officer)})
                        <br />
                        {officer.description}
                        <br />
                        <OfficerNoteTable officer={officer} />
                      </b>
                    </>
                  ) : (
                    <>
                      <b>{officer.name}</b>(<b> {officerStatus(officer)}</b>)<br />
                      {officer.description}
                      <br />
                      <OfficerNoteTable officer={officer} />
                    </>
                  )}
                </td>
                <td className="legacy-l l" style={{ textAlign: "center", verticalAlign: "middle", width: 90 }}>
                  <OfficerRecruitLink days={90} officer={officer} onRecruit={onRecruit} pending={pending} />
                </td>
              </tr>
              <tr>
                <td className="legacy-l l" style={{ textAlign: "center", verticalAlign: "middle", width: 90 }}>
                  <OfficerRecruitLink days={7} officer={officer} onRecruit={onRecruit} pending={pending} />
                </td>
              </tr>
              {officer.id !== officers.rows[officers.rows.length - 1]?.id ? (
                <tr>
                  <td className="legacy-c c" colSpan={3} style={{ height: 4 }} />
                </tr>
              ) : null}
            </React.Fragment>
          ))}
        </tbody>
      </table>
      <br />
      <br />
      <br />
      <br />
      <br />
    </LegacyCenter>
  );
}

function OfficerNoteTable({ officer }: { officer: GameOfficerRow }) {
  return (
    <div style={{ margin: "4px 4px" }}>
      <table>
        <tbody>
          <tr>
            <td>
              <img alt={officer.name} height={32} src={`${gameImageBase}/${officer.icon}`} style={{ verticalAlign: "middle" }} width={32} />
            </td>
            <td style={{ backgroundColor: "transparent" }}>
              <strong style={{ color: "skyblue", verticalAlign: "middle" }}>{officer.note}</strong>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}

function OfficerRecruitLink({
  days,
  officer,
  onRecruit,
  pending
}: {
  days: 7 | 90;
  officer: GameOfficerRow;
  onRecruit: (draft: GameOfficerRecruitment) => void;
  pending: boolean;
}) {
  const price = days === 90 ? officer.threeMonthCost : officer.weekCost;
  return (
    <a
      href={officerRecruitHref(officer.id, days)}
      onClick={(event) => {
        event.preventDefault();
        if (!pending) {
          onRecruit({ officerId: officer.id, days });
        }
      }}
    >
      <b>
        {days === 90 ? "3 months/months for" : "1 week for"}
        <br />
        {days === 90 ? (
          <LegacyFont color="lime">total {formatLegacyNumber(price)}</LegacyFont>
        ) : (
          <LegacyFont color="lime">{formatLegacyNumber(price)}</LegacyFont>
        )}
        <br />
        Dark Matter
      </b>
    </a>
  );
}

function officerStatus(officer: GameOfficerRow) {
  if (!officer.active) {
    return <LegacyFont color="red">Inactive</LegacyFont>;
  }
  return (
    <strong>
      <LegacyFont color="lime">Active</LegacyFont> more {officer.daysLeft} days
    </strong>
  );
}

function officerRecruitHref(officerID: number, days: number) {
  const query = new URLSearchParams(window.location.search);
  query.set("buynow", "1");
  query.set("type", String(officerID));
  query.set("days", String(days));
  return gameRouteURL("/game/officers", `?${query.toString()}`);
}

function AdminTable({ admin, onAdminAction }: { admin: GameAdmin; onAdminAction: (action: GameAdminAction) => void }) {
  if (admin.mode === "Bans") {
    return (
      <AdminModeShell admin={admin}>
        <AdminBansTable admin={admin} onAdminAction={onAdminAction} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Broadcast") {
    return (
      <AdminModeShell admin={admin}>
        <AdminBroadcastTable onAdminAction={onAdminAction} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Reports") {
    return (
      <AdminModeShell admin={admin}>
        <AdminReportsTable onAdminAction={onAdminAction} rows={admin.reportRows ?? []} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Bots") {
    return (
      <AdminModeShell admin={admin}>
        <AdminBotsTable admin={admin} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Coupons") {
    return (
      <AdminModeShell admin={admin}>
        <AdminCouponsTable admin={admin} onAdminAction={onAdminAction} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "ColonySettings") {
    return (
      <AdminModeShell admin={admin}>
        <AdminColonySettingsTable />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Debug") {
    return (
      <AdminModeShell admin={admin}>
        <AdminMessagesTable className="legacy-admin-debug-table" mode="Debug" rows={admin.messageRows ?? []} withFilter />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Errors") {
    return (
      <AdminModeShell admin={admin}>
        <AdminMessagesTable className="legacy-admin-errors-table" mode="Errors" rows={admin.messageRows ?? []} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Logins") {
    return (
      <AdminModeShell admin={admin}>
        <AdminLoginsTable />
      </AdminModeShell>
    );
  }
  if (admin.mode === "UserLogs") {
    return (
      <AdminModeShell admin={admin}>
        <AdminUserLogsTable rows={admin.userLogRows ?? []} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Browse") {
    return (
      <AdminModeShell admin={admin}>
        <AdminBrowseTable />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Fleetlogs") {
    return (
      <AdminModeShell admin={admin}>
        <AdminFleetlogsTable rows={admin.fleetLogRows ?? []} onAdminAction={onAdminAction} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Queue") {
    return (
      <AdminModeShell admin={admin}>
        <AdminQueueTable rows={admin.queueRows ?? []} onAdminAction={onAdminAction} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Users") {
    return (
      <AdminModeShell admin={admin}>
        <AdminUsersTable admin={admin} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Planets") {
    return (
      <AdminModeShell admin={admin}>
        <AdminPlanetsTable admin={admin} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Uni") {
    return (
      <AdminModeShell admin={admin}>
        <AdminUniverseTable admin={admin} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Checksum") {
    return (
      <AdminModeShell admin={admin}>
        <AdminChecksumTable groups={admin.checksumGroups ?? []} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "DB") {
    return (
      <AdminModeShell admin={admin}>
        <AdminDatabaseTable backups={admin.databaseBackups ?? []} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "BattleSim") {
    return (
      <AdminModeShell admin={admin}>
        <AdminBattleSimTable />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Expedition") {
    return (
      <AdminModeShell admin={admin}>
        <AdminExpeditionTable admin={admin} onAdminAction={onAdminAction} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "BattleReport") {
    return (
      <AdminModeShell admin={admin}>
        <AdminBattleReportsTable rows={admin.battleReports ?? []} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "BotEdit") {
    return (
      <AdminModeShell admin={admin}>
        <AdminBotEditTable admin={admin} />
      </AdminModeShell>
    );
  }
  if (admin.mode === "RakSim") {
    return (
      <AdminModeShell admin={admin}>
        <AdminRakSimTable />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Loca") {
    return (
      <AdminModeShell admin={admin}>
        <AdminLocaTable />
      </AdminModeShell>
    );
  }
  if (admin.mode === "Mods") {
    return (
      <AdminModeShell admin={admin}>
        <AdminModsTable />
      </AdminModeShell>
    );
  }
  if (admin.mode !== "Home") {
    return (
      <AdminModeShell admin={admin}>
        <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table" width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c">Admin Area</td>
            </tr>
            <tr>
              <th>
                {admin.mode} migration is pending.
                <br />
                <a href={adminHomeHref()}>Back</a>
              </th>
            </tr>
          </tbody>
        </table>
      </AdminModeShell>
    );
  }
  const rows: GameAdminMenuItem[][] = [];
  for (let index = 0; index < admin.menu.length; index += 5) {
    rows.push(admin.menu.slice(index, index + 5));
  }
  return (
    <>
      <br />
      <br />
      <table border={0} cellPadding={0} cellSpacing={1} className="s legacy-admin-home-table" style={{ verticalAlign: "top" }} width="100%">
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr key={`admin-menu-row-${rowIndex}`}>
              {row.map((item) => (
                <th key={item.mode}>
                  <a href={adminModeHref(item.mode)}>
                    <img alt="" src={item.image} />
                    <br />
                    {item.label}
                  </a>
                </th>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

function AdminModeShell({ admin, children }: { admin: GameAdmin; children: React.ReactNode }) {
  return (
    <LegacyCenter>
      <table border={0} cellPadding={0} cellSpacing={1} className="legacy-admin-mode-shell" width={750}>
        <tbody />
      </table>
      <AdminQuickPanel admin={admin} />
      {children}
      <br />
      <br />
      <br />
      <br />
    </LegacyCenter>
  );
}

function AdminQuickPanel({ admin }: { admin: GameAdmin }) {
  return (
    <>
      <table className="legacy-admin-quick-panel">
        <tbody>
          <tr>
            <td>
              {admin.menu.map((item) => (
                <React.Fragment key={item.mode}>
                  <a href={adminModeHref(item.mode)}>
                    <img alt={item.label} height={32} src={item.image} title={item.label} width={32} />
                  </a>
                  {"\n\n"}
                </React.Fragment>
              ))}
            </td>
          </tr>
        </tbody>
      </table>
      <br />
    </>
  );
}

function AdminBansTable({ admin, onAdminAction }: { admin: GameAdmin; onAdminAction: (action: GameAdminAction) => void }) {
  const [searched, setSearched] = React.useState(false);
  const users = searched ? uniqueAdminUsers([...(admin.userRows ?? []), ...(admin.activeUsers ?? [])]) : [];
  return (
    <>
      <form
        action={adminModeActionHref("Bans", "search")}
        method="POST"
        onSubmit={(event) => {
          event.preventDefault();
          setSearched(true);
        }}
      >
        <table className="legacy-admin-bans-table">
          <tbody>
            <tr>
              <td className="c" colSpan={2}>
                Find users
              </td>
            </tr>
            <tr>
              <td>
                <select name="searchby">
                  <option value="0">Banned with VM</option>
                  <option value="1">Banned without VM</option>
                  <option value="2">Attack bans</option>
                  <option value="3">Recently registered (days)</option>
                  <option value="4">User name (rough)</option>
                  <option value="5">Alliance Tag</option>
                  <option value="6">Same email address</option>
                  <option value="7">Same IP</option>
                </select>
              </td>
              <td>
                <input name="text" size={20} type="text" />
              </td>
            </tr>
            <tr>
              <td className="c" colSpan={2}>
                <input type="submit" value="Submit" />
              </td>
            </tr>
          </tbody>
        </table>
      </form>
      {searched ? (
        <form
          action={adminModeActionHref("Bans", "ban")}
          id="banform"
          method="POST"
          onSubmit={(event) => {
            event.preventDefault();
            const data = new FormData(event.currentTarget);
            const targetIds = data
              .getAll("id")
              .map((value) => Number(value))
              .filter((value) => Number.isFinite(value) && value > 0);
            onAdminAction({
              action: "ban",
              targetIds,
              banMode: Number(data.get("banmode")) || 0,
              days: Math.max(0, Number(data.get("days")) || 0),
              hours: Math.max(0, Number(data.get("hours")) || 0),
              reason: String(data.get("reason") ?? "")
            });
          }}
        >
          <table className="legacy-admin-bans-table">
            <tbody>
              <tr>
                <td className="c">ID</td>
                <td className="c">Name</td>
                <td className="c">Home Planet</td>
                <td className="c">Status</td>
              </tr>
              {users.length === 0 ? (
                <tr>
                  <td colSpan={4}>Not found</td>
                </tr>
              ) : (
                users.map((user) => (
                  <tr key={user.playerId}>
                    <th>
                      <input className="ids" name="id" type="checkbox" value={user.playerId} />
                      {user.playerId}
                    </th>
                    <th dangerouslySetInnerHTML={{ __html: adminUserNameHTML(user) }} />
                    <th>{user.homePlanet ? `${formatCoordinates(user.homePlanet.coordinates)} ${user.homePlanet.name}` : ""}</th>
                    <th>{adminUserStatus(user)}</th>
                  </tr>
                ))
              )}
              <tr>
                <td className="c" colSpan={4}>
                  Actions
                </td>
              </tr>
              <tr>
                <td colSpan={3}>
                  <label><input defaultChecked name="banmode" type="radio" value="1" /> <LegacyFont color="red"><b>Ban with vacation mode</b></LegacyFont></label>{" "}
                  <label><input name="banmode" type="radio" value="0" /> <LegacyFont color="firebrick"><b>Ban without vacation mode</b></LegacyFont></label>{" "}
                  <label><input name="banmode" type="radio" value="2" /> <LegacyFont color="yellow"><b>Attack ban</b></LegacyFont></label>{" "}
                  <label><input name="banmode" type="radio" value="3" /> <LegacyFont color="lime"><b>Unban</b></LegacyFont></label>{" "}
                  <label><input name="banmode" type="radio" value="4" /> <LegacyFont color="lime"><b>Allow attacks</b></LegacyFont></label>
                </td>
                <td>
                  <input name="days" size={5} type="text" /> days <input name="hours" size={3} type="text" /> hours
                </td>
              </tr>
              <tr>
                <th colSpan={3}>
                  Reason <textarea cols={40} name="reason" rows={4} />
                </th>
                <th>
                  <input type="submit" value="Submit" />
                </th>
              </tr>
            </tbody>
          </table>
        </form>
      ) : null}
    </>
  );
}

function uniqueAdminUsers(users: GameAdminUserRow[]): GameAdminUserRow[] {
  const seen = new Set<number>();
  const result: GameAdminUserRow[] = [];
  for (const user of users) {
    if (seen.has(user.playerId)) {
      continue;
    }
    seen.add(user.playerId);
    result.push(user);
  }
  return result;
}

function AdminBroadcastTable({ onAdminAction }: { onAdminAction: (action: GameAdminAction) => void }) {
  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    onAdminAction({
      action: "broadcast_send",
      category: Number(data.get("cat") ?? 0),
      subject: String(data.get("subj") ?? ""),
      text: String(data.get("text") ?? "")
    });
  };
  return (
    <form action={adminModeHref("Broadcast")} method="POST" onSubmit={handleSubmit}>
      <table className="legacy-admin-broadcast-table">
        <tbody>
          <tr>
            <td>
              To:{" "}
              <select name="cat">
                <option value="0">All</option>
                <option value="1">Beginners (less than 5.000 points)</option>
                <option value="2">Players in the top 100</option>
                <option value="3">Operators</option>
              </select>
            </td>
          </tr>
          <tr>
            <td>
              Subject: <input name="subj" size={80} />
            </td>
          </tr>
          <tr>
            <td>
              <textarea cols={100} name="text" rows={20} />
            </td>
          </tr>
          <tr>
            <td>
              <center>
                <input type="submit" value="Send" />
              </center>
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function AdminReportsTable({ onAdminAction, rows }: { onAdminAction: (action: GameAdminAction) => void; rows: GameAdminReportRow[] }) {
  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const deleteMode = String(data.get("deletemessages") ?? "deletemarked");
    const reportIds = rows.filter((row) => data.get(`delmes${row.id}`) === "on").map((row) => row.id);
    onAdminAction({ action: "reports_delete", reportIds, deleteMode });
  };
  return (
    <table className="header legacy-admin-reports-outer">
      <tbody>
        <tr className="header">
          <td>
            <form action={adminModeHref("Reports")} method="POST" onSubmit={handleSubmit}>
              <table className="legacy-admin-reports-table" width={519}>
                <tbody>
                  <tr>
                    <td className="c" colSpan={5}>
                      Messages
                    </td>
                  </tr>
                  <tr>
                    <th>Action</th>
                    <th>Date</th>
                    <th>From</th>
                    <th>Recipient</th>
                    <th>Subject</th>
                  </tr>
                  {rows.map((row) => (
                    <React.Fragment key={row.id}>
                      <tr>
                        <th>
                          <input name={`delmes${row.id}`} type="checkbox" />
                        </th>
                        <th>{formatLegacyAdminMessageDate(row.date)}</th>
                        <th dangerouslySetInnerHTML={{ __html: `${sanitizeLegacyMessageHTML(legacyAdminHTMLWithSession(row.from))} ` }} />
                        <th>
                          <AdminUserLink ownerId={row.ownerId} ownerName={row.ownerName} />{" "}
                        </th>
                        <th dangerouslySetInnerHTML={{ __html: `${sanitizeLegacyMessageHTML(legacyAdminHTMLWithSession(row.subject))} ` }} />
                      </tr>
                      <tr>
                        <td className="b"> </td>
                        <td className="b" colSpan={4} dangerouslySetInnerHTML={{ __html: sanitizeLegacyMessageHTML(legacyAdminHTMLWithSession(row.text)) }} />
                      </tr>
                    </React.Fragment>
                  ))}
                  <tr>
                    <td className="b"> </td>
                    <td className="b" colSpan={4} />
                  </tr>
                  <tr>
                    <th colSpan={5} style={{ padding: "0px 105px" }} />
                  </tr>
                  <tr>
                    <th colSpan={5}>
                      <select name="deletemessages">
                        <option value="deletemarked">Delete highlighted messages</option>
                        <option value="deleteall">Delete all messages</option>
                      </select>
                      <input type="submit" value="ok" />
                    </th>
                  </tr>
                  <tr>
                    <td colSpan={5}>
                      <center>     </center>
                    </td>
                  </tr>
                </tbody>
              </table>
            </form>
          </td>
        </tr>
      </tbody>
    </table>
  );
}

function AdminBotsTable({ admin }: { admin: GameAdmin }) {
  if (admin.viewer.level < 2) {
    return <LegacyFont color="red">Access denied.</LegacyFont>;
  }
  return (
    <>
      <center />
      <h2>Bot List:</h2>
      No bots found
      <br />
      <h2>Add bot:</h2>
      <form action={adminModeHref("Bots")} method="POST" onSubmit={(event) => event.preventDefault()}>
        <table className="legacy-admin-bots-table">
          <tbody>
            <tr>
              <td>
                Name <input name="name" size={10} type="text" /> <input type="submit" value="Submit" />
              </td>
            </tr>
          </tbody>
        </table>
      </form>
    </>
  );
}

function AdminCouponsTable({ admin, onAdminAction }: { admin: GameAdmin; onAdminAction: (action: GameAdminAction) => void }) {
  const handleAddOne = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    onAdminAction({ action: "add_one", amount: Math.max(0, Number(data.get("dm")) || 0) });
  };
  const handleAddDate = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    onAdminAction({
      action: "add_date",
      amount: Math.max(0, Number(data.get("darkmatter")) || 0),
      dayMonth: String(data.get("ddmm") ?? ""),
      hourMinute: String(data.get("hhmm") ?? ""),
      inactiveDays: Math.max(0, Number(data.get("inactive_days")) || 0),
      ingameDays: Math.max(0, Number(data.get("ingame_days")) || 0),
      periodicDays: Math.max(0, Number(data.get("periodic")) || 0)
    });
  };
  return (
    <>
      <table border={0} cellPadding={2} cellSpacing={1} className="legacy-admin-coupons-table">
        <tbody>
          <tr style={{ height: 20 }}>
            <td className="c">Code</td>
            <td className="c">Dark Matter</td>
            <td className="c">Activated</td>
            <td className="c">Universe</td>
            <td className="c">Player</td>
            <td className="c">Action</td>
          </tr>
          {(admin.couponRows ?? []).map((row) => (
            <tr key={row.id} style={{ height: 20 }}>
              <th>{row.code}</th>
              <th>{formatLegacyNumber(row.amount)}</th>
              <th>
                <LegacyFont color={row.used ? "red" : "lime"}>{row.used ? "Yes" : "No"}</LegacyFont>
              </th>
              <th>{row.used ? row.userUniverse : "-"}</th>
              <th>{row.userName}</th>
              <th>
                <a
                  href={`${adminModeActionHref("Coupons", "remove_one")}&item_id=${row.id}`}
                  onClick={(event) => {
                    event.preventDefault();
                    onAdminAction({ action: "remove_one", itemId: row.id });
                  }}
                >
                  Delete
                </a>
              </th>
            </tr>
          ))}
          <tr>
            <th colSpan={6} />
          </tr>
        </tbody>
      </table>
      <table>
        <tbody>
          <tr>
            <td className="c">Add a single coupon</td>
          </tr>
          <tr>
            <td>
              <form action={adminModeActionHref("Coupons", "add_one")} method="POST" onSubmit={handleAddOne}>
                Dark Matter <input name="dm" size={10} type="text" /> <input type="submit" />
              </form>
            </td>
          </tr>
        </tbody>
      </table>
      {(admin.couponQueueRows ?? []).map((row) => (
        <React.Fragment key={row.id}>
          {adminCouponQueuePrintR(row)}
          <br />
        </React.Fragment>
      ))}
      <form action={adminModeActionHref("Coupons", "add_date")} method="POST" onSubmit={handleAddDate}>
        <table>
          <tbody>
            <tr>
              <td className="c" colSpan={2}>
                Holiday coupons
              </td>
            </tr>
            <tr>
              <td>
                Day in the format DD.MM <input name="ddmm" size={10} type="text" />
              </td>
              <td>
                Time in HH:MM format <input defaultValue="10:00" name="hhmm" size={10} type="text" />
              </td>
            </tr>
            <tr>
              <td>Dark matter per coupon</td>
              <td>
                <input defaultValue="100000" name="darkmatter" size={10} type="text" />
              </td>
            </tr>
            <tr>
              <td>Send players who are inactive at least</td>
              <td>
                <input defaultValue="7" name="inactive_days" size={10} type="text" /> days
              </td>
            </tr>
            <tr>
              <td>Players must play at least</td>
              <td>
                <input defaultValue="365" name="ingame_days" size={10} type="text" /> days
              </td>
            </tr>
            <tr>
              <td>Periodicity of days (0-no periodicity)</td>
              <td>
                <input defaultValue="365" name="periodic" size={10} type="text" />
              </td>
            </tr>
            <tr>
              <td colSpan={2}>
                <input type="submit" />
              </td>
            </tr>
          </tbody>
        </table>
      </form>
    </>
  );
}

function adminCouponQueuePrintR(row: GameAdminCouponQueueRow) {
  const packedCriteria = ((row.inactiveDays & 0xffff) << 16) | (row.ingameDays & 0xffff);
  return `Array ( [task_id] => ${row.id} [owner_id] => 99999 [type] => Coupon [sub_id] => ${row.amount} [obj_id] => ${packedCriteria} [level] => ${row.periodicDays} [start] => ${row.start} [end] => ${row.end} [prio] => ${row.priority} )`;
}

function AdminColonySettingsTable() {
  return React.createElement("span", { dangerouslySetInnerHTML: { __html: adminColonySettingsHTML() } });
}

function adminColonySettingsHTML(): string {
  const rows = [
    ["Colonies in positions 1-3", ["50", "120", "72"], ["t1_a", "t1_b", "t1_c"]],
    ["Colonies in positions 4-6", ["50", "150", "120"], ["t2_a", "t2_b", "t2_c"]],
    ["Colonies in positions 7-9", ["50", "120", "120"], ["t3_a", "t3_b", "t3_c"]],
    ["Colonies in positions 10-12", ["50", "120", "96"], ["t4_a", "t4_b", "t4_c"]],
    ["Colonies in positions 13-15 (and beyond)", ["50", "150", "96"], ["t5_a", "t5_b", "t5_c"]]
  ] as const;
  let html = `\n<table class="legacy-admin-colony-settings-table" >\n<form action="${legacyHTMLAttribute(
    adminModeHref("ColonySettings")
  )}" method="POST" >\n<tr><td class=c colspan=2>Colonization settings</td></tr>\n\n`;
  for (const [label, values, names] of rows) {
    html += `<tr><th>${legacyHTMLText(label)}</th><th>\n`;
    values.forEach((value, index) => {
      html += `    <input type="text" name="${names[index]}" maxlength="3" size="3" value="${value}" />\n`;
    });
    html += "</th></tr>\n\n";
  }
  html += `<tr><th colspan=2><input type="submit" value="Save" /></th></tr>\n\n</form>\n</table>\n\n<br/>\nThe diameter of a new colony is calculated by the formula: <pre>D = RND(a, b) * c</pre>\nEach range has its own parameters (a, b, c)<br/>\n`;
  return html;
}

function AdminMessagesTable({
  className,
  mode,
  rows,
  withFilter = false
}: {
  className: string;
  mode: string;
  rows: GameAdminMessageRow[];
  withFilter?: boolean;
}) {
  return (
    <table className="header legacy-admin-messages-outer">
      <tbody>
        <tr className="header">
          <td>
            <form action={adminModeHref(mode)} method="POST" onSubmit={(event) => event.preventDefault()}>
              <table className={className} width={519}>
                <tbody>
                  <tr>
                    <td className="c" colSpan={4}>
                      Messages
                    </td>
                  </tr>
                  <tr>
                    <th>Action</th>
                    <th>Date</th>
                    <th>From</th>
                    <th>Browser</th>
                  </tr>
                  {rows.map((row) => (
                    <React.Fragment key={row.id}>
                      <tr>
                        <th>
                          <input name={`delmes${row.id}`} type="checkbox" />
                        </th>
                        <th>{formatLegacyAdminMessageDate(row.date)}</th>
                        <th>
                          <AdminUserLink ownerId={row.ownerId} ownerName={row.ownerName} />
                          {` [${row.ip}] `}
                        </th>
                        <th>{`${row.agent} `}</th>
                      </tr>
                      <tr>
                        <td className="b"> </td>
                        <td className="b" colSpan={3} dangerouslySetInnerHTML={{ __html: sanitizeLegacyMessageHTML(row.text) }} />
                      </tr>
                    </React.Fragment>
                  ))}
                  <tr>
                    <td className="b"> </td>
                    <td className="b" colSpan={3} />
                  </tr>
                  <tr>
                    <th colSpan={4} style={{ padding: "0px 105px" }} />
                  </tr>
                  <tr>
                    <th colSpan={4}>
                      <select name="deletemessages">
                        <option value="deletemarked">Delete highlighted messages</option>
                        {withFilter ? <option value="deleteshown">Delete all displayed messages </option> : null}
                        <option value="deleteall">Delete all messages</option>
                      </select>
                      <input type="submit" value="ok" />
                    </th>
                  </tr>
                  <tr>
                    <td colSpan={4}>
                      <center> </center>
                    </td>
                  </tr>
                  {withFilter ? (
                    <tr>
                      <th colSpan={4}>
                        Debug message filter: <input name="filter" type="text" />
                        <input type="submit" value="Show" />
                      </th>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </form>
          </td>
        </tr>
      </tbody>
    </table>
  );
}

function AdminUserLink({ blankWhenMissing = false, ownerId, ownerName }: { blankWhenMissing?: boolean; ownerId: number; ownerName: string }) {
  if (!ownerName && blankWhenMissing) {
    return null;
  }
  const query = new URLSearchParams(window.location.search);
  query.set("mode", "Users");
  query.set("player_id", String(ownerId));
  const label = ownerName || `Unknown UserID ${ownerId}`;
  return <a href={gameRouteURL("/game/admin", `?${query.toString()}`)}>{label}</a>;
}

function AdminLoginsTable() {
  return (
    <form action={adminModeHref("Logins")} method="POST" onSubmit={(event) => event.preventDefault()}>
      <table className="legacy-admin-logins-table">
        <tbody>
          <tr>
            <td className="d">By user name:</td>
            <td>
              <input name="name" size={20} type="text" />
            </td>
          </tr>
          <tr>
            <td className="d">By User ID:</td>
            <td>
              <input name="id" size={20} type="text" />
            </td>
          </tr>
          <tr>
            <td className="d">By IP address:</td>
            <td>
              <input name="ip" size={20} type="text" />
            </td>
          </tr>
          <tr>
            <td className="d" colSpan={2}>
              <center>
                <input type="submit" value="Search" />
              </center>
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function AdminUserLogsTable({ rows }: { rows: GameAdminUserLogRow[] }) {
  return (
    <>
      <h2>Recent actions of the players</h2>
      <table className="legacy-admin-userlogs-table">
        <tbody>
          <tr>
            <td className="c">Date</td>
            <td className="c">Player</td>
            <td className="c">Category</td>
            <td className="c">Action</td>
          </tr>
          {rows.map((row) => (
            <tr key={row.id}>
              <td>{formatLegacyAdminUserLogDate(row.date)}</td>
              <td>
                <span dangerouslySetInnerHTML={{ __html: adminUserLogNameHTML(row) }} />
              </td>
              <td>{row.type}</td>
              <td dangerouslySetInnerHTML={{ __html: sanitizeLegacyMessageHTML(row.text) }} />
            </tr>
          ))}
        </tbody>
      </table>
      <h2>Action history</h2>
      <table>
        <tbody>
          <tr>
            <td>
              <form action={adminModeHref("UserLogs")} method="POST" onSubmit={(event) => event.preventDefault()}>
                <table className="legacy-admin-userlogs-filter-table">
                  <tbody>
                    <tr>
                      <td>User name</td>
                      <td>
                        <input name="name" size={20} type="text" /> (can be approximate)
                      </td>
                    </tr>
                    <tr>
                      <td>Category</td>
                      <td>
                        <select name="type">
                          <option value="ALL">All</option>
                          <option value="BUILD">Buildings / Demolition</option>
                          <option value="RESEARCH">Research</option>
                          <option value="SHIPYARD">Fleet building</option>
                          <option value="DEFENSE">Defense building</option>
                          <option value="FLEET">Fleet dispatch</option>
                          <option value="PLANET">Planet settings</option>
                          <option value="SETTINGS">Account settings / VM</option>
                          <option value="OPER">Operator actions</option>
                        </select>
                      </td>
                    </tr>
                    <tr>
                      <td>For the period</td>
                      <td>
                        <input defaultValue="2" name="days" size={2} type="text" /> days{" "}
                        <input name="hours" size={2} type="text" /> hr.
                      </td>
                    </tr>
                    <tr>
                      <td>Starting from.</td>
                      <td>
                        <input defaultValue={legacyYesterday()} name="since" size={20} type="text" /> DD.MM.YYYY
                      </td>
                    </tr>
                    <tr>
                      <td className="c" colSpan={2}>
                        <input type="submit" value="Submit" />
                      </td>
                    </tr>
                  </tbody>
                </table>
              </form>
            </td>
          </tr>
        </tbody>
      </table>
    </>
  );
}

function AdminBrowseTable() {
  return (
    <>
      <span className="legacy-admin-browse-title">Recent history of transitions (50 entries):</span>
      <br />
      <table className="legacy-admin-browse-table">
        <tbody />
      </table>
    </>
  );
}

function AdminFleetlogsTable({ onAdminAction, rows }: { onAdminAction: (action: GameAdminAction) => void; rows: GameAdminFleetLogRow[] }) {
  const handleSubmit = (event: React.FormEvent<HTMLDivElement>) => {
    const form = (event.target as HTMLElement | null)?.closest("form");
    if (!(form instanceof HTMLFormElement)) {
      return;
    }
    const input = Array.from(form.elements).find((element) => element instanceof HTMLInputElement && element.name.startsWith("order_"));
    if (!(input instanceof HTMLInputElement)) {
      return;
    }
    event.preventDefault();
    const taskId = Number(input.value);
    const action = input.name === "order_2min"
      ? "fleetlogs_2min"
      : input.name === "order_end"
        ? "fleetlogs_end"
        : "fleetlogs_return";
    onAdminAction({ action, taskId });
  };
  return (
    <div
      onSubmit={handleSubmit}
      dangerouslySetInnerHTML={{
        __html: adminFleetlogsHTML(rows)
      }}
    />
  );
}

function adminFleetlogsHTML(rows: GameAdminFleetLogRow[]) {
  let html = "<table class=\"legacy-admin-fleetlogs-table\">\n";
  html +=
    "<tr><td class=c>N</td> <td class=c>Timer</td> <td class=c>Order</td> <td class=c>Sent</td> <td class=c>Arriving</td><td class=c>Flight time</td> <td class=c>Start</td> <td class=c>Target</td> <td class=c>Fleet</td> <td class=c>Cargo</td> <td class=c>Fuel</td> <td class=c>ACS</td> <td class=c colspan=3>Command</td> </tr>\n";
  for (const row of rows) {
    const start = formatLegacyAdminFleetLogDateParts(row.start);
    const end = formatLegacyAdminFleetLogDateParts(row.end);
    const remaining = Math.max(0, row.end - Math.floor(Date.now() / 1000));
    html += "\n\n        <tr  >\n\n";
    html += `        <th  > ${row.number} </th>\n\n`;
    html += "        <th  >\n";
    html += `<table style="width:77px"><tr  ><th  ><div id='bxx${row.number}' title='${remaining}' star='${row.start}'> </th>`;
    html += `<tr><th  >${end.date} ${"<br>"} ${end.time}</th></tr></table>`;
    html += "\n        </th>\n        <th  >\n";
    html += adminFleetLogMissionHTML(row.mission);
    html += "\n        </th>\n";
    html += `        <th  >${end.date === "" ? "" : `${start.date} <br> ${start.time}`}</th>\n`;
    html += `        <th  >${end.date} <br> ${end.time}</th>\n`;
    html += "        <th  >\n";
    html += adminFleetLogDurationHTML(row.flightTime);
    html += "\n        </th>\n        <th  >\n";
    html += adminFleetLogPlanetHTML(row.origin);
    html += "\n        </th>\n        <th  >\n";
    html += adminFleetLogPlanetHTML(row.target);
    html += "\n        </th>\n        <th  >\n";
    html += adminFleetLogShips(row.ships);
    html += "\n        </th>\n        <th  >\n";
    html += adminFleetLogCargoHTML(row.cargo);
    html += "\n        </th>\n";
    html += `        <th  >\n        ${formatLegacyNumber(row.fuel)}\n        </th>\n`;
    html += `        <th  >\n${row.unionId > 0 ? row.unionId : "-"}        </th>\n\n`;
    html += `        <th  >\n         ${adminFleetLogCommandFormHTML("order_2min", row.taskId, "2m")}\n        </th>\n`;
    html += `        <th  >\n         ${adminFleetLogCommandFormHTML("order_end", row.taskId, "F")}\n        </th><th  >\n         ${adminFleetLogCommandFormHTML("order_return", row.taskId, "R")}\n        </th>\n`;
    html += "        </tr>\n\n";
  }
  html += "</table>\n";
  return html;
}

function adminFleetLogMissionHTML(mission: number) {
  if (mission >= 1000) {
    return `<a title="">${legacyHTMLText(adminFleetLogMissionName(mission))}</a>\n(Custom)`;
  }
  let base = mission;
  let suffix = '<a title="Уход на задание">(У)</a>';
  if (mission >= 200) {
    base = mission - 200;
    suffix = '<a title="На планете">(Д)</a>';
  } else if (mission >= 100) {
    base = mission - 100;
    suffix = '<a title="Возвращение к планете">(В)</a>';
  }
  return `<a title="">${legacyHTMLText(adminFleetLogMissionName(base))}</a>\n${suffix}`;
}

function adminFleetLogMissionName(mission: number) {
  switch (mission) {
    case 1:
    case 21:
      return "Attack";
    case 2:
      return "Joint attack";
    case 3:
      return "Transport";
    case 4:
      return "Station";
    case 5:
      return "Defend";
    case 6:
      return "Espionage";
    case 7:
      return "Colonise";
    case 8:
      return "Recycle";
    case 9:
      return "Destroy";
    case 15:
      return "Expedition";
    case 20:
      return "Missile Attack";
    default:
      return `Mission ${mission}`;
  }
}

function adminFleetLogDurationHTML(seconds: number) {
  return `<nobr>${legacyHTMLText(formatLegacyDuration(seconds))}</nobr><br><nobr>${formatLegacyRawInteger(seconds)} s</nobr>`;
}

function adminFleetLogPlanetHTML(planet: GameAdminFleetLogPlanet) {
  const planetName = `<a href="${legacyHTMLAttribute(adminPlanetHref(planet.id))}">${legacyHTMLText(planet.name)}</a>`;
  const coordinates = adminPlanetCoordHTML(planet.coordinates);
  const owner = planet.ownerId > 0
    ? `<a href="${legacyHTMLAttribute(adminUserHref(planet.ownerId))}">${legacyHTMLText(planet.ownerName)}</a>`
    : legacyHTMLText(planet.ownerName);
  return `${planetName} ${coordinates} <br>${owner}`;
}

function adminFleetLogShips(ships: GameFleetShipCount[]) {
  return ships.map((ship) => `${ship.name}:${formatLegacyNumber(ship.count)}`).join(" ");
}

function adminFleetLogCargoHTML(cargo: GameFleetResourceLoad[]) {
  if (cargo.length === 0) {
    return "-";
  }
  return cargo.map((resource) => `${legacyHTMLText(resource.name)}: ${formatLegacyNumber(resource.loaded)}<br>`).join("");
}

function adminFleetLogCommandFormHTML(name: string, taskID: number, label: string) {
  return `<form action="${legacyHTMLAttribute(gameRouteURL("/game/admin", window.location.search))}" method="POST">
    <input type="hidden" name="${legacyHTMLAttribute(name)}" value="${taskID}" />
        <input type="submit" value="${legacyHTMLAttribute(label)}" />
     </form>`;
}

const legacyAdminQueueCompactStyle = `
.compact-buttons {
    white-space: nowrap;
}

.compact-buttons form {
    display: inline-block;
    margin: 0 1px;
}

.btn-compact {
    padding: 2px 2px !important;
    font-size: 12px !important;
    margin: 0;
    line-height: 1.2;
    height: auto;
}

.btn-delete {
    border: 1px solid red;
}

.delete-form {
    display: inline-block;
}
`;

function AdminQueueTable({ onAdminAction, rows }: { onAdminAction: (action: GameAdminAction) => void; rows: GameAdminQueueRow[] }) {
  const [now, setNow] = React.useState(() => Math.floor(Date.now() / 1000));
  React.useEffect(() => {
    const id = window.setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => window.clearInterval(id);
  }, []);
  return (
    <>
      <table className="legacy-admin-queue-table">
        <tbody>
          <tr>
            <td className="c">End time</td>
            <td className="c">Player</td>
            <td className="c">Task type</td>
            <td className="c">Description</td>
            <td className="c">Priority</td>
            <td className="c">ID</td>
            <td className="c">Control</td>
          </tr>
          {rows.map((row, index) => {
            const freezeSeconds = row.freeze ? Math.max(0, now - row.frozen) : 0;
            const remaining = Math.max(0, row.end - now + freezeSeconds);
            const freezeAction = row.freeze ? "unfreeze" : "freeze";
            const freezeLabel = row.freeze ? "ADM_QUEUE_UNFREEZE" : "ADM_QUEUE_FREEZE";
            return (
              <tr key={row.id}>
                <th>
                  {" "}
                  <table>
                    <tbody>
                      <tr>
                        <th>
                          <div
                            className="legacy-admin-queue-countdown"
                            id={`bxx${index + 1}`}
                            title={String(remaining)}
                            {...{ star: String(row.start) }}
                          >
                            {formatLegacyCountdown(remaining)}
                          </div>
                        </th>
                      </tr>
                      <tr>
                        <th>{formatLegacyAdminQueueDate(row.end)}</th>
                      </tr>
                    </tbody>
                  </table>
                </th>
                <th>
                  <AdminUserLink ownerId={row.ownerId} ownerName={row.ownerName} />
                </th>
                <th>{row.type}</th>
                <th
                  dangerouslySetInnerHTML={{
                    __html: `${row.description}${row.freeze ? ` (ADM_QUEUE_FROZEN ${freezeSeconds})` : ""}`
                  }}
                />
                <th>{row.priority}</th>
                <th>{row.id}</th>
                <style>{legacyAdminQueueCompactStyle}</style>
                <th className="compact-buttons">
                  {" \n    "}
                  <form
                    action={adminModeHref("Queue")}
                    method="POST"
                    onSubmit={(event) => {
                      event.preventDefault();
                      onAdminAction({ action: "queue_end", taskId: row.id });
                    }}
                  >
                    <input name="order_end" type="hidden" value={row.id} />
                    <input className="btn-compact" type="submit" value="End" />
                  </form>
                  {"\n    "}
                  <form
                    action={adminModeHref("Queue")}
                    method="POST"
                    onSubmit={(event) => {
                      event.preventDefault();
                      onAdminAction({ action: row.freeze ? "queue_unfreeze" : "queue_freeze", taskId: row.id });
                    }}
                  >
                    <input name={`order_${freezeAction}`} type="hidden" value={row.id} />
                    <input className="btn-compact" type="submit" value={freezeLabel} />
                  </form>
                  {"\n    "}
                  <form
                    action={adminModeHref("Queue")}
                    className="delete-form"
                    method="POST"
                    onSubmit={(event) => {
                      event.preventDefault();
                      onAdminAction({ action: "queue_remove", taskId: row.id });
                    }}
                  >
                    <input name="order_remove" type="hidden" value={row.id} />
                    <input className="btn-compact btn-delete" type="submit" value="Delete" />
                  </form>
                  {"\n"}
                </th>
              </tr>
            );
          })}
        </tbody>
      </table>
      <br />
      <form action={adminModeHref("Queue")} method="POST" onSubmit={(event) => event.preventDefault()}>
        {"\n    Show player's tasks: "}
        <input defaultValue="" name="player" size={15} />
        {"\n    "}
        <input type="submit" value="Send" />
        {"\n    "}
      </form>
      <form action={adminModeHref("Queue")} method="POST" onSubmit={(event) => event.preventDefault()}>
        <input name="order_cron" type="hidden" value="1" />
        <input type="submit" value="ADM_QUEUE_CRON" />
      </form>
    </>
  );
}

function AdminUsersTable({ admin }: { admin: GameAdmin }) {
  if (admin.selectedUser) {
    return (
      <div
        className="legacy-admin-users-detail"
        dangerouslySetInnerHTML={{ __html: adminUserDetailHTML(admin.selectedUser) }}
        style={{ display: "contents" }}
      />
    );
  }
  return (
    <div
      className="legacy-admin-users-table"
      dangerouslySetInnerHTML={{ __html: adminUsersHTML(admin) }}
      style={{ display: "contents" }}
    />
  );
}

const adminResearchRows = [
  { id: 106, name: "Espionage Technology" },
  { id: 108, name: "Computer Technology" },
  { id: 109, name: "Weapons Technology" },
  { id: 110, name: "Shielding Technology" },
  { id: 111, name: "Armour Technology" },
  { id: 113, name: "Energy Technology" },
  { id: 114, name: "Hyperspace Technology" },
  { id: 115, name: "Combustion Drive" },
  { id: 117, name: "Impulse Drive" },
  { id: 118, name: "Hyperspace Drive" },
  { id: 120, name: "Laser Technology" },
  { id: 121, name: "Ion Technology" },
  { id: 122, name: "Plasma Technology" },
  { id: 123, name: "Intergalactic Research Network" },
  { id: 124, name: "Expedition Technology" },
  { id: 199, name: "Graviton Technology" }
];

function adminUserDetailHTML(user: GameAdminUserDetail): string {
  let html = "";
  html += "<table>\n";
  html += `<form action="${legacyHTMLAttribute(adminModeActionHref("Users", "update"))}" method="POST" >\n`;
  html += `<tr><td class=c>${adminUserNameHTML(user)}</td><td class=c>Settings</td><td class=c>Research</td></tr>\n`;
  html += '<th class="legacy-admin-users-left" valign=top style="width:244.71px"><table style="width:100%">\n';
  html += `<tr><th>ID</th><th>${user.playerId}</th></tr>\n`;
  html += `<tr><th>Date of registration</th><th>${formatLegacyAdminDateTime(user.regDate)}</th></tr>\n`;
  html += `<tr><th>Alliance</th><th>${legacyHTMLText(user.alliance)}</th></tr>\n`;
  html += `<tr><th>Join Date</th><th>${user.joinDate ? formatLegacyAdminDateTime(user.joinDate) : ""}</th></tr>\n`;
  html += adminTextInputRow("Permanent address", "pemail", 100, 20, user.permanentEmail);
  html += adminTextInputRow("Temporary address", "email", 100, 20, user.email);
  html += adminUserCheckboxDateRow("Delete player", "deaktjava", user.disable, user.disableUntil);
  html += adminUserCheckboxDateRow("Vacation mode", "vacation", user.vacation, user.vacationUntil);
  html += adminUserCheckboxDateRow("Blocked", "banned", user.banned, user.bannedUntil);
  html += adminUserCheckboxDateRow("Attack ban", "noattack", user.noAttack, user.noAttackUntil);
  html += `<tr><th>Last login</th><th>${formatLegacyAdminDateTime(user.lastLogin)}</th></tr>\n`;
  html += `<tr><th>Activity</th><th>${formatLegacyAdminDateTime(user.lastClick)}</th></tr>\n`;
  html += `<tr><th>IP address</th><th><a href="http://nic.ru/whois/?query=${legacyHTMLAttribute(user.ipAddress)}" target=_blank>${legacyHTMLText(
    user.ipAddress
  )}</a></th></tr>\n`;
  html += `<tr><th>Activated</th><th><input type="checkbox" name="validated" ${adminChecked(
    user.validated
  )} /> <a href="${legacyHTMLAttribute(adminModeActionHref("Users", "reactivate"))}">send password</a></th></tr>\n`;
  html += `<tr><th>Home Planet</th><th>${user.homePlanet ? adminUserPlanetRefHTML(user.homePlanet) : ""}</th></tr>\n`;
  html += `<tr><th>Current planet</th><th>${user.activePlanet ? adminUserPlanetRefHTML(user.activePlanet) : "[0:0:0] <a href=\"\"></a>"}</th></tr>\n`;
  html += `<tr><th>Rights</th><th>${adminUserLevelSelectHTML(user.adminLevel)}</th></tr>\n`;
  html += adminCheckboxRow("Enable tracking", "sniff", user.sniff);
  html += adminCheckboxRow("Debug information", "debug", user.debug);
  html += `<tr><th colspan=2><a href="${legacyHTMLAttribute(adminModeActionHref("Users", "bot_start"))}" >[Start bot]</a></th></tr>\n`;
  html += "</table></th>\n";

  html += '<th class="legacy-admin-users-center" valign=top style="width:391.2px"><table style="width:100%">\n';
  html += '<colgroup><col style="width:130.1px"><col></colgroup>\n';
  html += `<tr><th>Planet sorting</th><th>${adminUserSortSelectHTML("settings_sort", user.sortBy, ["colonization order", "coordinates", "alphabetically"])}</th></tr>\n`;
  html += `<tr><th>Sorting order</th><th>${adminUserSortSelectHTML("settings_order", user.sortOrder, ["ascending", "descending"])}</th></tr>\n`;
  html += adminTextInputRow("Skin", "dpath", 80, 40, user.skin);
  html += adminCheckboxRow("Use skin", "design", user.useSkin);
  html += adminCheckboxRow("Deactivate the IP check", "deact_ip", user.deactivateIP);
  html += adminTextInputRow("Number of probes", "spio_anz", 2, 2, user.maxSpy);
  html += adminTextInputRow("Number of fleet messages", "settings_fleetactions", 2, 2, user.maxFleetMsg);
  html += "<tr><th colspan=2>&nbsp</th></tr>\n";
  html += "<tr><td class=c colspan=2>Statistics</td></tr>\n";
  html += `<tr><th>Points (old)</th><th>${formatLegacyNumber(Math.floor(user.oldScore1 / 1000))} / ${formatLegacyNumber(user.oldPlace1)}</th></tr>\n`;
  html += `<tr><th>Fleet (old)</th><th>${formatLegacyNumber(user.oldScore2)} / ${formatLegacyNumber(user.oldPlace2)}</th></tr>\n`;
  html += `<tr><th>Research (old)</th><th>${formatLegacyNumber(user.oldScore3)} / ${formatLegacyNumber(user.oldPlace3)}</th></tr>\n`;
  html += `<tr><th>Points</th><th>${formatLegacyNumber(Math.floor(user.score1 / 1000))} / ${formatLegacyNumber(user.place1)}</th></tr>\n`;
  html += `<tr><th>Fleet</th><th>${formatLegacyNumber(user.score2)} / ${formatLegacyNumber(user.place2)}</th></tr>\n`;
  html += `<tr><th>Research</th><th>${formatLegacyNumber(user.score3)} / ${formatLegacyNumber(user.place3)}</th></tr>\n`;
  html += `<tr><th>Date of old statistic</th><th>${formatLegacyAdminDateTime(user.scoreDate)}</th></tr>\n`;
  html += `<tr><th colspan=2><a href="${legacyHTMLAttribute(adminModeActionHref("Users", "recalc_stats"))}" >[Recalculate stats]</a></th></tr>\n`;
  html += "<tr><th colspan=2>&nbsp</th></tr>\n";
  html += "<tr><td class=c colspan=2>Officers</td></tr>\n";
  html += `<tr><th colspan=2><table><tr>${["commander", "admiral", "ingenieur", "geologe", "technokrat"]
    .map((name, index) => `<td align="center" width="35" class="header"><img border="0" src="/public-assets/game/img/${name}_ikon_un.gif" width="32" height="32"></td><td><input type="text" name="pr_${index + 1}" size="3" /></td>`)
    .join("")}</tr></table></th></tr>\n`;
  html += "<tr><th colspan=2><i>To extend an officer, specify the required number of days in the<br>input fields.<br>To delete, specify a value less than zero.</i></th></tr>\n";
  html += "</table></th>\n";

  html += '<th class="legacy-admin-users-right" valign=top style="width:182px"><table style="width:100%">\n';
  for (const row of adminResearchRows) {
    html += `<tr><th>${legacyHTMLText(row.name)}</th><th><input type="text" size=3 name="r${row.id}" value="${
      user.research[String(row.id)] ?? 0
    }" /></th></tr>\n`;
  }
  html += `<tr><td colspan=2>&nbsp;</td></tr>\n`;
  html += `<tr><th>Found Dark Matter</th><th><input type="text" size=5 name="dmfree" value="${user.darkMatterFree}" /></th></tr>\n`;
  html += `<tr><th>Purchased Dark Matter</th><th><input type="text" size=5 name="dm" value="${user.darkMatter}" /></th></tr>\n`;
  html += "</table></th>\n";
  html += `<tr><th colspan=3><input type="submit" value="Save" /></th></tr>\n`;
  html += "</form>\n</table>\n";
  html += adminUserPlanetListHTML(user);
  html += adminUserFleetLogLinksHTML(user);
  return html;
}

function adminUserCheckboxDateRow(label: string, name: string, checked: boolean, date: number): string {
  return `<tr><th>${legacyHTMLText(label)}</th><th><input type="checkbox" name="${legacyHTMLAttribute(name)}"  ${adminChecked(
    checked
  )}/>${checked && date ? ` ${formatLegacyAdminDateTime(date)}` : ""}</th></tr>\n`;
}

function adminUserPlanetRefHTML(planet: GameAdminUserPlanet): string {
  const coordinates = planet.coordinates;
  return `[${coordinates.galaxy}:${coordinates.system}:${coordinates.position}] <a href="${legacyHTMLAttribute(
    adminPlanetHref(planet.id)
  )}">${legacyHTMLText(planet.name)}</a>`;
}

function adminUserLevelSelectHTML(selected: number): string {
  const options = [
    [0, "User"],
    [1, "Operator"],
    [2, "Administrator"]
  ] as const;
  return `<select name="admin">${options
    .map(([value, label]) => `<option value="${value}" ${adminSelected(selected, value)}>${legacyHTMLText(label)}</option>`)
    .join("")}</select>`;
}

function adminUserSortSelectHTML(name: string, selected: number, labels: string[]): string {
  return `<select name="${legacyHTMLAttribute(name)}">${labels
    .map((label, index) => `<option value="${index}" ${adminSelected(selected, index)}>${legacyHTMLText(label)}</option>`)
    .join("")}</select>`;
}

function adminUserPlanetListHTML(user: GameAdminUserDetail): string {
  let html = "<br>\n<table>\n";
  html += `<form action="${legacyHTMLAttribute(adminModeActionHref("Users", "create_planet"))}" method="POST" >\n`;
  html += '<tr><td class=c colspan=20>Planet list</td></tr>\n<tr>\n';
  user.planets.forEach((planet, index) => {
    const coordinates = planet.coordinates;
    const planetType = /moon/i.test(planet.name) ? 0 : 1;
    html += `<td> <img src="${legacyHTMLAttribute(planetImagePath({ id: planet.id, type: planetType, coordinates }, true))}" width="32px" height="32px"></td>\n`;
    html += `<td> <a href="${legacyHTMLAttribute(adminPlanetHref(planet.id))}"> ${legacyHTMLText(planet.name)} </a> [<a href="${legacyHTMLAttribute(
      adminGalaxyHref(coordinates)
    )}">${coordinates.galaxy}:${coordinates.system}:${coordinates.position}</a>] </td>\n`;
    if ((index + 1) % 10 === 0) {
      html += "</tr>\n<tr>\n";
    }
  });
  html += "</tr>\n";
  html += '<tr><td colspan=20> Coordinates: <input name="g" size=2> <input name="s" size=2> <input name="p" size=2> <input type="submit" value="Create planet"></td></tr>\n';
  html += "</form>\n</table>\n";
  return html;
}

function adminUserFleetLogLinksHTML(user: GameAdminUserDetail): string {
  const orders = [
    "All",
    "Attack",
    "ACS attack",
    "Transport",
    "Deployment",
    "ACS hold",
    "Espionage",
    "Colonisation",
    "Recycle debris field",
    "Moon destruction",
    "Expedition",
    "Missile attack",
    "ACS head"
  ];
  let html = "<br>\n<table>\n";
  html += '<tr><td class=c colspan=3>Fleet logs</td></tr>\n';
  html += `<tr><td>Order</td><td>From ${legacyHTMLText(user.name)}</td><td>To ${legacyHTMLText(user.name)}</td></tr>\n`;
  orders.forEach((label, index) => {
    html += `<tr><td>${legacyHTMLText(label)}</td><td><a href="${legacyHTMLAttribute(adminModeActionHref("Users", "fleetlogs"))}&from=1&mission=${index}">Show</a></td><td><a href="${legacyHTMLAttribute(
      adminModeActionHref("Users", "fleetlogs")
    )}&from=0&mission=${index}">Show</a></td></tr>\n`;
  });
  html += "</table>\n";
  return html;
}

function adminUsersHTML(admin: GameAdmin): string {
  const users = admin.userRows ?? [];
  const activeUsers = admin.activeUsers ?? [];
  let html = "";
  html += "New users:<br>\n";
  html += "<table>\n";
  html += '<tr><td class=c>Date of registration</td><td class=c>Home Planet</td><td class=c>Player Name</td></tr>\n';
  for (const user of users) {
    html += `<tr><th>${formatLegacyAdminDateTime(user.regDate)}</th>`;
    html += `<th>${user.homePlanet ? adminUserHomePlanetHTML(user.homePlanet) : "-"}</th>`;
    html += `<th>${adminUserNameHTML(user)}</th></tr>\n`;
  }
  html += "</table>\n";
  html += "\n    <br>\n    <table>\n";
  html += `    <tr><td class=c>Active in the last 24 hours (${activeUsers.length})</td></tr>\n`;
  html += "    <tr><td>\n";
  html += activeUsers.map((user) => adminUserNameHTML(user)).join(", ");
  html += "\n    </td></tr>\n    </table>\n";
  return html;
}

function adminUserHomePlanetHTML(planet: GameAdminUserPlanet): string {
  const coordinates = planet.coordinates;
  return `[${coordinates.galaxy}:${coordinates.system}:${coordinates.position}] <a href="${legacyHTMLAttribute(
    adminPlanetHref(planet.id)
  )}">${legacyHTMLText(planet.name)}</a>`;
}

function adminUserNameHTML(user: GameAdminUserRow): string {
  let name = legacyHTMLText(user.name);
  const status = adminUserStatus(user);
  if (status !== "") {
    name += ` (${legacyHTMLText(status)})`;
  }
  const color = adminUserColor(user);
  if (color !== "") {
    name = `<font color=${legacyHTMLAttribute(color)}>${name}</font>`;
  }
  return `<a href="${legacyHTMLAttribute(adminUserHref(user.playerId))}">${name}</a>`;
}

function adminUserLogNameHTML(row: GameAdminUserLogRow): string {
  if (!row.ownerName) {
    return "";
  }
  return adminUserNameHTML({
    playerId: row.ownerId,
    name: row.ownerName,
    regDate: 0,
    lastClick: row.lastClick,
    vacation: row.vacation,
    banned: row.banned,
    noAttack: row.noAttack,
    disable: row.disable,
  });
}

function adminUserStatus(user: GameAdminUserRow): string {
  const now = Math.floor(Date.now() / 1000);
  let status = "";
  if (user.lastClick <= now - 604800) {
    status += "i";
  }
  if (user.lastClick <= now - 604800 * 4) {
    status += "I";
  }
  if (user.vacation) {
    status += "v";
  }
  if (user.banned) {
    status += "b";
  }
  if (user.noAttack) {
    status += "\u0410";
  }
  if (user.disable) {
    status += "g";
  }
  return status;
}

function adminUserColor(user: GameAdminUserRow): string {
  const now = Math.floor(Date.now() / 1000);
  if (user.disable) {
    return "orange";
  }
  if (user.banned) {
    return "red";
  }
  if (user.noAttack) {
    return "yellow";
  }
  if (user.vacation) {
    return "skyBlue";
  }
  if (user.lastClick <= now - 604800 * 4) {
    return "#999999";
  }
  if (user.lastClick <= now - 604800) {
    return "#cccccc";
  }
  return "";
}

function AdminPlanetsTable({ admin }: { admin: GameAdmin }) {
  if (admin.selectedPlanet) {
    return (
      <div
        className="legacy-admin-planets-detail"
        dangerouslySetInnerHTML={{ __html: adminPlanetDetailHTML(admin.selectedPlanet) }}
        style={{ display: "contents" }}
      />
    );
  }
  return (
    <div
      className="legacy-admin-planets-table"
      dangerouslySetInnerHTML={{ __html: adminPlanetsHTML(admin) }}
      style={{ display: "contents" }}
    />
  );
}

function adminPlanetDetailHTML(planet: GameAdminPlanetDetail): string {
  const owner = planet.owner;
  let html = "<table>\n";
  html += `<form action="${legacyHTMLAttribute(adminModeActionHref("Planets", "update"))}" method="POST" >\n`;
  html += `<tr><td class=c colspan=2>Planet "${legacyHTMLText(planet.name)}" (${
    owner ? `<a href="${legacyHTMLAttribute(adminUserHref(owner.playerId))}">${legacyHTMLText(owner.name)}</a>` : ""
  })</td><td class=c>Buildings</td> <td class=c>Fleet</td> <td class=c>Defense</td> </tr>\n`;
  html += "<tr>";
  html += `<th><img src="${legacyHTMLAttribute(planetImagePath(planet, false))}"> <br>Type: ${planet.type}`;
  html += `<br>Points: ${formatLegacyNumber(Math.floor(planet.score.points / 1000))}`;
  html += `<br>Buildings: ${formatLegacyNumber(Math.floor((planet.score.points - (planet.score.fleetPoints + planet.score.defensePoints)) / 1000))}`;
  html += `<br>Fleet: ${formatLegacyNumber(Math.floor(planet.score.fleetPoints / 1000))}`;
  html += `<br>Defense: ${formatLegacyNumber(Math.floor(planet.score.defensePoints / 1000))}</th>`;
  html += "<th>";
  if (planet.type === 1) {
    if (planet.moon) {
      html += `<a href="${legacyHTMLAttribute(adminPlanetHref(planet.moon.id))}"><img src="${legacyHTMLAttribute(
        planetImagePath({ id: planet.moon.id, type: 0, coordinates: planet.moon.coordinates }, true)
      )}"><br>\n${legacyHTMLText(planet.moon.name)}</a>`;
    } else {
      html += `<a href="${legacyHTMLAttribute(adminModeActionHref("Planets", "create_moon"))}" >Create moon</a>\n`;
    }
    html += "<br/><br/>\n";
    if (planet.debris) {
      html += `<a href="${legacyHTMLAttribute(adminPlanetHref(planet.debris.id))}"><img src="${skinBase}/planeten/debris.jpg"><br>\n${legacyHTMLText(
        planet.debris.name
      )}</a>`;
    } else {
      html += `<a href="${legacyHTMLAttribute(adminModeActionHref("Planets", "create_debris"))}" >Create debris field</a>\n`;
    }
    html += "<br/><br/>\n";
  }
  html += '<br><br><textarea rows=10 cols=10 id="spiotext"></textarea>';
  html += '<a href="#">Parse espionage report</a> <br><a href="#">Reset</a>';
  html += "</th>";
  html += `<th valign=top>${adminTechnologyValueTableHTML(planet.buildings, true)}</th>\n`;
  html += `<th valign=top>${adminTechnologyValueTableHTML(planet.fleet, true)}</th>\n`;
  html += `<th valign=top>${adminTechnologyValueTableHTML(planet.defense, false)}</th>\n`;
  html += "</tr>\n";
  html += `<tr><th>Creation date</th><th>${formatLegacyAdminDateTime(planet.date)}</th> <td colspan=10 class=c>Build Queue</td></tr>`;
  html += `<tr><th>Date of removal</th><th>${formatLegacyAdminDateTime(planet.removeDate)}</th> <th colspan=3 rowspan=12 valign=top style='text-align: left;'>${adminPlanetBuildQueueHTML(
    planet
  )}</th></tr>`;
  html += `<tr><th>Last activity</th><th>${formatLegacyAdminDateTime(planet.lastActivity)}</th><input type="hidden" name="type" value="${planet.type}" ></tr>\n`;
  html += `<tr><th>Last state update</th><th>${formatLegacyAdminDateTime(planet.lastUpdate)}</th></tr>\n`;
  html += `<tr><th>Diameter <br><a href="${legacyHTMLAttribute(adminModeActionHref("Planets", "random_diam"))}" >new diameter</a>  </th><th><input size=5 type="text" name="diameter" value="${
    planet.diameter
  }" /> km (${planet.fields} / ${planet.maxFields} fields) <a href="${legacyHTMLAttribute(
    adminModeActionHref("Planets", "recalc_fields")
  )}" >recalc fields</a> </th></tr>\n`;
  html += `<tr><th>Temperature</th><th> <input size=5 type="text" name="temp" value="${planet.temperature}" />°C  -  ${
    planet.temperature + 40
  }°C</th></tr>\n`;
  html += `<tr><th>Coordinates</th><th>[<input type="text" name="g" value="${planet.coordinates.galaxy}" size=1 />:<input type="text" name="s" value="${
    planet.coordinates.system
  }" size=2 />:<input type="text" name="p" value="${planet.coordinates.position}" size=1 />]</th></tr>\n`;
  html += '<tr><td class=c colspan=2>Resources</td></tr>\n';
  html += `<tr><th>Metal</th><th><input id="obj700" type="text" name="700" value="${Math.ceil(planet.resources.metal)}" /></th></tr>\n`;
  html += `<tr><th>Crystal</th><th><input id="obj701" type="text" name="701" value="${Math.ceil(planet.resources.crystal)}" /></th></tr>\n`;
  html += `<tr><th>Deuterium</th><th><input id="obj702" type="text" name="702" value="${Math.ceil(planet.resources.deuterium)}" /></th></tr>\n`;
  html += `<tr><th>Energy</th><th>${planet.energyBalance} / ${planet.energyCapacity}</th></tr>\n`;
  html += `<tr><th>Factor</th><th>${formatProductionFactor(planet.productionFactor)}</th></tr>\n`;
  html += '<tr><th colspan=8><input type="submit" value="Save" />  <input type="submit" name="delete_planet" value="Remove" /> </th></tr>\n';
  html += "</form>\n</table>\n";
  return html;
}

function adminTechnologyValueTableHTML(rows: GameAdminTechnologyValue[], withProduction: boolean): string {
  let html = "<table>\n";
  for (const row of rows) {
    html += `<tr><th>${legacyHTMLText(row.name)}`;
    if (row.id === 43) {
      html += " ";
    }
    html += `</th><th><nobr><input id="obj${row.id}" type="text" size=${row.id < 200 ? 3 : 6} name="${row.id}" value="${row.value}" />`;
    if (withProduction && row.percent >= 0 && adminHasProductionSelect(row.id)) {
      html += adminPlanetProductionSelectHTML(row.id, row.percent);
    }
    html += "</nobr></th></tr>\n";
  }
  html += "</table>";
  return html;
}

function adminHasProductionSelect(id: number): boolean {
  return [1, 2, 3, 4, 12, 212].includes(id);
}

function adminPlanetProductionSelectHTML(id: number, percent: number): string {
  const name = id === 212 ? "prod212" : `prod${id}`;
  let html = `<select name='${name}'>\n`;
  for (let value = 0; value <= 100; value += 10) {
    const raw = value / 100;
    html += `<option value='${raw}' ${adminSelected(percent, value)}>${value}</option>\n`;
  }
  html += "</select>\n";
  return html;
}

function adminPlanetBuildQueueHTML(planet: GameAdminPlanetDetail): string {
  let html = "<table>";
  planet.buildQueue.forEach((entry, index) => {
    html += `<tr><td> <table><tr><th><div id='bxx${index + 1}' title='0' star='0'></th>`;
    html += `<tr><th>${formatLegacyAdminDateTime(entry.end)}</th></tr></table></td>`;
    html += `<td><img width='32px' src='${skinBase}/gebaeude/${entry.techId}.gif'></td>`;
    html += `<td><b>${legacyHTMLText(entry.name)}</b><br>level ${entry.level}</td></tr>`;
  });
  html += "</table>";
  return html;
}

function adminPlanetsHTML(admin: GameAdmin): string {
  const planets = admin.planetRows ?? [];
  let html = "";
  html += "New Planets:<br>\n";
  html += "<table>\n";
  html += '<tr><td class=c>Creation date</td><td class=c>Coordinates</td><td class=c>Planet</td><td class=c>Player</td></tr>\n';
  for (const planet of planets) {
    html += `<tr><th>${formatLegacyAdminDateTime(planet.date)}</th><th>${adminPlanetCoordHTML(planet.coordinates)}</th>`;
    html += `<th><a href="${legacyHTMLAttribute(adminPlanetHref(planet.id))}">${legacyHTMLText(planet.name)}</a></th>`;
    html += `<th>${planet.owner ? adminUserNameHTML(planet.owner) : ""}</th></tr>\n`;
  }
  html += "</table>\n";
  html += "\n       </th> \n       </tr> \n    </table>\n";
  html += "    Search:<br>\n";
  html += ` <form action="${legacyHTMLAttribute(adminModeActionHref("Planets", "search"))}" method="post">\n`;
  html += " <table>\n  <tr>\n   <th>\n";
  html += '    <select name="type">\n';
  html += '     <option value="playername">Player name</option>\n';
  html += '     <option value="planetname" >Planet name</option>\n';
  html += '     <option value="allytag" >Ally tag</option>\n';
  html += "    </select>\n";
  html += "    &nbsp;&nbsp;\n";
  html += '    <input type="text" name="searchtext" value=""/>\n';
  html += "    &nbsp;&nbsp;\n";
  html += '    <input type="submit" value="Search" />\n';
  html += "   </th>\n  </tr>\n </table>\n </form>\n";
  return html;
}

function adminPlanetCoordHTML(coordinates: Coordinates): string {
  return `[<a href="${legacyHTMLAttribute(adminGalaxyHref(coordinates))}">${coordinates.galaxy}:${coordinates.system}:${coordinates.position}</a>]`;
}

function AdminUniverseTable({ admin }: { admin: GameAdmin }) {
  const universe = admin.universe;
  if (!universe) {
    return null;
  }
  return (
    <div
      className="legacy-admin-universe-table"
      dangerouslySetInnerHTML={{ __html: adminUniverseHTML(universe) }}
      style={{ display: "contents" }}
    />
  );
}

function adminUniverseHTML(universe: GameAdminUniverseSettings): string {
  let html = "";
  html += "<table >\n";
  html += `<form action="${legacyHTMLAttribute(adminModeHref("Uni"))}" method="POST" >\n`;
  html += `<tr><td class=c colspan=2>Universe ${universe.number} Settings</td></tr>\n`;
  html += `<tr><th>Date of opening</th><th>${formatLegacyAdminDateTime(universe.startDate)}</th></tr>\n`;
  html += `<tr><th>Hack attempt counter <a title="Any SQL injection attempts are logged for all players and the counter is incremented after each attempt. Cleared after relogin"><img src='/public-assets/game/img/r5.png' /></a></th><th><a href="${legacyHTMLAttribute(
    adminHackCheckHref()
  )}">${universe.hacks} (Check)</a></th></tr>\n`;
  html += `<tr><th>Number of players</th><th>${universe.userCount}</th></tr>\n`;
  html += adminTextInputRow("Maximum number of players", "maxusers", 10, 10, universe.maxUsers);
  html += adminTextInputRow("The amount of starting Dark Matter", "start_dm", 10, 10, universe.startDarkMatter);
  html += adminTextInputRow("Number of galaxies", "galaxies", 3, 3, universe.galaxies);
  html += adminTextInputRow("Number of systems in the galaxy", "systems", 3, 3, universe.systems);
  html += adminTextInputRow("Maximum number of units in a shipyard order", "max_werf", 9, 9, universe.maxShipyard);
  html += adminTextInputRow("RSS/Atom refresh period in minutes for Commander", "feedage", 3, 3, universe.feedAge);
  html += adminSpeedSelectRow("Game speed", "speed", universe.speed);
  html += adminSpeedSelectRow("Fleet speed", "fspeed", universe.fleetSpeed);
  html += adminPercentSelectRow("Fleet into the debris", "fid", universe.fleetDebris);
  html += adminPercentSelectRow("Defense into the debris", "did", universe.defenseDebris);
  html += `<tr><th>Restoring Defense</th><th>\n<input type="text" name="defrepair" maxlength="3" size="3" value="${legacyHTMLAttribute(
    String(universe.defenseRepair)
  )}" /> +/-\n<input type="text" name="defrepair_delta" maxlength="3" size="3" value="${legacyHTMLAttribute(
    String(universe.defenseDelta)
  )}" /> %\n</th></tr>\n`;
  html += `<tr><th>Invited players to the ACS</th><th><input type="text" name="acs" maxlength="3" size="3" value="${legacyHTMLAttribute(
    String(universe.acs)
  )}" /> (max ${universe.acs * universe.acs} fleets)</th></tr>\n`;
  html += adminCheckboxRow("Rapidfire", "rapid", universe.rapidFire);
  html += adminCheckboxRow("Moons and Death Stars", "moons", universe.moons);
  html += adminTextInputRow("News 1", "news1", 99, 20, universe.news1);
  html += adminTextInputRow("News 2", "news2", 99, 20, universe.news2);
  if (Math.floor(Date.now() / 1000) > universe.newsUntil) {
    html += '<tr><th>Prolong the news</th><th><input type="text" name="news_upd" maxlength="3" size="3" value="0" /> days</th></tr>\n';
  } else {
    html += `<tr><th>Show the news until</th><th>${formatLegacyAdminDateTime(
      universe.newsUntil
    )} <input type="checkbox" name="news_off"  /> remove</th></tr>\n`;
  }
  html += '<tr><th>Interface language</th><th>\n   <select name="lang">\n';
  for (const language of adminUniverseLanguages) {
    html += `    <option value="${legacyHTMLAttribute(language.id)}" ${adminSelected(universe.language, language.id)} >${legacyHTMLText(
      language.name
    )}</option>\n`;
  }
  html += "   </select>\n</th></tr>\n";
  html += adminCheckboxRow("Forced to use the language of the universe", "force_lang", universe.forceLanguage);
  html += adminTextInputRow("Board", "ext_board", 99, 20, universe.extBoard);
  html += adminTextInputRow("Discord", "ext_discord", 99, 20, universe.extDiscord);
  html += adminTextInputRow("Help", "ext_tutorial", 99, 20, universe.extTutorial);
  html += adminTextInputRow("Rules", "ext_rules", 99, 20, universe.extRules);
  html += adminTextInputRow("Impressum", "ext_impressum", 99, 20, universe.extImpressum);
  html += adminTextInputRow("Path to battle engine", "battle_engine", 99, 20, universe.battleEngine);
  html += adminCheckboxRow("Use a PHP-based battle engine", "php_battle", universe.phpBattle);
  html += adminTextInputRow("Maximum number of units on one side", "battle_max", 99, 20, universe.battleMax);
  html += `<tr><th>Pause the universe <a title="When the universe is paused, no events will be triggered (the queue will be stopped). After the pause is removed, all completed events will be executed in the queue order. All active players are forced into vacation mode."><img src='/public-assets/game/img/r5.png' /></a>\n</th><th><input type="checkbox" name="freeze"  ${adminChecked(
    universe.freeze
  )} /></th></tr>\n`;
  html += '<tr><th colspan=2><input type="submit" value="Save" /></th></tr>\n\n';
  html += "</form>\n</table>\n";
  return html;
}

const adminUniverseLanguages = [
  { id: "de", name: "Deutsch" },
  { id: "en", name: "English" },
  { id: "es", name: "Espa\u00f1ol" },
  { id: "fr", name: "Fran\u00e7ais" },
  { id: "it", name: "Italiano" },
  { id: "jp", name: "\u65e5\u672c\u8a9e" },
  { id: "ru", name: "\u0420\u0443\u0441\u0441\u043a\u0438\u0439" }
];

function adminTextInputRow(label: string, name: string, maxLength: number, size: number, value: string | number): string {
  return `<tr><th>${legacyHTMLText(label)}</th><th><input type="text" name="${legacyHTMLAttribute(name)}" maxlength="${maxLength}" size="${size}" value="${legacyHTMLAttribute(
    String(value)
  )}" /></th></tr>\n`;
}

function adminCheckboxRow(label: string, name: string, checked: boolean): string {
  return `<tr><th>${legacyHTMLText(label)}</th><th><input type="checkbox" name="${legacyHTMLAttribute(name)}"  ${adminChecked(checked)} /></th></tr>\n`;
}

function adminSpeedSelectRow(label: string, name: string, selected: number): string {
  let html = `\n  <tr>\n   <th>${legacyHTMLText(label)}</th>\n   <th>\n   <select name="${legacyHTMLAttribute(name)}">\n`;
  for (let value = 1; value <= 10; value += 1) {
    html += `     <option value="${value}" ${adminSelected(selected, value)}>${value}x</option>\n`;
  }
  html += "   </select>\n   </th>\n </tr>\n\n";
  return html;
}

function adminPercentSelectRow(label: string, name: string, selected: number): string {
  let html = `\n  <tr>\n   <th>${legacyHTMLText(label)}</th>\n   <th>\n   <select name="${legacyHTMLAttribute(name)}">\n`;
  for (let value = 0; value <= 100; value += 10) {
    html += `     <option value="${value}" ${adminSelected(selected, value)}>${value}%</option>\n`;
  }
  html += "   </select>\n   </th>\n </tr>\n\n";
  return html;
}

function adminSelected(option: string | number, value: string | number): string {
  return String(option) === String(value) ? "selected" : "";
}

function adminChecked(checked: boolean): string {
  return checked ? "checked" : "";
}

function AdminChecksumTable({ groups }: { groups: GameAdminChecksumGroup[] }) {
  return (
    <>
      {groups.map((group) => (
        <React.Fragment key={group.title}>
          <h2>{group.title}</h2>
          <table className="legacy-admin-checksum-table" width={519}>
            <tbody>
              <tr>
                <td className="c">File path</td>
                <td className="c">Checksum</td>
                <td className="c">Status</td>
              </tr>
              {group.rows.map((row) => (
                <tr key={`${group.title}-${row.path}`}>
                  <td>{row.path}</td>
                  <td>{row.checksum}</td>
                  <td>
                    <LegacyFont color={row.status === "OK" ? "lime" : "red"}>
                      <b>{row.status}</b>
                    </LegacyFont>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </React.Fragment>
      ))}
      <br />
      <form action={adminModeHref("Checksum")} method="POST" onSubmit={(event) => event.preventDefault()}>
        <input type="submit" value="Fix Checksums" />
      </form>
    </>
  );
}

function AdminDatabaseTable({ backups }: { backups: GameAdminDatabaseBackup[] }) {
  return <div dangerouslySetInnerHTML={{ __html: adminDatabaseHTML(backups) }} />;
}

function adminDatabaseHTML(backups: GameAdminDatabaseBackup[]): string {
  let html = "";
  html += "<h2>Database Backup</h2>\n";
  html += "&#9888;&#65039; Know what you're doing. Mindlessly pressing buttons can lead to unfortunate consequences!<br/>";
  html += '<table class="legacy-admin-db-table">\n';
  html += "<tr><td class=c>File name</td><td class=c>Operation</td></tr>\n";
  for (const backup of backups) {
    const fileName = legacyHTMLText(backup.fileName);
    const restoreHref = legacyHTMLAttribute(adminModeFileActionHref("DB", "restore", backup.fileName));
    const deleteHref = legacyHTMLAttribute(adminModeFileActionHref("DB", "delete", backup.fileName));
    html += `<tr><td>${fileName}<br/></td><td><a href="${restoreHref}">Restore</a> <a href="${deleteHref}">Delete</a></td></tr>\n`;
  }
  html += "</table>\n";
  html += "<br/>\n";
  html += `<form action="${legacyHTMLAttribute(adminModeActionHref("DB", "create"))}" method="POST"><input type=submit value="Create a database backup" /></form>`;
  html += "<h2>Comparison of tables from install and real database</h2>";
  html += "<font color=green>No differences were found.</font><br/>";
  html += "<h2>Comparison of real database and tables from install</h2>";
  html += "<font color=green>No differences were found.</font><br/>";
  return html;
}

const adminSimFleetRows = [
  { id: 202, name: "Small Cargo" },
  { id: 203, name: "Large Cargo" },
  { id: 204, name: "Light Fighter" },
  { id: 205, name: "Heavy Fighter" },
  { id: 206, name: "Cruiser" },
  { id: 207, name: "Battleship" },
  { id: 208, name: "Colony Ship" },
  { id: 209, name: "Recycler" },
  { id: 210, name: "Espionage Probe" },
  { id: 211, name: "Bomber" },
  { id: 212, name: "Solar Satellite" },
  { id: 213, name: "Destroyer" },
  { id: 214, name: "Deathstar" },
  { id: 215, name: "Battlecruiser" }
];

const adminSimDefenseRows = ["Rocket Launcher", "Light Laser", "Heavy Laser", "Gauss Cannon", "Ion Cannon", "Plasma Turret", "Small Shield Dome", "Large Shield Dome"];
const adminBattleSimMaxSlot = 9;
const adminRakSimDefenseRows = [
  { id: 401, name: "Rocket Launcher", missileTarget: true },
  { id: 402, name: "Light Laser", missileTarget: true },
  { id: 403, name: "Heavy Laser", missileTarget: true },
  { id: 404, name: "Gauss Cannon", missileTarget: true },
  { id: 405, name: "Ion Cannon", missileTarget: true },
  { id: 406, name: "Plasma Turret", missileTarget: true },
  { id: 407, name: "Small Shield Dome", missileTarget: true },
  { id: 408, name: "Large Shield Dome", missileTarget: true },
  { id: 502, name: "Anti-Ballistic Missiles", missileTarget: false },
  { id: 503, name: "Interplanetary Missiles", missileTarget: false }
];

function AdminBattleSimTable() {
  const handleBattleSimEvent = (event: React.SyntheticEvent<HTMLDivElement>) => {
    adminBattleSimHandleEventTarget(event.target);
  };
  React.useEffect(() => {
    const legacyWindow = window as Window & {
      OnChangeSlot?: (attacker: number) => void;
      OnChangeValue?: (attacker: number, id: number) => void;
      OnChangeTechValue?: (attacker: number) => void;
      RecalcAttackersDefendersNum?: () => void;
      toint?: (value: unknown) => number;
    };
    const previous = {
      OnChangeSlot: legacyWindow.OnChangeSlot,
      OnChangeValue: legacyWindow.OnChangeValue,
      OnChangeTechValue: legacyWindow.OnChangeTechValue,
      RecalcAttackersDefendersNum: legacyWindow.RecalcAttackersDefendersNum,
      toint: legacyWindow.toint
    };
    legacyWindow.toint = adminBattleSimToInt;
    legacyWindow.RecalcAttackersDefendersNum = adminBattleSimRecalcAttackersDefendersNum;
    legacyWindow.OnChangeSlot = adminBattleSimOnChangeSlot;
    legacyWindow.OnChangeValue = adminBattleSimOnChangeValue;
    legacyWindow.OnChangeTechValue = adminBattleSimOnChangeTechValue;
    const cleanupListeners = adminBattleSimAttachEventHandlers();
    adminBattleSimRecalcAttackersDefendersNum();
    return () => {
      cleanupListeners();
      legacyWindow.OnChangeSlot = previous.OnChangeSlot;
      legacyWindow.OnChangeValue = previous.OnChangeValue;
      legacyWindow.OnChangeTechValue = previous.OnChangeTechValue;
      legacyWindow.RecalcAttackersDefendersNum = previous.RecalcAttackersDefendersNum;
      legacyWindow.toint = previous.toint;
    };
  }, []);
  return <div dangerouslySetInnerHTML={{ __html: adminBattleSimHTML() }} onChange={handleBattleSimEvent} onInput={handleBattleSimEvent} onKeyUp={handleBattleSimEvent} />;
}

function adminBattleSimHTML(): string {
  const action = legacyHTMLAttribute(adminModeHref("BattleSim"));
  let html = "";
  html += `<table class="legacy-admin-battlesim-table" cellpadding=0 cellspacing=0>\n`;
  html += `<form name="simForm" action="${action}" method="POST" >\n\n`;
  html += '<input type="hidden" id="anum" name="anum" value="1" />\n';
  html += '<input type="hidden" id="dnum" name="dnum" value="1" />\n\n';
  html += "<tr>        <td class=c>Attacker</td>                <td class=c>Defender</td>  </tr>\n\n";
  html += "<tr> \n<td> \n";
  html += '    Weapons: <input id="a_weap" size=2  onKeyUp="OnChangeTechValue(1);"  value="0" > \n';
  html += '    Shields: <input id="a_shld" size=2  onKeyUp="OnChangeTechValue(1);"  value="0" > \n';
  html += '    Armor: <input id="a_armor" size=2  onKeyUp="OnChangeTechValue(1);"  value="0" ></td> \n';
  html += "<td> \n";
  html += '    Weapons: <input id="d_weap" size=2  onKeyUp="OnChangeTechValue(0);"  value="0" > \n';
  html += '    Shields: <input id="d_shld" size=2  onKeyUp="OnChangeTechValue(0);"  value="0" > \n';
  html += '    Armor: <input id="d_armor" size=2  onKeyUp="OnChangeTechValue(0);"  value="0" ></td> \n';
  html += "</tr>\n\n";
  html += "        <tr> <th valign=top>\n        <table>\n";
  html += adminBattleSimFleetSection("a");
  html += "\n<tr><td colspan=2> \n<table>\n";
  html += "<tr><td class=c colspan=2>Settings</td></tr>\n";
  html += '<tr><td>Debug information</td><td><input type="checkbox" name="debug"  ></td></tr>\n';
  html += '<tr><td>Rapidfire</td><td><input type="checkbox" name="rapid" checked ></td></tr>\n';
  html += '<tr><td>Fleet in debris</td><td><input name="fid" size=3 value="30"> </td></tr>\n';
  html += '<tr><td>Defense in debris</td><td><input name="did" size=3 value="0"></td></tr>\n';
  html += '<tr><td>ADM_SIM_MAX_ROUND</td><td><input name="max_round" size=3 value="6"></td></tr>\n';
  html += "</table>\n</td></tr>\n\n        </table>\n        </th>\n\n        <th valign=top>\n        <table>\n";
  html += adminBattleSimFleetSection("d");
  html += '<tr><td class=c><b>Defense</b></td></tr>\n';
  html += adminSimDefenseRows
    .map((name, index) => `           <tr><td> ${legacyHTMLText(name)} </td> <td> <input id="d_${401 + index}" size=5 onKeyUp="OnChangeValue(0, ${401 + index});" value="0" > </td> </tr>\n`)
    .join("");
  html += "        </table>\n        </th></tr>\n\n";
  html += "<tr><td colspan=2> \n<table>\n";
  html += "<tr><td class=c colspan=2>ADM_SIM_BATTLE_SOURCE</td></tr>\n";
  html += '<tr><td><textarea id="battle_source" name="battle_source"></textarea></td></tr>\n';
  html += "</table>\n</td></tr>\n\n";
  html += '<tr><td colspan=2><center><input type="submit" value="Start the Battle"></center></td></tr>\n\n';
  html += adminBattleSimHiddenInputs();
  html += "\n</form>\n</table>\n";
  return html;
}

function adminBattleSimFleetSection(prefix: "a" | "d"): string {
  const slotHandler = prefix === "a" ? 1 : 0;
  const valueHandler = prefix === "a" ? 1 : 0;
  let html = `<tr><td class=c><b>Fleet</b></td> <td>Slot: <select name="${prefix}slot" onchange="OnChangeSlot(${slotHandler});">\n${adminBattleSimSlotOptions()}</select> </td>  </tr>\n`;
  html += adminSimFleetRows
    .map((row) => `           <tr><td> ${legacyHTMLText(row.name)} </td> <td> <input id="${prefix}_${row.id}" size=5  onKeyUp="OnChangeValue(${valueHandler}, ${row.id});" value="0" > </td> </tr>\n`)
    .join("");
  return html;
}

function adminBattleSimSlotOptions(): string {
  let html = "";
  for (let n = 1; n <= adminBattleSimMaxSlot; n++) {
    html += `<option value="${n}">${n}</option>\n`;
  }
  return html;
}

function adminBattleSimHiddenInputs(): string {
  const hidden: string[] = [];
  for (let n = 0; n < adminBattleSimMaxSlot; n++) {
    for (const row of adminSimFleetRows) {
      hidden.push(`<input type="hidden" id="a${n}_${row.id}" name="a${n}_${row.id}" value="0"  /> `);
    }
    for (const row of adminSimFleetRows) {
      hidden.push(`<input type="hidden" id="d${n}_${row.id}" name="d${n}_${row.id}" value="0"  /> `);
    }
    for (let index = 0; index < adminSimDefenseRows.length; index++) {
      hidden.push(`<input type="hidden" id="d${n}_${401 + index}" name="d${n}_${401 + index}" value="0"  /> `);
    }
    hidden.push(`<input type="hidden" id="a${n}_weap" name="a${n}_weap" size=2 value="0"  /> `);
    hidden.push(`<input type="hidden" id="a${n}_shld" name="a${n}_shld" size=2 value="0"  /> `);
    hidden.push(`<input type="hidden" id="a${n}_armor" name="a${n}_armor" size=2 value="0"  /> \n`);
    hidden.push(`<input type="hidden" id="d${n}_weap" name="d${n}_weap" size=2 value="0"  /> `);
    hidden.push(`<input type="hidden" id="d${n}_shld" name="d${n}_shld" size=2 value="0"  /> `);
    hidden.push(`<input type="hidden" id="d${n}_armor" name="d${n}_armor" size=2 value="0"  /> \n`);
  }
  return `${hidden.join("\n")}\n`;
}

function adminBattleSimToInt(value: unknown): number {
  const parsed = Number.parseInt(String(value ?? "0"), 10);
  return Number.isFinite(parsed) ? parsed : 0;
}

function adminBattleSimInput(id: string): HTMLInputElement | null {
  const element = document.getElementById(id);
  return element instanceof HTMLInputElement ? element : null;
}

function adminBattleSimInputValue(id: string): string {
  return adminBattleSimInput(id)?.value ?? "";
}

function adminBattleSimSetInputValue(id: string, value: string): void {
  const input = adminBattleSimInput(id);
  if (input) {
    input.value = value;
  }
}

function adminBattleSimSetVisibleValue(id: string, value: string): void {
  adminBattleSimSetInputValue(id, adminBattleSimToInt(value) > 0 ? value : "");
}

function adminBattleSimSelectedSlot(prefix: "a" | "d"): number {
  const slot = document.querySelector<HTMLSelectElement>(`select[name="${prefix}slot"]`);
  if (slot instanceof HTMLSelectElement) {
    return Math.max(0, adminBattleSimToInt(slot.value) - 1);
  }
  return 0;
}

function adminBattleSimHandleEventTarget(target: EventTarget | null): void {
  if (target instanceof HTMLSelectElement) {
    if (target.name === "aslot") {
      adminBattleSimOnChangeSlot(1);
    } else if (target.name === "dslot") {
      adminBattleSimOnChangeSlot(0);
    }
    return;
  }
  if (!(target instanceof HTMLInputElement)) {
    return;
  }
  const valueMatch = /^([ad])_(\d+)$/.exec(target.id);
  if (valueMatch) {
    adminBattleSimOnChangeValue(valueMatch[1] === "a" ? 1 : 0, Number(valueMatch[2]));
    return;
  }
  const techMatch = /^([ad])_(weap|shld|armor)$/.exec(target.id);
  if (techMatch) {
    adminBattleSimOnChangeTechValue(techMatch[1] === "a" ? 1 : 0);
  }
}

function adminBattleSimAttachEventHandlers(): () => void {
  const cleanups: Array<() => void> = [];
  const addInputListener = (id: string, listener: EventListener) => {
    const input = adminBattleSimInput(id);
    if (!input) {
      return;
    }
    input.addEventListener("input", listener);
    input.addEventListener("keyup", listener);
    cleanups.push(() => {
      input.removeEventListener("input", listener);
      input.removeEventListener("keyup", listener);
    });
  };
  const addSelectListener = (name: string, listener: EventListener) => {
    const select = document.querySelector<HTMLSelectElement>(`select[name="${name}"]`);
    if (!(select instanceof HTMLSelectElement)) {
      return;
    }
    select.addEventListener("change", listener);
    cleanups.push(() => select.removeEventListener("change", listener));
  };
  addSelectListener("aslot", () => adminBattleSimOnChangeSlot(1));
  addSelectListener("dslot", () => adminBattleSimOnChangeSlot(0));
  for (const row of adminSimFleetRows) {
    addInputListener(`a_${row.id}`, () => adminBattleSimOnChangeValue(1, row.id));
    addInputListener(`d_${row.id}`, () => adminBattleSimOnChangeValue(0, row.id));
  }
  for (let index = 0; index < adminSimDefenseRows.length; index += 1) {
    const id = 401 + index;
    addInputListener(`d_${id}`, () => adminBattleSimOnChangeValue(0, id));
  }
  for (const tech of ["weap", "shld", "armor"]) {
    addInputListener(`a_${tech}`, () => adminBattleSimOnChangeTechValue(1));
    addInputListener(`d_${tech}`, () => adminBattleSimOnChangeTechValue(0));
  }
  return () => {
    for (const cleanup of cleanups) {
      cleanup();
    }
  };
}

function adminBattleSimRecalcAttackersDefendersNum(): void {
  let attackers = 1;
  let defenders = 1;
  for (let slot = 0; slot < adminBattleSimMaxSlot; slot += 1) {
    const attackerShips = adminSimFleetRows.reduce((sum, row) => sum + adminBattleSimToInt(adminBattleSimInputValue(`a${slot}_${row.id}`)), 0);
    if (attackerShips > 0) {
      attackers = slot + 1;
    }
    const defenderShips = adminSimFleetRows.reduce((sum, row) => sum + adminBattleSimToInt(adminBattleSimInputValue(`d${slot}_${row.id}`)), 0);
    const defenderDefense = adminSimDefenseRows.reduce((sum, _row, index) => sum + adminBattleSimToInt(adminBattleSimInputValue(`d${slot}_${401 + index}`)), 0);
    if (defenderShips + defenderDefense > 0) {
      defenders = slot + 1;
    }
  }
  adminBattleSimSetInputValue("anum", String(attackers));
  adminBattleSimSetInputValue("dnum", String(defenders));
}

function adminBattleSimOnChangeSlot(attackerValue: number): void {
  const attacker = adminBattleSimToInt(attackerValue) > 0;
  const prefix = attacker ? "a" : "d";
  const slot = adminBattleSimSelectedSlot(prefix);
  for (const row of adminSimFleetRows) {
    adminBattleSimSetVisibleValue(`${prefix}_${row.id}`, adminBattleSimInputValue(`${prefix}${slot}_${row.id}`));
  }
  if (!attacker) {
    for (let index = 0; index < adminSimDefenseRows.length; index += 1) {
      const id = 401 + index;
      adminBattleSimSetVisibleValue(`d_${id}`, adminBattleSimInputValue(`d${slot}_${id}`));
    }
  }
  for (const tech of ["weap", "shld", "armor"]) {
    adminBattleSimSetVisibleValue(`${prefix}_${tech}`, adminBattleSimInputValue(`${prefix}${slot}_${tech}`));
  }
  adminBattleSimRecalcAttackersDefendersNum();
}

function adminBattleSimOnChangeValue(attackerValue: number, id: number): void {
  const prefix = adminBattleSimToInt(attackerValue) > 0 ? "a" : "d";
  const slot = adminBattleSimSelectedSlot(prefix);
  adminBattleSimSetInputValue(`${prefix}${slot}_${id}`, adminBattleSimInputValue(`${prefix}_${id}`));
  adminBattleSimRecalcAttackersDefendersNum();
}

function adminBattleSimOnChangeTechValue(attackerValue: number): void {
  const prefix = adminBattleSimToInt(attackerValue) > 0 ? "a" : "d";
  const slot = adminBattleSimSelectedSlot(prefix);
  for (const tech of ["weap", "shld", "armor"]) {
    adminBattleSimSetInputValue(`${prefix}${slot}_${tech}`, adminBattleSimInputValue(`${prefix}_${tech}`));
  }
}

function AdminExpeditionTable({ admin, onAdminAction }: { admin: GameAdmin; onAdminAction: (action: GameAdminAction) => void }) {
  if (!admin.expedition) {
    return null;
  }
  const handleSubmit = (event: React.FormEvent<HTMLDivElement>) => {
    const form = event.target;
    if (!(form instanceof HTMLFormElement)) {
      return;
    }
    event.preventDefault();
    const action = new URL(form.action, window.location.href).searchParams.get("action") ?? "";
    if (action !== "settings") {
      return;
    }
    const data = new FormData(form);
    const values: Record<string, number> = {};
    for (const name of adminExpeditionSettingNames) {
      values[name] = Number(data.get(name)) || 0;
    }
    onAdminAction({ action: "settings", values });
  };
  return (
    <div
      className="legacy-admin-expedition-table"
      dangerouslySetInnerHTML={{ __html: adminExpeditionHTML(admin.expedition) }}
      onSubmit={handleSubmit}
      style={{ display: "contents" }}
    />
  );
}

const adminExpeditionSettingNames = [
  "dm_factor",
  "chance_success",
  "depleted_min",
  "depleted_med",
  "depleted_max",
  "chance_depleted_min",
  "chance_depleted_med",
  "chance_depleted_max",
  "chance_alien",
  "chance_pirates",
  "chance_dm",
  "chance_lost",
  "chance_delay",
  "chance_accel",
  "chance_res",
  "chance_fleet",
  "score_cap1",
  "limit_cap1",
  "score_cap2",
  "limit_cap2",
  "score_cap3",
  "limit_cap3",
  "score_cap4",
  "limit_cap4",
  "score_cap5",
  "limit_cap5",
  "score_cap6",
  "limit_cap6",
  "score_cap7",
  "limit_cap7",
  "score_cap8",
  "limit_cap8",
  "limit_max"
];

function adminExpeditionHTML(values: Record<string, number>): string {
  let html = "";
  html += "<h2>Expedition Settings</h2>\n";
  html += `<form action="${legacyHTMLAttribute(adminModeActionHref("Expedition", "settings"))}" method="POST">\n`;
  html += "<table>\n";
  html += adminExpeditionInputRow("The multiplier of Dark Matter found", "dm_factor", values);
  html += adminExpeditionInputRow("Chance of successful expedition (if >= then success); Successful expedition if something happened.", "chance_success", values);
  html += '<tr><td class=c colspan=2>Expedition depletion settings</td></tr>\n';
  html += adminExpeditionInputRow("Visit count without depletion (if <= there is no depletion)", "depleted_min", values);
  html += adminExpeditionInputRow("Visit count for moderate depletion (if <= then moderate depletion)", "depleted_med", values);
  html += adminExpeditionInputRow("Visit count for significant depletion (if <= then significantly depleted. A value higher is severe depletion)", "depleted_max", values);
  html += adminExpeditionInputRow("Chance of failure for moderate depletion (>= expedition failure)", "chance_depleted_min", values);
  html += adminExpeditionInputRow("Chance of failure for significant depletion (>= expedition failure)", "chance_depleted_med", values);
  html += adminExpeditionInputRow("Chance of failure for severe depletion (>= expedition failure)", "chance_depleted_max", values);
  html += '<tr><td class=c colspan=2>The following checks are performed sequentially (type of successful expedition)</td></tr>\n';
  html += adminExpeditionInputRow("Meeting aliens (if the die value >=)", "chance_alien", values);
  html += adminExpeditionInputRow("Meet the pirates (otherwise if the die value is >=)", "chance_pirates", values);
  html += adminExpeditionInputRow("Finding Dark Matter (otherwise if the die value is >=)", "chance_dm", values);
  html += adminExpeditionInputRow("The loss of a fleet in a black hole (otherwise if the die value is >=)", "chance_lost", values);
  html += adminExpeditionInputRow("Delayed return (otherwise if the die value is >=)", "chance_delay", values);
  html += adminExpeditionInputRow("Faster return (otherwise if the die value is >=)", "chance_accel", values);
  html += adminExpeditionInputRow("Finding resources (otherwise if the die value is >=)", "chance_res", values);
  html += adminExpeditionInputRow("Finding the fleet (otherwise if the die value is >=)", "chance_fleet", values);
  html += "<tr><td class=d>Otherwise, the Merchant will be found</td> <td> &nbsp; </td></tr>\n\n";
  html += '<tr><td class=c colspan=2>Settings for determining the upper limit of expedition points (affects the size of the find)</td></tr>\n';
  for (let index = 1; index <= 8; index += 1) {
    html += `<tr><td class=d>If top1 has less than (${index}) points, the expedition limit will be (${index})</td> <td> <input type=text size=20 name=score_cap${index} value="${adminExpeditionValue(
      values,
      `score_cap${index}`
    )}">  <input type=text size=20 name=limit_cap${index} value="${adminExpeditionValue(values, `limit_cap${index}`)}"></td></tr>\n`;
  }
  html += `<tr><td class=d>Otherwise, the limit of the expedition will be maxed out</td> <td> <input type=text size=20 name=limit_max value="${adminExpeditionValue(
    values,
    "limit_max"
  )}"> </td></tr>\n\n`;
  html += '<tr><td colspan=2 class=d><center><input type="submit" value="Save"></center></td></tr>\n';
  html += "</table>\n</form>\n\n";
  html += "For all expedition rolls a 100-sided die [0, 99] is thrown (including 0 and 99). If some parameters seem unclear to you, you will have to examine the source code.\n\n\n\n\n";
  html += "<h2>Expedition Simulator</h2>\n";
  html += `<form action="${legacyHTMLAttribute(adminModeActionHref("Expedition", "sim"))}" method="POST">\n`;
  html += "<table>\n";
  html += '<tr><td class=d>Number of expeditions</td> <td> <input type=text size=20 name=expcount value="1000"></td></tr>\n';
  html += '<tr><td colspan=2 class=d><center><input type="submit" value="Simulate"></center></td></tr>\n';
  html += "</table>\n</form>\n\n";
  return html;
}

function adminExpeditionInputRow(label: string, name: string, values: Record<string, number>): string {
  return `<tr><td class=d>${legacyHTMLText(label)}</td> <td> <input type=text size=20 name=${legacyHTMLAttribute(name)} value="${adminExpeditionValue(
    values,
    name
  )}"></td></tr>\n`;
}

function adminExpeditionValue(values: Record<string, number>, name: string): string {
  return legacyHTMLAttribute(String(values[name] ?? 0));
}

function AdminBattleReportsTable({ rows }: { rows: GameAdminBattleReportRow[] }) {
  return (
    <table className="legacy-admin-battle-report-table">
      <tbody>
        {rows.map((row) => (
          <tr key={row.id}>
            <td>{formatLegacyAdminBattleReportDate(row.date)}</td>
            <td dangerouslySetInnerHTML={{ __html: sanitizeLegacyMessageHTML(legacyAdminHTMLWithSession(row.title)) }} />
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function AdminBotEditTable({ admin }: { admin: GameAdmin }) {
  React.useEffect(() => {
    if (admin.viewer.level < 2) {
      return;
    }
    let cancelled = false;
    loadLegacyBotEditorScripts().then(() => {
      if (!cancelled) {
        const legacyWindow = window as Window & { init?: () => void; session?: string };
        legacyWindow.session = new URLSearchParams(window.location.search).get("session") ?? "";
        const strategySelect = document.getElementById("strategyId") as HTMLSelectElement | null;
        const selectedStrategy = strategySelect?.value;
        if (strategySelect) {
          strategySelect.value = "";
        }
        legacyWindow.init?.();
        if (strategySelect && selectedStrategy !== undefined) {
          strategySelect.value = selectedStrategy;
        }
      }
    });
    return () => {
      cancelled = true;
    };
  }, [admin.viewer.level]);

  if (admin.viewer.level < 2) {
    return <LegacyFont color="red">Access denied.</LegacyFont>;
  }
  return <div dangerouslySetInnerHTML={{ __html: adminBotEditHTML(admin) }} />;
}

const legacyBotEditorScripts = ["/public-assets/game/js/tw-sack.js", "/public-assets/game/js/go.js", "/public-assets/game/js/go-game.js"];
const legacyScriptLoads = new Map<string, Promise<void>>();

function loadLegacyBotEditorScripts(): Promise<void> {
  return legacyBotEditorScripts.reduce((chain, src) => chain.then(() => loadLegacyScript(src)), Promise.resolve());
}

function loadLegacyScript(src: string): Promise<void> {
  const existing = legacyScriptLoads.get(src);
  if (existing) {
    return existing;
  }
  const promise = new Promise<void>((resolve, reject) => {
    const current = document.querySelector<HTMLScriptElement>(`script[data-ogame-legacy-src="${src}"]`);
    if (current) {
      resolve();
      return;
    }
    const script = document.createElement("script");
    script.src = src;
    script.dataset.ogameLegacySrc = src;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error(`failed to load legacy script: ${src}`));
    document.body.appendChild(script);
  });
  legacyScriptLoads.set(src, promise);
  return promise;
}

function adminBotEditHTML(admin: GameAdmin): string {
  const importAction = legacyHTMLAttribute(adminModeActionHref("BotEdit", "import"));
  const strategyOptions = (admin.botStrategies ?? [])
    .map((strategy) => `<option value="${strategy.id}">${legacyHTMLText(strategy.name)}</option>\n`)
    .join("");
  return `<div id="sample" class="legacy-admin-botedit-table">
  <div style="width:100%; white-space:nowrap;">
    <span style="display: inline-block; vertical-align: top; padding: 5px; width:100px">
      <div id="myPalette" style="background-color: #344566; border: solid 1px black; height: 500px"></div>
    </span>
    <span style="display: inline-block; vertical-align: top; padding: 5px; width:88%">
      <div id="myDiagram" style="background-color: #344566; border: solid 1px black; height: 500px"></div>
    </span>
  </div>

<span style="float:left;">
 Name of the edited strategy: <input type="text" size="50" id="strategyName">
 <button onclick="newstrat()">New</button>
 <button onclick="rename()">Rename</button>
 <button onclick="showimg()">Show</button>
 <button onclick="export_strat()">Export</button>
</span>

<span style="float:right;">
  <button onclick="save()">Save</button>
<select id="strategyId">
<option value="0">-- Choose a strategy --</option>
${strategyOptions}</select>
  <button onclick="load()">Load</button>
</span>
  <textarea id="mySavedModel" style="width:100%;height:300px; display:none;">
{ "class": "go.GraphLinksModel",
  "linkFromPortIdProperty": "fromPort",
  "linkToPortIdProperty": "toPort",
  "nodeDataArray": [ ],
  "linkDataArray": [ ]}
  </textarea>
</div>

<form action="${importAction}" method="post" enctype="multipart/form-data">
 <input type="hidden" id="strategyId_ForImport" name="strategyId_ForImport" value="0" >
 <input type="file" name="fileToUpload" id="fileToUpload" /> <input type="submit" value="ADM_BOTEDIT_IMPORT" />
</form>

<img src="" id="preview_img" style="display:none;">`;
}

function AdminRakSimTable() {
  return <div dangerouslySetInnerHTML={{ __html: adminRakSimHTML() }} />;
}

function adminRakSimHTML(): string {
  const action = legacyHTMLAttribute(adminModeHref("RakSim"));
  let html = "";
  html += `<table class="legacy-admin-raksim-table" cellpadding=0 cellspacing=0>\n`;
  html += `<form name="simForm" action="${action}" method="POST" >\n\n`;
  html += "<tr>        <td class=c>Attacker</td>                <td class=c>Defender</td>  </tr>\n\n";
  html += "<tr> \n<td> \n";
  html += '    Weapons: <input type="text" name="a_weap" size=2 value="0"> \n';
  html += "<td> \n";
  html += '    Armor: <input type="text" name="d_armor" size=2 value="0"></td> \n';
  html += "</tr>\n\n\n";
  html += "        <tr> <th valign=top>\n        <table>\n\n<tr><td colspan=2> \n<table>\n";
  html += "<tr><td class=c colspan=2>Settings</td></tr>\n\n";
  html += '<tr><td>\nInterplanetary Missiles:     <input type="text" name="anz" size="2" maxlength="2" value="0"/></td></tr>\n\n';
  html += '    <tr><td>\n    Target:\n     <select name="pziel">\n';
  html += '      <option value="0" selected >Target all</option>\n';
  for (const row of adminRakSimDefenseRows) {
    if (!row.missileTarget) {
      break;
    }
    html += `       <option value="${row.id}" >${legacyHTMLText(row.name)}</option>\n`;
  }
  html += "           </select>\n    </td></tr>\n\n</table>\n</td></tr>\n\n        </table>\n        </th>\n\n\n\n        <th valign=top>\n        <table>\n\n";
  html += '<tr><td class=c colspan=2><b>Defense</b></td></tr>\n';
  html += adminRakSimDefenseRows.map((row) => `           <tr><td> ${legacyHTMLText(row.name)} </td> <td> <input name="d_${row.id}" size=5 value=0> </td> </tr>\n`).join("");
  html += "        </table>\n        </th></tr>            \n\n\n";
  html += '<tr><td colspan=2><center><input type="submit" value="Missile attack"></center></td></tr>\n';
  html += "</form>\n</table>\n";
  return html;
}

function AdminLocaTable() {
  const locaDirectories = ["de_de", "en_en", "es_es", "fr_fr", "it_it", "jp_jp", "ru_ru"];
  return (
    <form action={adminModeActionHref("Loca", "search")} method="POST" onSubmit={(event) => event.preventDefault()}>
      <table className="legacy-admin-loca-table">
        <tbody>
          <tr>
            <td className="c" colSpan={2}>
              Compare localization between the specified languages
            </td>
          </tr>
          <tr>
            <td>Source language:</td>
            <td>
              <select name="loca_src" defaultValue="de_de">
                {locaDirectories.map((directory) => (
                  <option key={directory} value={directory}>
                    {directory}
                  </option>
                ))}
              </select>
            </td>
          </tr>
          <tr />
          <tr>
            <td>Target language:</td>
            <td>
              <select name="loca_dst" defaultValue="de_de">
                {locaDirectories.map((directory) => (
                  <option key={directory} value={directory}>
                    {directory}
                  </option>
                ))}
              </select>
            </td>
          </tr>
          <tr>
            <td className="c" colSpan={2}>
              <input type="submit" value="Compare" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

type AdminModInfo = {
  folder: string;
  name: string;
  version: string;
  author: string;
  description: string;
  website: string;
};

const adminAvailableMods: AdminModInfo[] = [
  {
    folder: "BogusMod",
    name: "Bogus Modification",
    version: "1.0.0",
    author: "ogamespec",
    description: "A simple modification to demonstrate the capabilities",
    website: "https://github.com/ogamespec/ogame-opensource"
  },
  {
    folder: "DeepSpaceHorror",
    name: "Deep Space Horror",
    version: "1.0.0",
    author: "ogamespec",
    description:
      "Ancient cosmic horrors stir from the abyss, roaming the galaxy and leaving behind only fleet wreckage and bountiful rewards for the bold.",
    website: "https://github.com/ogamespec/ogame-opensource"
  },
  {
    folder: "GalaxyTool",
    name: "GalaxyTool",
    version: "1.0.0",
    author: "ogamespec",
    description: "Integrated Galaxytool",
    website: "https://github.com/ogamespec/ogame-opensource"
  },
  {
    folder: "SpaceStorm",
    name: "Space Storm",
    version: "1.0.0",
    author: "ogamespec",
    description:
      "As a global event, the Space Storm can temporarily change the game mechanics themselves, creating unique tactical situations.",
    website: "https://github.com/ogamespec/ogame-opensource"
  }
];

function AdminModsTable() {
  return (
    <>
      <h2 className="legacy-admin-mods-heading">ADM_MODS_HEAD</h2>
      <div className="legacy-admin-mods-table mods-container">
        <div className="mod-column">
          <h3>ADM_MODS_HEAD_ACITVE</h3>
          <div className="empty-message">ADM_MODS_NO_ACTIVE</div>
        </div>
        <div className="mod-column">
          <h3>ADM_MODS_HEAD_AVAILABLE</h3>
          {adminAvailableMods.map((mod) => (
            <AdminModPanel key={mod.folder} mod={mod} />
          ))}
        </div>
      </div>
      <div style={{ color: "#E6EBFB", marginTop: 20, textAlign: "center" }}>
        <p>ADM_MODS_TOT_ACTIVE: 0 | ADM_MODS_TOT_AVAILABLE: {adminAvailableMods.length}</p>
      </div>
    </>
  );
}

function AdminModPanel({ mod }: { mod: AdminModInfo }) {
  return (
    <div className="mod-item">
      <span className="status-indicator status-inactive">ADM_MODS_STATE_AVAILABLE</span>
      <img alt={mod.name} className="mod-background" src={`/public-assets/game/mods/${mod.folder}/img/bg.png`} />
      <div className="mod-content">
        <div className="mod-title">{mod.name}</div>
        <div className="mod-description">{mod.description}</div>
        <div className="mod-info" dangerouslySetInnerHTML={{ __html: adminModInfoHTML(mod) }} />
        <div className="mod-actions">
          <a className="mod-action-link" href={adminModeModActionHref("Mods", "install", mod.folder)}>
            ADM_MODS_OP_INSTALL
          </a>
        </div>
      </div>
    </div>
  );
}

function adminModInfoHTML(mod: AdminModInfo) {
  const version = escapeHTML(mod.version);
  const author = escapeHTML(mod.author);
  const website = escapeHTML(mod.website);
  return `\n                    ADM_MODS_INFO_VERSION: ${version}<br>\n                    ADM_MODS_INFO_AUTHOR: ${author}<br>\n                    ADM_MODS_INFO_WEBSITE: <a href="${website}" style="color:#E6EBFB;" target=_blank>${website}</a>\n                `;
}

function legacyYesterday() {
  const date = new Date(Date.now() - 24 * 60 * 60 * 1000);
  const day = String(date.getDate()).padStart(2, "0");
  const month = String(date.getMonth() + 1).padStart(2, "0");
  return `${day}.${month}.${date.getFullYear()}`;
}

function adminModeHref(mode: string) {
  const query = new URLSearchParams(window.location.search);
  query.set("mode", mode);
  return gameRouteURL("/game/admin", `?${query.toString()}`);
}

function adminModeActionHref(mode: string, action: string) {
  const query = new URLSearchParams(window.location.search);
  query.set("mode", mode);
  query.set("action", action);
  return gameRouteURL("/game/admin", `?${query.toString()}`);
}

function adminModeFileActionHref(mode: string, action: string, fileName: string) {
  const query = new URLSearchParams(window.location.search);
  query.set("mode", mode);
  query.set("action", action);
  query.set("fname", fileName);
  return gameRouteURL("/game/admin", `?${query.toString()}`);
}

function adminUserHref(playerID: number) {
  const query = new URLSearchParams(window.location.search);
  query.set("mode", "Users");
  query.set("player_id", String(playerID));
  return gameRouteURL("/game/admin", `?${query.toString()}`);
}

function adminPlanetHref(planetID: number) {
  const query = new URLSearchParams(window.location.search);
  query.set("mode", "Planets");
  query.set("cp", String(planetID));
  return gameRouteURL("/game/admin", `?${query.toString()}`);
}

function adminGalaxyHref(coordinates: Coordinates) {
  const query = new URLSearchParams(window.location.search);
  query.set("galaxy", String(coordinates.galaxy));
  query.set("system", String(coordinates.system));
  return gameRouteURL("/game/galaxy", `?${query.toString()}`);
}

function adminHackCheckHref() {
  const query = new URLSearchParams(window.location.search);
  query.set("mode", "Debug");
  query.set("filter", "HACKING");
  return gameRouteURL("/game/admin", `?${query.toString()}`);
}

function adminModeModActionHref(mode: string, action: string, modname: string) {
  const query = new URLSearchParams(window.location.search);
  query.set("mode", mode);
  query.set("action", action);
  query.set("modname", modname);
  return gameRouteURL("/game/admin", `?${query.toString()}`);
}

function adminHomeHref() {
  const query = new URLSearchParams(window.location.search);
  query.delete("mode");
  return gameRouteURL("/game/admin", `?${query.toString()}`);
}

function AllianceTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  if (alliance.view === "info" && alliance.target) {
    return <AllianceInfoTable alliance={alliance} />;
  }
  if (alliance.pending && alliance.target) {
    return <AlliancePendingTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  if (alliance.view === "create") {
    return <AllianceCreateTable onAction={onAction} pending={pending} />;
  }
  if (alliance.view === "search") {
    return <AllianceSearchTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  if (alliance.view === "apply" && alliance.target) {
    return <AllianceApplyTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  if (alliance.view === "applications" && alliance.own) {
    return <AllianceApplicationsTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  if (alliance.view === "members" && alliance.own) {
    return <AllianceMembersTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  if (alliance.view === "circular" && alliance.own) {
    return <AllianceCircularTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  if (alliance.view === "ranks" && alliance.own) {
    return <AllianceRanksTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  if (alliance.view === "management" && alliance.own) {
    return <AllianceManagementTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  if (alliance.own) {
    return <AllianceHomeTable alliance={alliance} onAction={onAction} pending={pending} />;
  }
  return <AllianceNoAllianceTable />;
}

function AllianceInfoTable({ alliance }: { alliance: GameAlliance }) {
  const target = alliance.target;
  if (!target) {
    return null;
  }
  return (
    <LegacyCenter>
      {target.imageLogo ? <img alt="" className="reloadimage" src="/game/img/preload.gif" title={`pic.php?url=${encodeURIComponent(target.imageLogo)}`} /> : null}
      <table width={519}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              Alliance Information
            </td>
          </tr>
          <tr>
            <th>Tag</th>
            <th>{target.tag}</th>
          </tr>
          <tr>
            <th>Name</th>
            <th>{target.name}</th>
          </tr>
          <tr>
            <th>Member</th>
            <th>{target.memberCount}</th>
          </tr>
          <tr>
            <th colSpan={2} style={{ height: 100 }}>
              {target.externalText}
            </th>
          </tr>
          <tr>
            <th>Homepage</th>
            <th>{target.homepage ? <a href={`redir.php?url=${encodeURIComponent(target.homepage)}`}>{target.homepage}</a> : ""}</th>
          </tr>
        </tbody>
      </table>
    </LegacyCenter>
  );
}

function AllianceNoAllianceTable() {
  return (
    <LegacyCenter>
      <table className="legacy-alliance-menu-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              Alliance
            </td>
          </tr>
          <tr>
            <th>
              <a href={allianceURL({ a: "1" })}>Start your own alliance</a>
            </th>
            <th>
              <a href={allianceURL({ a: "2" })}>Search for alliances</a>
            </th>
          </tr>
        </tbody>
      </table>
      <br />
      <br />
      <br />
      <br />
      <br />
    </LegacyCenter>
  );
}

function AllianceCreateTable({ onAction, pending }: { onAction: (action: GameAllianceAction) => void; pending: boolean }) {
  return (
    <LegacyCenter>
      <form
        action={allianceURL({ a: "1", weiter: "1" })}
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          if (pending) {
            return;
          }
          const data = new FormData(event.currentTarget);
          onAction({ action: "create", tag: String(data.get("tag") ?? ""), name: String(data.get("name") ?? "") });
        }}
      >
        <table className="legacy-alliance-create-table" width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c" colSpan={2}>
                Found an alliance
              </td>
            </tr>
            <tr>
              <th>Alliance abbreviation (3-8 characters)</th>
              <th>
                <input maxLength={8} name="tag" size={8} type="text" />
              </th>
            </tr>
            <tr>
              <th>Alliance name (3-30 characters)</th>
              <th>
                <input maxLength={30} name="name" size={20} type="text" />
              </th>
            </tr>
            <tr>
              <th colSpan={2}>
                <input disabled={pending} type="submit" value="Found" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      <br />
      <br />
      <br />
      <br />
    </LegacyCenter>
  );
}

function AllianceSearchTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  return (
    <LegacyCenter>
      <table className="legacy-alliance-search-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              Looking for alliances.
            </td>
          </tr>
          <tr>
            <th>Seek</th>
            <th>
              <form
                action={allianceURL({ a: "2" })}
                method="post"
                onSubmit={(event) => {
                  event.preventDefault();
                  if (pending) {
                    return;
                  }
                  const data = new FormData(event.currentTarget);
                  onAction({ action: "search", text: String(data.get("suchtext") ?? "") });
                }}
              >
                <input defaultValue={alliance.searchText} name="suchtext" type="text" />
                <input disabled={pending} type="submit" value="Search" />
              </form>
            </th>
          </tr>
        </tbody>
      </table>
      <br />
      {alliance.searchResults.length > 0 ? (
        <table className="legacy-alliance-search-results-table" width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c" colSpan={3}>
                Alliance Search Results
              </td>
            </tr>
            <tr>
              <th>
                <center>Alliance abbreviation</center>
              </th>
              <th>
                <center>Alliance name</center>
              </th>
              <th>
                <center>Number of members</center>
              </th>
            </tr>
            {alliance.searchResults.map((row) => (
              <tr key={row.id}>
                <th>
                  <center>
                    [
                    <a href={allianceURL({ page: "bewerben", allyid: String(row.id) })}>{row.tag}</a>
                    ]
                  </center>
                </th>
                <th>
                  <center>{row.name}</center>
                </th>
                <th>
                  <center>{row.memberCount}</center>
                </th>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      <br />
      <br />
      <br />
    </LegacyCenter>
  );
}

function AllianceApplyTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  const target = alliance.target;
  if (!target) {
    return null;
  }
  if (!target.open) {
    return (
      <LegacyCenter>
        <h1>Register</h1>
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c">It is not possible to apply to alliance [{target.tag}]</td>
            </tr>
            <tr>
              <th>This alliance is not accepting new members at this time</th>
            </tr>
            <tr>
              <th>
                <a href={allianceURL()}>Back</a>
              </th>
            </tr>
          </tbody>
        </table>
        <br />
        <br />
        <br />
        <br />
      </LegacyCenter>
    );
  }
  return (
    <LegacyCenter>
      <h1>Register</h1>
      <form
        action={allianceURL({ page: "bewerben", allyid: String(target.id) })}
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          if (pending) {
            return;
          }
          const data = new FormData(event.currentTarget);
          onAction({ action: "apply", allianceId: target.id, text: String(data.get("text") ?? "") });
        }}
      >
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c" colSpan={2}>
                Alliance application [{target.tag}] write
              </td>
            </tr>
            <tr>
              <th>
                Message (<span id="cntChars">0</span> / 6000 characters)
              </th>
              <th>
                <textarea cols={40} defaultValue={target.insertApp ? target.applicationText : ""} name="text" rows={10} />
              </th>
            </tr>
            <tr>
              <th>A little help</th>
              <th>
                <input type="submit" value="Sample" />
              </th>
            </tr>
            <tr>
              <th colSpan={2}>
                <input disabled={pending} name="weiter" type="submit" value="Submit" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      <br />
      <br />
      <br />
      <br />
    </LegacyCenter>
  );
}

function AlliancePendingTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  const pendingApp = alliance.pending;
  const target = alliance.target;
  if (!pendingApp || !target) {
    return null;
  }
  return (
    <LegacyCenter>
      <form
        action={allianceURL()}
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          if (!pending) {
            onAction({ action: "withdraw", applicationId: pendingApp.id });
          }
        }}
      >
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c" colSpan={2}>
                Your statement
              </td>
            </tr>
            <tr>
              <th colSpan={2}>You have already applied to alliance [{target.tag}]. Wait for a response or withdraw your application.</th>
            </tr>
            <tr>
              <th colSpan={2}>
                <input disabled={pending} name="bcancel" type="submit" value="Withdraw application" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      <br />
      <br />
      <br />
      <br />
    </LegacyCenter>
  );
}

function AllianceHomeTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  const own = alliance.own;
  if (!own) {
    return null;
  }
  return (
    <LegacyCenter>
      {own.imageLogo ? <img alt="" className="reloadimage" src="/game/img/preload.gif" title={`pic.php?url=${encodeURIComponent(own.imageLogo)}`} /> : null}
      <table className="legacy-alliance-owned-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              Your alliance
            </td>
          </tr>
          <tr>
            <th>Abbreviation</th>
            <th>
              {own.tag}
              {own.oldTag && own.tagUntil > Math.floor(Date.now() / 1000) ? ` (former ${own.oldTag})` : ""}
            </th>
          </tr>
          <tr>
            <th>Name</th>
            <th>
              {own.name}
              {own.oldName && own.nameUntil > Math.floor(Date.now() / 1000) ? ` (former ${own.oldName})` : ""}
            </th>
          </tr>
          <tr>
            <th>Members</th>
            <th>
              {own.memberCount}
              {alliance.viewer.rankRights & 0x008 || alliance.viewer.founder ? (
                <>
                  {" ("}
                  <a href={allianceURL({ a: "4" })}>members list</a>
                  {")"}
                </>
              ) : null}
            </th>
          </tr>
          <tr>
            <th>your rank</th>
            <th>
              {alliance.viewer.rankName}
              {alliance.viewer.rankRights & 0x020 || alliance.viewer.founder ? (
                <>
                  {" ("}
                  <a href={allianceURL({ a: "5" })}>alliance management</a>
                  {")"}
                </>
              ) : null}
            </th>
          </tr>
          {own.applicationCount > 0 ? (
            <tr
              dangerouslySetInnerHTML={{
                __html: `<th>Applications</th><th><a href="${legacyHTMLAttribute(allianceURL({ page: "bewerbungen" }))}">${own.applicationCount} Application(s)</a></th>`
              }}
            />
          ) : null}
          {alliance.viewer.rankRights & 0x080 || alliance.viewer.founder ? (
            <tr>
              <th>General Message</th>
              <th>
                <a href={allianceURL({ a: "17" })}>Send General Message</a>
              </th>
            </tr>
          ) : null}
          <tr>
            <th colSpan={2} style={{ height: 100 }}>
              {own.externalText}
            </th>
          </tr>
          <tr>
            <th>Homepage</th>
            <th>{own.homepage ? <a href={`redir.php?url=${encodeURIComponent(own.homepage)}`}>{own.homepage}</a> : ""}</th>
          </tr>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              Internal Competency
            </td>
          </tr>
          <tr>
            <th colSpan={2} style={{ height: 100 }}>
              {own.internalText}
            </th>
          </tr>
        </tbody>
      </table>
      <br />
      {!alliance.viewer.founder ? (
        <form
          action={allianceURL({ a: "3" })}
          method="post"
          onSubmit={(event) => {
            event.preventDefault();
            if (!pending) {
              onAction({ action: "leave" });
            }
          }}
        >
          <table width={519}>
            <tbody>
              <tr>
                <td className="legacy-c c" colSpan={2}>
                  Leave this alliance
                </td>
              </tr>
              <tr>
                <th colSpan={2}>
                  <input disabled={pending} type="submit" value="Yes!" />
                </th>
              </tr>
            </tbody>
          </table>
        </form>
      ) : null}
    </LegacyCenter>
  );
}

function AllianceCircularTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  if (!alliance.own) {
    return null;
  }
  if (alliance.circularResult) {
    const recipients = alliance.circularResult.recipients;
    const resultAction = recipients.length > 0 ? allianceURL() : allianceURL({ a: "17" });
    const body =
      recipients.length > 0
        ? `<tr><td class=c>The following players have received your general message</td></tr><tr><th>\n${recipients
            .map((name) => `${legacyHTMLText(name)}<br>`)
            .join("\n")}</th></tr><tr><th><input type=submit value="Ok"></th></tr>`
        : `<tr><td class=c>Error</td></tr><tr><th>Sorry, no recipients were found.</th></tr><tr><th><input type=submit value="Back"></th></tr>`;
    return React.createElement("center", {
      dangerouslySetInnerHTML: {
        __html: `<script src="/public-assets/game/js/cntchar.js" type="text/javascript"></script><script src="/public-assets/game/js/win.js" type="text/javascript"></script>
<table class="legacy-alliance-circular-table" width=519>
<form action="${legacyHTMLAttribute(resultAction)}" method=POST>
${body}
</table></center></form>`
      }
    });
  }
  const rankOptions = alliance.ranks
    .filter((rank) => rank.id !== 0 && rank.id !== 1)
    .map((rank) => `    <option value=${rank.id}>Only to a specific rank: ${legacyHTMLText(rank.name)}</option>`)
    .join("\n");
  const options = `    <option value=0>All players</option>${rankOptions ? `\n${rankOptions}` : ""}`;
  const action = allianceURL({ a: "17", sendmail: "1" });
  return React.createElement("center", {
    dangerouslySetInnerHTML: {
      __html: `<script src="/public-assets/game/js/cntchar.js" type="text/javascript"></script><script src="/public-assets/game/js/win.js" type="text/javascript"></script>
<table class="legacy-alliance-circular-table" width=519>
<form action="${legacyHTMLAttribute(action)}" method=POST data-alliance-action="send_circular">
<tr><td class=c colspan=2>Send general message</td></tr>
<tr><th>Recipient</th><th>
<select name=r>
${options}
</select></th></tr>
<tr><th>Message text (<span id="cntChars">0</span> / 2000 Simv.)</th><th><textarea name=text cols=60 rows=10 onkeyup="javascript:cntchar(2000)"></textarea></th></tr>
<tr><th colspan=2><input type=submit value="Submit"></th></tr></table></form>`
    },
    onSubmit: (event: React.FormEvent<HTMLElement>) => {
      const form = event.target instanceof HTMLFormElement ? event.target : null;
      if (!form || form.dataset.allianceAction !== "send_circular") {
        return;
      }
      event.preventDefault();
      if (pending) {
        return;
      }
      const data = new FormData(form);
      onAction({
        action: "send_circular",
        circularRankId: Number(data.get("r") ?? 0) || 0,
        text: String(data.get("text") ?? "")
      });
    }
  });
}

function AllianceRanksTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  if (!alliance.own) {
    return null;
  }
  if (!alliance.viewer.founder && (alliance.viewer.rankRights & 0x020) === 0) {
    return (
      <LegacyCenter>
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c">View not possible</td>
            </tr>
            <tr>
              <th>Not enough permissions to perform the operation</th>
            </tr>
          </tbody>
        </table>
      </LegacyCenter>
    );
  }
  return React.createElement("center", {
    dangerouslySetInnerHTML: { __html: allianceRanksHTML(alliance) },
    onSubmit: (event: React.FormEvent<HTMLElement>) => {
      const form = event.target instanceof HTMLFormElement ? event.target : null;
      const action = form?.dataset.allianceAction ?? "";
      if (!form || action === "") {
        return;
      }
      event.preventDefault();
      if (pending) {
        return;
      }
      const data = new FormData(form);
      if (action === "add_rank") {
        onAction({ action: "add_rank", rankName: String(data.get("newrangname") ?? "") });
        return;
      }
      if (action === "save_ranks") {
        onAction({ action: "save_ranks", rankRights: allianceRankRightsFromForm(alliance.ranks, data) });
      }
    },
    onClick: (event: React.MouseEvent<HTMLElement>) => {
      const link = (event.target as HTMLElement).closest("a[data-alliance-action='delete_rank']");
      if (!link || !(link instanceof HTMLAnchorElement)) {
        return;
      }
      event.preventDefault();
      if (pending) {
        return;
      }
      onAction({ action: "delete_rank", rankId: Number(link.dataset.rankId ?? 0) || 0 });
    }
  });
}

function allianceRanksHTML(alliance: GameAlliance): string {
  const rights = [
    "Dissolve alliance",
    "Expel player",
    "View statements",
    "View member list",
    "Edit statements",
    "Manage Alliance",
    'View status "online" in the member list',
    "Compose a general message",
    "'Right Hand' (required to transfer founder status)"
  ];
  const rightImages = Array.from({ length: 9 }, (_, index) => `<th>\n   <img src=/public-assets/game/img/r${index + 1}.png>\n  </th>`).join("\n  ");
  const rows = alliance.ranks
    .filter((rank) => rank.id !== 0 && rank.id !== 1)
    .map((rank) => {
      const boxes = Array.from({ length: 9 }, (_, index) => {
        const checked = rank.rights & (1 << index) ? " checked" : "";
        return `<th><input type=checkbox name="u${rank.id}r${index}"${checked}></th>`;
      }).join("");
      return ` <tr>\n  <th><a href="${legacyHTMLAttribute(allianceURL({ a: "15", d: String(rank.id) }))}" data-alliance-action="delete_rank" data-rank-id="${rank.id}"><img src="${skinBase}/pic/abort.gif" alt="Delete rank" border="0"></a></th>\n  <th>&nbsp;${legacyHTMLText(rank.name)}&nbsp;</th>\n${boxes}\n </tr>\n`;
    })
    .join("");
  const infoRows = rights
    .map((text, index) => `<tr><th><img src=/public-assets/game/img/r${index + 1}.png></th><th>${legacyHTMLText(text)}</th></tr>`)
    .join("\n");
  return `<script src="/public-assets/game/js/cntchar.js" type="text/javascript"></script><script src="/public-assets/game/js/win.js" type="text/javascript"></script><br />
<a href="${legacyHTMLAttribute(allianceURL({ a: "5" }))}">Back to review</a>
<table class="legacy-alliance-ranks-table" width="519">
 <tr>
  <td class="c" colspan="11">Form rights</td>
 </tr>
 <form action="${legacyHTMLAttribute(allianceURL({ a: "15" }))}" method="POST" data-alliance-action="save_ranks">
 <tr>
  <th></th>
  <th>Rank name</th>
  ${rightImages}
 </tr>
${rows} <tr>
  <th colspan="11"><input type="submit" value="Save"></th>
 </tr>
</form>
</table>
<br /><form action="${legacyHTMLAttribute(allianceURL({ a: "15" }))}" method=POST data-alliance-action="add_rank">
<table width=519>
<tr><td class=c colspan=2>Assign new rank</td></tr>
<tr><th>Rank name</th><th><input type=text name="newrangname" size=20 maxlength=30></th></tr>
<tr><th colspan=2><input type=submit value="Assign"></th></tr>
</form></table>

<br/><form action="${legacyHTMLAttribute(allianceURL({ a: "15" }))}" method=POST>
<table width=519>
<tr><td class=c colspan=2>Explanation of Rights</td></tr>
${infoRows}
</form></table>`;
}

function allianceRankRightsFromForm(ranks: GameAllianceRank[], data: FormData) {
  return ranks
    .filter((rank) => rank.id !== 0 && rank.id !== 1)
    .map((rank) => {
      let rights = 0;
      for (let index = 0; index < 9; index += 1) {
        if (data.get(`u${rank.id}r${index}`) === "on") {
          rights |= 1 << index;
        }
      }
      return { id: rank.id, rights };
    });
}

function AllianceManagementTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  const own = alliance.own;
  if (!own) {
    return null;
  }
  if (!alliance.viewer.founder && (alliance.viewer.rankRights & 0x020) === 0) {
    return (
      <LegacyCenter>
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c">View not possible</td>
            </tr>
            <tr>
              <th>Not enough permissions to perform the operation</th>
            </tr>
          </tbody>
        </table>
      </LegacyCenter>
    );
  }
  const textKind = alliance.textKind >= 1 && alliance.textKind <= 3 ? alliance.textKind : 1;
  const textValue = textKind === 2 ? own.internalText : textKind === 3 ? own.applicationText : own.externalText;
  const textDescription =
    textKind === 2
      ? "Internal alliance text"
      : textKind === 3
        ? "Sample application text"
        : "External alliance text";
  const founderRank = alliance.ranks.find((rank) => rank.id === 0)?.name ?? alliance.viewer.rankName;
  const founderRankValue = founderRank === "Founder" ? "" : founderRank;
  return (
    <LegacyCenter>
      <script src="/public-assets/game/js/cntchar.js" type="text/javascript" />
      <script src="/public-assets/game/js/win.js" type="text/javascript" />
      <table className="legacy-alliance-management-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              alliance management
            </td>
          </tr>
          <tr>
            <th colSpan={2}>
              <a href={allianceURL({ a: "6" })}>set ranks</a>
            </th>
          </tr>
          <tr>
            <th colSpan={2}>
              <a href={allianceURL({ a: "7" })}>Alliance Members</a>
            </th>
          </tr>
          <tr>
            <th
              colSpan={2}
              dangerouslySetInnerHTML={{
                __html: `<a href="${legacyHTMLAttribute(allianceURL({ a: "9" }))}"><img src="${skinBase}/pic/appwiz.gif" border=0 alt="Change alliance abbreviation (1 time per week only)"></a>&nbsp;
<a href="${legacyHTMLAttribute(allianceURL({ a: "10" }))}"><img src="${skinBase}/pic/appwiz.gif" border=0 alt="Change alliance name (1 time per week only)"></a>`
              }}
            />
          </tr>
        </tbody>
      </table>
      <br />
      <form
        action={allianceURL({ a: "11", d: "1", t: String(textKind) })}
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          if (pending) {
            return;
          }
          const data = new FormData(event.currentTarget);
          onAction({
            action: "save_text",
            textKind,
            text: String(data.get("text") ?? ""),
            insertApp: String(data.get("bewforce") ?? "0") === "1"
          });
        }}
      >
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c" colSpan={3}>
                Edit text
              </td>
            </tr>
            <tr>
              <th>
                <a href={allianceURL({ a: "5", t: "1" })}>External text</a>
              </th>
              <th>
                <a href={allianceURL({ a: "5", t: "2" })}>Internal text</a>
              </th>
              <th>
                <a href={allianceURL({ a: "5", t: "3" })}>Application Text</a>
              </th>
            </tr>
            <tr>
              <td
                className="legacy-c c"
                colSpan={3}
                dangerouslySetInnerHTML={{
                  __html: `${legacyHTMLText(textDescription)} (<span id="cntChars">
${textValue.length}</span> / 5000 characters)`
                }}
              />
            </tr>
            <tr>
              <th colSpan={3}>
                <textarea cols={70} defaultValue={textValue} name="text" rows={15} />
              </th>
            </tr>
            {textKind === 3 ? (
              <tr
                dangerouslySetInnerHTML={{
                  __html: `<th colspan=3>Sample application <select name=bewforce><option value=0${
                    own.insertApp ? "" : " SELECTED"
                  }>do not show automatically</option><option value=1${
                    own.insertApp ? " SELECTED" : ""
                  }>show automatically</option></select></th>`
                }}
              />
            ) : null}
            <tr>
              <th colSpan={3}>
                <input type="reset" value="Delete" /> <input disabled={pending} type="submit" value="Save" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      <br />
      <form
        action={allianceURL({ a: "11", d: "2" })}
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          if (pending) {
            return;
          }
          const data = new FormData(event.currentTarget);
          onAction({
            action: "save_settings",
            homepage: String(data.get("hp") ?? ""),
            imageLogo: String(data.get("logo") ?? ""),
            open: String(data.get("bew") ?? "0") !== "1",
            founderRankName: String(data.get("fname") ?? "")
          });
        }}
      >
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c" colSpan={2}>
                Settings
              </td>
            </tr>
            <tr>
              <th>Homepage</th>
              <th>
                <input defaultValue={own.homepage} name="hp" size={70} type="text" />
              </th>
            </tr>
            <tr>
              <th>Alliance Logo</th>
              <th>
                <input defaultValue={own.imageLogo} name="logo" size={70} type="text" />
              </th>
            </tr>
            <tr>
              <th>Applications</th>
              <th>
                <select defaultValue={own.open ? "0" : "1"} name="bew">
                  <option value="0">Possible (alliance open)</option>
                  <option value="1">Impossible (alliance closed)</option>
                </select>
              </th>
            </tr>
            <tr>
              <th>Chapter Name</th>
              <th>
                <input defaultValue={founderRankValue} name="fname" size={30} type="text" />
              </th>
            </tr>
            <tr>
              <th colSpan={2}>
                <input disabled={pending} type="submit" value="Save" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      <form action={allianceURL({ a: "12" })} method="post">
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c">Dissolve alliance</td>
            </tr>
            <tr>
              <th>
                <input type="submit" value="Next" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      <form action={allianceURL({ a: "18" })} method="post">
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c">Leave/Replace this alliance</td>
            </tr>
            <tr>
              <th>
                <input type="submit" value="Next" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
    </LegacyCenter>
  );
}

function AllianceApplicationsTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  const own = alliance.own;
  if (!own) {
    return null;
  }
  if (alliance.applications.length === 0) {
    return React.createElement("center", {
      dangerouslySetInnerHTML: {
        __html: allianceApplicationsHTML(alliance)
      }
    });
  }
  if (!alliance.selectedApp) {
    return React.createElement("center", {
      dangerouslySetInnerHTML: {
        __html: allianceApplicationsHTML(alliance)
      }
    });
  }
  return (
    <LegacyCenter>
      <table className="legacy-alliance-applications-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              Overview of enrollment in this alliance [{own.tag}].
            </td>
          </tr>
          {alliance.selectedApp ? (
            <>
              <tr>
                <th colSpan={2}>Statement from {alliance.selectedApp.playerName}</th>
              </tr>
              <tr>
                <th colSpan={2}>{alliance.selectedApp.text}</th>
              </tr>
              <tr>
                <td className="legacy-c c" colSpan={2}>
                  Response to this statement
                </td>
              </tr>
              <tr>
                <th> </th>
                <th>
                  <button disabled={pending} onClick={() => onAction({ action: "accept", applicationId: alliance.selectedApp?.id ?? 0 })} type="button">
                    Accept
                  </button>
                </th>
              </tr>
              <tr>
                <th>Reason (optional) 0 / 2000 characters</th>
                <th>
                  <AllianceRejectForm application={alliance.selectedApp} onAction={onAction} pending={pending} />
                </th>
              </tr>
              <tr>
                <td> </td>
              </tr>
            </>
          ) : null}
          <tr>
            <th colSpan={2}>Available {alliance.applications.length} statements. Click on the desired player's name to view their message.</th>
          </tr>
          <tr>
            <td className="legacy-c c">
              <center>
                <a href={allianceURL({ page: "bewerbungen", sort: "1" })}>Applicant</a>
              </center>
            </td>
            <td className="legacy-c c">
              <center>
                <a href={allianceURL({ page: "bewerbungen", sort: "0" })}>Application Date</a>
              </center>
            </td>
          </tr>
          {alliance.applications.map((app) => (
            <tr key={app.id}>
              <th>
                <center>
                  <a href={allianceURL({ page: "bewerbungen", show: String(app.id), sort: "1" })}>{app.playerName}</a>
                </center>
              </th>
              <th>
                <center>{formatLegacyServerDateTime(app.date)}</center>
              </th>
            </tr>
          ))}
        </tbody>
      </table>
      <br />
      <br />
      <br />
      <br />
    </LegacyCenter>
  );
}

function allianceApplicationsHTML(alliance: GameAlliance): string {
  const own = alliance.own;
  if (!own) {
    return "";
  }
  if (alliance.applications.length === 0) {
    return `<table class="legacy-alliance-applications-table" width=519><tr><td class=c colspan=2>Overview of enrollment in this alliance [${legacyHTMLText(own.tag)}].</td></tr><tr><th colspan=2>No more applications.</th></tr></table><br><br><br><br>`;
  }
  const rows = alliance.applications
    .map(
      (app) => `    <th><center><a href="${legacyHTMLAttribute(
        allianceURL({ page: "bewerbungen", show: String(app.id), sort: "1" })
      )}">${legacyHTMLText(app.playerName)}</a></center></th>
    <th><center>${formatLegacyServerDateTime(app.date)}</center></th></tr>
`
    )
    .join("");
  return `<table class="legacy-alliance-applications-table" width=519>
<tr><td class=c colspan=2>Overview of enrollment in this alliance [${legacyHTMLText(own.tag)}].</td></tr>
<tr><th colspan=2>Available ${alliance.applications.length} statements. Click on the desired player's name to view their message.</th></tr>
<tr>
    <td class=c><center><a href="${legacyHTMLAttribute(allianceURL({ page: "bewerbungen", show: "0", sort: "1" }))}">Applicant</a></center></td>
    <td class=c><center><a href="${legacyHTMLAttribute(allianceURL({ page: "bewerbungen", show: "0", sort: "0" }))}">Application Date</a></center></th></tr>
<tr>
${rows}</table><br><br><br><br>`;
}

function AllianceRejectForm({
  application,
  onAction,
  pending
}: {
  application: GameAllianceApplication;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  return (
    <form
      action={allianceURL({ page: "bewerbungen", show: String(application.id), sort: "1" })}
      method="post"
      onSubmit={(event) => {
        event.preventDefault();
        if (pending) {
          return;
        }
        const data = new FormData(event.currentTarget);
        onAction({ action: "reject", applicationId: application.id, text: String(data.get("text") ?? "") });
      }}
    >
      <textarea cols={40} name="text" rows={10} />
      <br />
      <input disabled={pending} name="aktion" type="submit" value="Reject" />
    </form>
  );
}

function AllianceMembersTable({
  alliance,
  onAction,
  pending
}: {
  alliance: GameAlliance;
  onAction: (action: GameAllianceAction) => void;
  pending: boolean;
}) {
  const own = alliance.own;
  if (!own) {
    return null;
  }
  const action = new URLSearchParams(window.location.search).get("a") ?? "4";
  if (action === "7") {
    if (!alliance.viewer.founder && (alliance.viewer.rankRights & 0x020) === 0) {
      return (
        <LegacyCenter>
          <table width={519}>
            <tbody>
              <tr>
                <td className="legacy-c c">View not possible</td>
              </tr>
              <tr>
                <th>Not enough permissions to perform the operation</th>
              </tr>
            </tbody>
          </table>
        </LegacyCenter>
      );
    }
    return React.createElement("center", {
      dangerouslySetInnerHTML: {
        __html: allianceMemberSettingsHTML(alliance)
      },
      onSubmit: (event: React.FormEvent<HTMLElement>) => {
        const form = event.target instanceof HTMLFormElement ? event.target : null;
        if (!form || form.dataset.allianceAction !== "assign_rank") {
          return;
        }
        event.preventDefault();
        if (pending) {
          return;
        }
        const data = new FormData(form);
        onAction({
          action: "assign_rank",
          targetPlayerId: Number(form.dataset.targetPlayerId ?? 0) || 0,
          targetRankId: Number(data.get("newrang") ?? 0) || 0
        });
      },
      onClick: (event: React.MouseEvent<HTMLElement>) => {
        const link = (event.target as HTMLElement).closest("a[data-alliance-action='kick_member']");
        if (!link || !(link instanceof HTMLAnchorElement)) {
          return;
        }
        event.preventDefault();
        if (pending) {
          return;
        }
        const name = link.dataset.playerName ?? "";
        if (name && !window.confirm(`Are you sure that player ${name} should leave the alliance?`)) {
          return;
        }
        onAction({ action: "kick_member", targetPlayerId: Number(link.dataset.targetPlayerId ?? 0) || 0 });
      }
    });
  }
  if (!alliance.viewer.founder && (alliance.viewer.rankRights & 0x008) === 0) {
    return (
      <LegacyCenter>
        <table width={519}>
          <tbody>
            <tr>
              <td className="legacy-c c">View not possible</td>
            </tr>
            <tr>
              <th>Not enough permissions to perform the operation</th>
            </tr>
          </tbody>
        </table>
      </LegacyCenter>
    );
  }
  return React.createElement("center", {
    dangerouslySetInnerHTML: {
      __html: allianceMembersHTML(alliance)
    }
  });
}

function allianceMemberSettingsHTML(alliance: GameAlliance): string {
  const own = alliance.own;
  if (!own) {
    return "";
  }
  const selectedUser = Number(new URLSearchParams(window.location.search).get("u") ?? 0) || 0;
  const sort2 = new URLSearchParams(window.location.search).get("sort2") === "0" ? "0" : "1";
  const nextSort2 = sort2 === "1" ? "0" : "1";
  const now = Math.floor(Date.now() / 1000);
  const rankOptions = (selectedRankID: number) =>
    alliance.ranks
      .filter((rank) => rank.id !== 0)
      .map((rank) => `<option value="${rank.id}"${rank.id === selectedRankID ? " SELECTED" : ""}>${legacyHTMLText(rank.name)}`)
      .join("");
  const rows = alliance.members
    .map((member, index) => {
      const message =
        alliance.viewer.playerId === member.playerId
          ? "<th></th>"
          : `<th><a href="${legacyHTMLAttribute(gameMessageComposeURL(member.playerId, window.location.search))}"><img src="${skinBase}/img/m.gif" border=0 alt="Write message"></a></th>`;
      const coordQuery = new URLSearchParams(window.location.search);
      coordQuery.set("galaxy", String(member.galaxy));
      coordQuery.set("system", String(member.system));
      coordQuery.set("position", String(member.position));
      const coordURL = gameRouteURL("/game/galaxy", coordQuery.toString());
      const days = Math.floor(Math.max(0, now - member.lastClick) / (60 * 60 * 24));
      const actions =
        member.rankId > 0
          ? `<th><a data-alliance-action="kick_member" data-target-player-id="${member.playerId}" data-player-name="${legacyHTMLAttribute(member.name)}" onmouseover='return overlib("<font color=white>Kick player</font>", WIDTH, 100);' onmouseout='return nd();' alt='Kick player' href="${legacyHTMLAttribute(allianceURL({ a: "13", u: String(member.playerId) }))}"><img src='${skinBase}/pic/abort.gif' alt='Kick player' border='0' ></a><a onmouseover="return overlib('<font color=white>Assign rank</font>', WIDTH, 100);" onmouseout='return nd();' alt='Assign rank' href="${legacyHTMLAttribute(allianceURL({ a: "7", u: String(member.playerId) }))}"><img src="${skinBase}/pic/key.gif" alt='Assign rank' border=0></a>&nbsp;&nbsp;&nbsp;&nbsp;</th>`
          : "<th>&nbsp;</th>";
      const assignment =
        member.playerId === selectedUser && member.rankId > 0
          ? `<form action="${legacyHTMLAttribute(allianceURL({ a: "16", u: String(selectedUser) }))}" method=POST data-alliance-action="assign_rank" data-target-player-id="${selectedUser}"><tr><th colspan=3>Assign rank to ${legacyHTMLText(member.name)}</th><th><select name="newrang">${rankOptions(
              member.rankId
            )}</th><th colspan=5><input type=submit value="Save"></th></tr></form>\n`
          : "";
      return `<tr><th>${index + 1}</th><th>${legacyHTMLText(member.name)}</th>${message}<th>${legacyHTMLText(
        member.rankName
      )}</th><th>${formatLegacyPlainNumber(Math.floor(member.score / 1000))}</th><th><a href="${legacyHTMLAttribute(coordURL)}" >[${member.galaxy}:${
        member.system
      }:${member.position}]</a></th><th>${member.joinedAt > 0 ? formatLegacyServerDateTime(member.joinedAt) : "-"}</th><th>${days}d</th>${actions}</tr>\n${assignment}`;
    })
    .join("");
  return `<script src="/public-assets/game/js/cntchar.js" type="text/javascript"></script><script src="/public-assets/game/js/win.js" type="text/javascript"></script><br>
<a href="${legacyHTMLAttribute(allianceURL({ a: "5" }))}">Back to review</a>
<table class="legacy-alliance-member-settings-table" width=519>
<tr><td class='c' colspan='10'>List of members (count: ${own.memberCount})</td></tr>
<tr>
    <th>N</th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "7", sort1: "1", sort2: nextSort2 }))}">Name</a></th>
    <th> </th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "7", sort1: "2", sort2: nextSort2 }))}">Status</a></th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "7", sort1: "3", sort2: nextSort2 }))}">Points</a></th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "7", sort1: "0", sort2: nextSort2 }))}">Coordinates</a></th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "7", sort1: "4", sort2: nextSort2 }))}">Entry</a></th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "7", sort1: "5", sort2: nextSort2 }))}">Inactive days</a></th>
    <th>Action</th></tr>
${rows}</table>`;
}

function allianceMembersHTML(alliance: GameAlliance): string {
  const own = alliance.own;
  if (!own) {
    return "";
  }
  const sort2 = new URLSearchParams(window.location.search).get("sort2") === "0" ? "0" : "1";
  const nextSort2 = sort2 === "1" ? "0" : "1";
  const showOnline = alliance.viewer.founder || (alliance.viewer.rankRights & 0x040) !== 0;
  const now = Math.floor(Date.now() / 1000);
  const headerOnline = showOnline
    ? `    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "4", sort1: "5", sort2: nextSort2 }))}">Online</a></th></tr>\n`
    : "";
  const rows = alliance.members
    .map((member, index) => {
      const message =
        alliance.viewer.playerId === member.playerId
          ? "    <th></th>\n"
          : `    <th><a href="${legacyHTMLAttribute(gameMessageComposeURL(member.playerId, window.location.search))}"><img src="${skinBase}/img/m.gif" border=0 alt="Write message"></a></th>\n`;
      const coordQuery = new URLSearchParams(window.location.search);
      coordQuery.set("galaxy", String(member.galaxy));
      coordQuery.set("system", String(member.system));
      coordQuery.set("position", String(member.position));
      const coordURL = gameRouteURL("/game/galaxy", coordQuery.toString());
      const online = showOnline ? `    <th>${allianceMemberOnlineHTML(member.lastClick, now)}</th>` : "";
      return `<tr>
    <th>${index + 1}</th>
    <th>${legacyHTMLText(member.name)}</th>
${message}    <th>${legacyHTMLText(member.rankName)}</th>
    <th>${formatLegacyPlainNumber(Math.floor(member.score / 1000))}</th>
    <th><a href="${legacyHTMLAttribute(coordURL)}" >[${member.galaxy}:${member.system}:${member.position}]</a></th>
    <th>${member.joinedAt > 0 ? formatLegacyServerDateTime(member.joinedAt) : "-"}</th>
${online}</tr>
`;
    })
    .join("");
  return `<script src="/public-assets/game/js/cntchar.js" type="text/javascript"></script><script src="/public-assets/game/js/win.js" type="text/javascript"></script>
<table class="legacy-alliance-members-table" width=519>
<tr><td class='c' colspan='10'>List of members (count: ${own.memberCount})</td></tr>
<tr>
    <th>N</th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "4", sort1: "1", sort2: nextSort2 }))}">Name</a></th>
    <th> </th><th><a href="${legacyHTMLAttribute(allianceURL({ a: "4", sort1: "2", sort2: nextSort2 }))}">Status</a></th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "4", sort1: "3", sort2: nextSort2 }))}">Points</a></th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "4", sort1: "0", sort2: nextSort2 }))}">Coordinates</a></th>
    <th><a href="${legacyHTMLAttribute(allianceURL({ a: "4", sort1: "4", sort2: nextSort2 }))}">Entry</a></th>
${headerOnline}${rows}</table>`;
}

function allianceMemberOnlineHTML(lastClick: number, now: number): string {
  const minutes = Math.floor((now - lastClick) / 60);
  if (minutes < 15) {
    return "<font color=lime>Yes</font>";
  }
  if (minutes < 60) {
    return `<font color=yellow>${minutes} min</font>`;
  }
  return "<font color=red>No</font>";
}

function allianceURL(params: Record<string, string> = {}) {
  const query = new URLSearchParams(window.location.search);
  for (const key of ["page", "a", "allyid", "show", "sort", "suchtext", "weiter", "sendmail", "d", "t", "u", "sort1", "sort2"]) {
    query.delete(key);
  }
  for (const [key, value] of Object.entries(params)) {
    query.set(key, value);
  }
  return gameRouteURL("/game/alliance", `?${query.toString()}`);
}

function MerchantTable({
  merchant,
  onCall,
  onTrade,
  pending
}: {
  merchant: GameMerchant;
  onCall: (offerID: number) => void;
  onTrade: (values: GameMerchantTradeValues) => void;
  pending: boolean;
}) {
  const activeOfferID = merchant.activeOfferId;
  const [selectedOfferID, setSelectedOfferID] = React.useState(activeOfferID || 1);
  const [values, setValues] = React.useState<Record<number, number>>({ 1: 0, 2: 0, 3: 0 });
  React.useEffect(() => {
    setSelectedOfferID(activeOfferID || 1);
    setValues({ 1: 0, 2: 0, 3: 0 });
  }, [activeOfferID, merchant.rates.metal, merchant.rates.crystal, merchant.rates.deuterium]);
  const activeRow = merchant.rows.find((row) => row.id === activeOfferID);
  const exchangeValues = normalizeMerchantExchangeValues(values, merchant, activeOfferID);
  const offerCost = merchantOfferCost(exchangeValues, merchant, activeOfferID);
  const submitCall = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onCall(selectedOfferID);
  };
  const submitTrade = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onTrade({
      metal: exchangeValues[1] ?? 0,
      crystal: exchangeValues[2] ?? 0,
      deuterium: exchangeValues[3] ?? 0
    });
  };
  const setExchangeValue = (resourceID: number, rawValue: number) => {
    setValues((current) => normalizeMerchantExchangeValues({ ...current, [resourceID]: rawValue }, merchant, activeOfferID, resourceID));
  };
  return (
    <>
      <form action={gameRouteURL("/game/merchant", window.location.search)} method="POST" name="TraderForm" onSubmit={submitCall}>
        <table className="legacy-overview-table legacy-merchant-call-table c" width={520}>
          <tbody>
            <tr>
              <td align="center" className="legacy-c c">
                {activeRow ? `There is a merchant to whom you can sell ${activeRow.name}.` : "Merchant not found!"}
              </td>
            </tr>
            <tr>
              <th align="center" className="legacy-c c">
                <br />
                {"You want to sell "}
                <select
                  name="offer_id"
                  onChange={(event) => setSelectedOfferID(Number.parseInt(event.currentTarget.value, 10) || 1)}
                  style={{ color: "lime" }}
                  value={selectedOfferID}
                >
                  {merchant.rows.map((row) => (
                    <option key={row.id} value={row.id}>
                      {row.name}
                    </option>
                  ))}
                </select>
                {" !"}<br />
                <div id="darkmatter2">Summoning a merchant costs 2500 dark matter.</div>
                <br />
                <br />
                <input disabled={pending} name="call_trader" type="submit" value={activeOfferID > 0 ? "Call another merchant" : "Call merchant"} />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      <br />
      {activeOfferID > 0 ? (
        <form action={gameRouteURL("/game/merchant", window.location.search)} method="POST" name="TraderForm" onSubmit={submitTrade}>
          <table className="legacy-overview-table legacy-merchant-exchange-table" width={520}>
            <tbody>
              <tr>
                <td align="center" className="legacy-c c" colSpan={4}>
                  Exchange
                </td>
              </tr>
              <tr>
                <th />
                <th />
                <th>Free storage space</th>
                <th>Exchange rate</th>
              </tr>
              {merchant.rows.map((row) => (
                <tr key={row.id}>
                  <th align="center" className="legacy-c c" style={{ width: "25%" }}>
                    {row.name}
                  </th>
                  <th align="center" className="legacy-c c" style={{ width: "25%" }}>
                    {row.offered ? (
                      <span id={`${row.id}_value`}>{formatLegacyNumber(offerCost)}</span>
                    ) : (
                      <>
                        <input
                          name={`${row.id}_value`}
                          onChange={(event) => setExchangeValue(row.id, legacyInputNumber(event.currentTarget.value))}
                          onKeyUp={(event) => setExchangeValue(row.id, legacyInputNumber(event.currentTarget.value))}
                          size={9}
                          style={{ textAlign: "right" }}
                          type="text"
                          value={formatLegacyNumber(exchangeValues[row.id] ?? 0)}
                        />{" "}
                        <a
                          href="#"
                          onClick={(event) => {
                            event.preventDefault();
                            setExchangeValue(row.id, 99999999999999);
                          }}
                        >
                          max
                        </a>
                      </>
                    )}
                  </th>
                  <th align="center" className="legacy-c c" style={{ width: "25%" }}>
                    {row.offered ? "---" : <span id={`${row.id}_storage`}>{formatLegacyNumber(Math.max(0, row.freeStorage - (exchangeValues[row.id] ?? 0)))}</span>}
                  </th>
                  <th align="center" className="legacy-c c" style={{ width: "25%" }}>
                    {row.offered ? (
                      <MerchantRateText rate={row.rate} />
                    ) : (
                      <a href="#" title={merchantExchangeTitle(merchant, activeOfferID, row)}>
                        <MerchantRateText rate={row.rate} />
                      </a>
                    )}
                  </th>
                </tr>
              ))}
              <tr>
                <th align="center" className="legacy-c c" colSpan={4}>
                  <br />
                  The merchant supplies as much as your storage units can hold.
                  <br />
                  <br />
                  <input disabled={pending} name="trade" type="submit" value="Exchange!" />
                </th>
              </tr>
            </tbody>
          </table>
        </form>
      ) : null}
      <br />
      <br />
      <br />
      <br />
    </>
  );
}

function normalizeMerchantExchangeValues(
  values: Record<number, number>,
  merchant: GameMerchant,
  activeOfferID: number,
  changedID?: number
): Record<number, number> {
  if (activeOfferID <= 0) {
    return values;
  }
  const normalized: Record<number, number> = { 1: 0, 2: 0, 3: 0 };
  for (const row of merchant.rows) {
    if (row.id === activeOfferID) {
      normalized[row.id] = 0;
      continue;
    }
    normalized[row.id] = clampNumber(Math.abs(Math.floor(values[row.id] ?? 0)), 0, row.freeStorage);
  }
  const activeRow = merchant.rows.find((row) => row.id === activeOfferID);
  const changedRow = changedID ? merchant.rows.find((row) => row.id === changedID) : undefined;
  if (!activeRow || !changedRow || changedRow.id === activeOfferID) {
    return normalized;
  }
  const otherCost = merchant.rows
    .filter((row) => row.id !== activeOfferID && row.id !== changedRow.id)
    .reduce((sum, row) => sum + Math.floor((normalized[row.id] ?? 0) * merchantRate(merchant, activeOfferID) / Math.max(merchantRate(merchant, row.id), 0.000001)), 0);
  const freeOffer = Math.max(0, activeRow.value - otherCost);
  const changedCost = Math.floor((normalized[changedRow.id] ?? 0) * merchantRate(merchant, activeOfferID) / Math.max(merchantRate(merchant, changedRow.id), 0.000001));
  if (changedCost > freeOffer) {
    normalized[changedRow.id] = Math.max(0, Math.round(freeOffer / Math.max(merchantRate(merchant, activeOfferID), 0.000001) * merchantRate(merchant, changedRow.id)));
  }
  return normalized;
}

function MerchantRateText({ rate }: { rate: number }) {
  return React.createElement(
    "font",
    { size: 3 } as React.HTMLAttributes<HTMLElement> & { size: number },
    React.createElement("b", null, formatMerchantRate(rate))
  );
}

function merchantOfferCost(values: Record<number, number>, merchant: GameMerchant, activeOfferID: number): number {
  if (activeOfferID <= 0) {
    return 0;
  }
  return merchant.rows
    .filter((row) => row.id !== activeOfferID)
    .reduce((sum, row) => sum + Math.floor((values[row.id] ?? 0) * merchantRate(merchant, activeOfferID) / Math.max(merchantRate(merchant, row.id), 0.000001)), 0);
}

function merchantRate(merchant: GameMerchant, resourceID: number): number {
  if (resourceID === 1) {
    return merchant.rates.metal;
  }
  if (resourceID === 2) {
    return merchant.rates.crystal;
  }
  if (resourceID === 3) {
    return merchant.rates.deuterium;
  }
  return 0;
}

function merchantExchangeTitle(merchant: GameMerchant, activeOfferID: number, row: GameMerchantResourceRow): string {
  const activeRow = merchant.rows.find((candidate) => candidate.id === activeOfferID);
  if (!activeRow) {
    return "";
  }
  const ratio = merchantRate(merchant, row.id) / Math.max(merchantRate(merchant, activeOfferID), 0.000001);
  return `One ${activeRow.name} gives ${formatMerchantRate(Math.round(ratio * 100) / 100)} ${row.name}`;
}

function formatMerchantRate(value: number): string {
  if (!Number.isFinite(value)) {
    return "0";
  }
  return value.toFixed(2).replace(/\.?0+$/, "");
}

function legacyInputNumber(value: string): number {
  const parsed = Number.parseInt(value.replaceAll(".", "").trim(), 10);
  return Number.isFinite(parsed) ? parsed : 0;
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
      <ResearchFrame>
        <table className="legacy-overview-table legacy-research-table" width={530}>
          <tbody>
            <tr>
              <td className="legacy-l l" colSpan={2}>
                Description
              </td>
              <td className="legacy-l l">
                <b>Qty.</b>
              </td>
            </tr>
          </tbody>
        </table>
        <table>
          <tbody>
            <tr>
              <td className="legacy-c c">In order to do this, you need to build a research lab!</td>
            </tr>
          </tbody>
        </table>
      </ResearchFrame>
    );
  }
  return (
    <ResearchFrame>
      <table className="legacy-overview-table legacy-research-table" width={530}>
        <tbody>
          <tr>
            <td className="legacy-l l" colSpan={2}>
              Description
            </td>
            <td className="legacy-l l">
              <b>Qty.</b>
            </td>
          </tr>
          {research.items.map((item) => {
            const active = research.active?.techId === item.id ? research.active : undefined;
            const action = item.action === "Cancel" ? "cancel" : "start";
            return (
              <tr data-research-row={item.id} key={item.id}>
                <td className="legacy-l l legacy-building-image">
                  <a href={technologyInfoURL(item.id)}>
                    <img alt="" height={120} src={`${skinBase}/gebaeude/${item.id}.gif`} width={120} />
                  </a>
                </td>
                <td
                  className="legacy-l l legacy-building-description"
                  dangerouslySetInnerHTML={{ __html: buildingDescriptionHTML(item) }}
                />
                <td className="legacy-k k legacy-building-action">
                  {active ? (
                    <ResearchQueueCountdown active={active} onCancel={onAction} pending={pending} />
                  ) : item.action === "-" ? (
                    <> - </>
                  ) : item.canBuild ? (
                    <a
                      href={researchActionURL(action, item.id)}
                      onClick={(event) => {
                        event.preventDefault();
                        if (!pending) {
                          onAction(action, item.id);
                        }
                      }}
                    >
                      <span style={{ color: "#00FF00" }}>
                        <ResearchActionLabel item={item} />
                      </span>
                    </a>
                  ) : (
                    <span style={{ color: "#FF0000" }}>
                      <ResearchActionLabel item={item} />
                    </span>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </ResearchFrame>
  );
}

function ResearchFrame({ children }: { children: React.ReactNode }) {
  return (
    <table className="legacy-research-frame">
      <tbody>
        <tr>
          <td style={{ backgroundColor: "transparent" }}>{children}</td>
        </tr>
      </tbody>
    </table>
  );
}

function ResearchQueueCountdown({
  active,
  onCancel,
  pending
}: {
  active: GameResearchQueue;
  onCancel: (action: "start" | "cancel", techID: number) => void;
  pending: boolean;
}) {
  const [now, setNow] = React.useState(() => Math.floor(Date.now() / 1000));
  React.useEffect(() => {
    const id = window.setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => window.clearInterval(id);
  }, []);
  const remaining = Math.max(0, active.end - now);
  if (remaining <= 0) {
    return (
      <div className="z" id="bxx">
        Done
        <br />
        <a href={researchNextURL(active.planetId)}>next</a>
      </div>
    );
  }
  return (
    <div className="z" id="bxx">
      {formatLegacyCountdown(remaining)}
      <br />
      {active.cancelable ? (
        <a
          href={researchActionURL("cancel", active.techId, active.planetId)}
          onClick={(event) => {
            event.preventDefault();
            if (!pending) {
              onCancel("cancel", active.techId);
            }
          }}
        >
          Cancel
        </a>
      ) : null}
    </div>
  );
}

function ResearchActionLabel({ item }: { item: GameBuildingItem }) {
  if (item.action === "Research level") {
    return (
      <>
        Research
        <br />
        level {item.nextLevel}
      </>
    );
  }
  if (item.action === "research") {
    return <>Research</>;
  }
  return <>{item.action}</>;
}

function researchActionURL(action: "start" | "cancel", techID: number, planetID?: number) {
  const query = new URLSearchParams(window.location.search);
  query.delete("bau");
  query.delete("unbau");
  if (action === "start") {
    query.set("bau", String(techID));
  } else {
    query.set("unbau", String(techID));
    if (planetID !== undefined) {
      query.set("cp", String(planetID));
    }
  }
  return gameRouteURL("/game/research", `?${query.toString()}`);
}

function researchNextURL(planetID: number) {
  const query = new URLSearchParams(window.location.search);
  query.delete("bau");
  query.delete("unbau");
  query.set("cp", String(planetID));
  return gameRouteURL("/game/research", query.toString());
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
  onComplete,
  onSubmit,
  pending,
  shipyard
}: {
  onComplete: () => void;
  onSubmit: (orders: Record<string, number>) => void;
  pending: boolean;
  shipyard: GameShipyard;
}) {
  if (!shipyard.hasShipyard) {
    return (
      <>
        <table className="legacy-overview-table legacy-shipyard-table" width={530}>
          <tbody>
            <tr>
              <td className="legacy-l l" colSpan={2}>
                Description
              </td>
              <td className="legacy-l l">
                <b>Qty.</b>
              </td>
            </tr>
          </tbody>
        </table>
        <div className="legacy-shipyard-required-notice">
          <table>
            <tbody>
              <tr>
                <td className="legacy-c c">In order to do that, you need to build a shipyard!</td>
              </tr>
            </tbody>
          </table>
        </div>
      </>
    );
  }
  return (
    <>
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
            <td className="legacy-l l" colSpan={2}>
              Description
            </td>
            <td className="legacy-l l">
              <b>Qty.</b>
            </td>
          </tr>
          {shipyard.items.map((item) => (
            <tr data-shipyard-row={item.id} key={item.id}>
              <td className="legacy-l l legacy-building-image">
                <a href={technologyInfoURL(item.id)}>
                  <img alt="" height={120} src={`${skinBase}/gebaeude/${item.id}.gif`} width={120} />
                </a>
              </td>
              <td
                className="legacy-l l legacy-building-description"
                dangerouslySetInnerHTML={{ __html: shipyardDescriptionHTML(item) }}
              />
              <td className="legacy-k k legacy-building-action">
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
                    {shipyard.commanderActive && item.maxBuild > 0 ? (
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
            <td align="center" className="legacy-c c" colSpan={2}>
              <input disabled={pending} type="submit" value="Build" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
    <ShipyardQueuePanel onComplete={onComplete} queue={shipyard.queue} />
    </>
  );
}

function shipyardDescriptionHTML(item: GameShipyardItem): string {
  const href = legacyHTMLAttribute(technologyInfoURL(item.id));
  const stock = item.count > 0 ? ` (in stock ${formatLegacyRawInteger(item.count)})` : "";
  const costs = costParts(item.cost)
    .map((part) => ` ${legacyHTMLText(part.name)}: <b>${formatLegacyPlainNumber(part.value)}</b>`)
    .join("");
  return `<a href="${href}">${legacyHTMLText(item.name)}</a>${stock}<br>${legacyHTMLText(item.description)}<br>Cost:${costs}<br>Duration: ${formatLegacyDuration(item.durationSeconds)}<br>`;
}

type ShipyardQueueRuntime = {
  activeIndex: number;
  displayRemaining: number;
  g: number;
  timerStartMs: number;
  entries: ShipyardQueueRuntimeEntry[];
};

type ShipyardQueueRuntimeEntry = GameShipyardQueueEntry & {
  unitSeconds: number;
};

function ShipyardQueuePanel({ onComplete: _onComplete, queue }: { onComplete: () => void; queue: GameShipyardQueueEntry[] }) {
  const queueKey = queue.map((entry) => `${entry.taskId}:${entry.end}:${entry.count}`).join("|");
  const [runtime, setRuntime] = React.useState<ShipyardQueueRuntime>(() => createShipyardQueueRuntime(queue));
  React.useEffect(() => {
    setRuntime(createShipyardQueueRuntime(queue));
  }, [queueKey]);
  React.useEffect(() => {
    const id = window.setInterval(() => setRuntime((current) => advanceShipyardQueueRuntime(current, Date.now())), 200);
    return () => window.clearInterval(id);
  }, []);
  const active = runtime.entries[runtime.activeIndex];
  if (queue.length === 0 || runtime.entries.length === 0) {
    return null;
  }
  const totalRemaining = queue.reduce((sum, entry, index) => {
    const unitSeconds = Math.max(0, entry.end - entry.start);
    if (index === 0) {
      return sum + Math.max(0, entry.remainingSeconds) + Math.max(0, entry.count - 1) * unitSeconds;
    }
    return sum + Math.max(0, entry.count) * unitSeconds;
  }, 0);
  const queueComplete = runtime.activeIndex >= runtime.entries.length;
  return (
    <center className="legacy-shipyard-queue-panel">
      <br />
      Now being produced:{" "}
      <div
        className="z"
        dangerouslySetInnerHTML={{ __html: queueComplete || !active ? "Tasks completed" : shipyardQueueActiveHTML(active, runtime.displayRemaining) }}
        id="bx"
      />
      <br />
      <form action={gameRouteURL("/game/shipyard", window.location.search)} method="get" name="Atr">
        <input name="session" type="hidden" value={new URLSearchParams(window.location.search).get("session") ?? ""} />
        <input name="mode" type="hidden" value="Flotte" />
        <table width={530}>
          <tbody>
            <tr>
              <td className="c">Expected tasks</td>
            </tr>
            <tr>
              <th>
                <select name="auftr" size={10}>
                  {queueComplete ? (
                    <option>Tasks completed</option>
                  ) : (
                    runtime.entries.slice(runtime.activeIndex).map((entry, index) => (
                      <option key={entry.taskId} value={runtime.activeIndex + index + 1}>
                        {entry.count} "{entry.name}"{index === 0 ? " (produced)" : ""}
                      </option>
                    ))
                  )}
                </select>
              </th>
            </tr>
            <tr>
              <td className="c" />
            </tr>
          </tbody>
        </table>
      </form>
      The entire production will take {formatLegacyDuration(totalRemaining)}
      <br />
    </center>
  );
}

function createShipyardQueueRuntime(queue: GameShipyardQueueEntry[]): ShipyardQueueRuntime {
  const entries = queue.map((entry) => ({
    ...entry,
    count: Math.max(0, entry.count),
    unitSeconds: Math.max(0, entry.end - entry.start)
  }));
  const active = entries[0];
  const g = active ? Math.max(0, active.unitSeconds - active.remainingSeconds) : 0;
  return {
    activeIndex: 0,
    displayRemaining: active ? Math.max(0, active.remainingSeconds) : 0,
    entries,
    g,
    timerStartMs: Date.now() - 500
  };
}

function advanceShipyardQueueRuntime(runtime: ShipyardQueueRuntime, nowMs: number): ShipyardQueueRuntime {
  const active = runtime.entries[runtime.activeIndex];
  if (!active) {
    return runtime.displayRemaining === 0 ? runtime : { ...runtime, displayRemaining: 0 };
  }
  const elapsed = Math.round((nowMs - runtime.timerStartMs) / 1000);
  const remaining = active.unitSeconds - runtime.g - elapsed;
  if (remaining >= 0) {
    return remaining === runtime.displayRemaining ? runtime : { ...runtime, displayRemaining: remaining };
  }
  const entries = runtime.entries.map((entry, index) =>
    index === runtime.activeIndex ? { ...entry, count: Math.max(0, entry.count - 1) } : entry
  );
  const activeIndex = entries[runtime.activeIndex]?.count > 0 ? runtime.activeIndex : runtime.activeIndex + 1;
  return {
    activeIndex,
    displayRemaining: 0,
    entries,
    g: 0,
    timerStartMs: nowMs
  };
}

function shipyardQueueActiveHTML(active: GameShipyardQueueEntry, remaining: number): string {
  return `${legacyHTMLText(active.name)} ${formatLegacyCountdown(remaining)}`;
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
  const [dispatchStage, setDispatchStage] = React.useState<"ships" | "target" | "mission">("ships");
  const [unionMission, setUnionMission] = React.useState<GameFleetMission | null>(null);
  const [pendingMissionDraftKey, setPendingMissionDraftKey] = React.useState<string | null>(null);
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
    setDispatchStage("target");
    onPrepare({
      ships: collectLegacyFleetShips(event.currentTarget),
      target: dispatchTarget,
      targetType: dispatchTargetType,
      mission: dispatchMission,
      speed: 10
    });
  };
  const submitFleetCommand = (event: React.FormEvent<HTMLDivElement>) => {
    event.preventDefault();
    const form = event.target;
    if (!(form instanceof HTMLFormElement)) {
      return;
    }
    const formData = new FormData(form);
    const unionID = Number(formData.get("order_union") ?? 0);
    if (unionID > 0) {
      const mission = fleet.missions.find((item) => item.id === unionID);
      if (mission) {
        setUnionMission(mission);
      }
      return;
    }
    const recallID = Number(formData.get("order_return") ?? 0);
    if (recallID > 0) {
      onRecall(recallID);
    }
  };
  const dispatchDraft = fleet.dispatchDraft?.hasSelection ? fleet.dispatchDraft : null;
  React.useEffect(() => {
    if (!pendingMissionDraftKey || !dispatchDraft) {
      return;
    }
    if (fleetDispatchPrepareKey(dispatchDraft) === pendingMissionDraftKey) {
      setPendingMissionDraftKey(null);
      setDispatchStage("mission");
    }
  }, [dispatchDraft, pendingMissionDraftKey]);
  if (dispatchDraft && dispatchStage === "target") {
    return (
      <>
        <FleetTargetStepTable
          draft={dispatchDraft}
          fleet={fleet}
          onPrepare={(draft) => {
            setPendingMissionDraftKey(fleetDispatchPrepareKey(draft));
            onPrepare(draft);
          }}
          pending={pending}
        />
        <br />
        <br />
        <br />
        <br />
      </>
    );
  }
  if (dispatchDraft && dispatchStage === "mission") {
    return (
      <>
        <FleetDispatchPreviewTable draft={dispatchDraft} fleet={fleet} onLaunch={onLaunch} pending={pending} />
        <br />
        <br />
        <br />
        <br />
      </>
    );
  }
  const content = (
    <>
      <div
        className="legacy-fleet-missions-view"
        dangerouslySetInnerHTML={{ __html: fleetMissionsTableHTML(fleet, pending) }}
        onSubmit={submitFleetCommand}
      />

      {unionMission ? (
        <FleetUnionManagementTable fleet={fleet} mission={unionMission} />
      ) : (
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
              <tr style={{ height: 20 }}>
                <td className="legacy-c c" colSpan={4}>
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
                    <a title={`Speed: ${Math.floor(Math.max(0, ship.speed))}`}>{ship.name}</a>
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
                          FLEET1_ALL
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
              {fleet.templates.commanderActive ? (
                <>
                  <tr style={{ height: 20 }}>
                    <td className="legacy-c c" colSpan={4}>
                      <u>
                        <a href={gameRouteURL("/game/fleet-templates", window.location.search)}>Standard fleets</a>
                      </u>
                    </td>
                  </tr>
                  {fleet.templates.items.reduce<React.ReactNode[]>((rows, template, index) => {
                    if (index % 2 !== 0) {
                      return rows;
                    }
                    const second = fleet.templates.items[index + 1];
                    const pair: GameFleetTemplate[] = second ? [template, second] : [template];
                    rows.push(
                      <tr key={template.id} style={{ height: 20 }}>
                        {pair.map((item) => (
                          <th colSpan={2} key={item.id}>
                            <a
                              href={fleetTemplateJavascriptHref(item)}
                              onClick={(event) => {
                                event.preventDefault();
                                setLegacyFleetTemplateShips(item);
                              }}
                            >
                              {item.name}
                            </a>
                          </th>
                        ))}
                        {second ? null : <th colSpan={2}></th>}
                      </tr>
                    );
                    return rows;
                  }, [])}
                </>
              ) : null}
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
      )}
      <br />
      <br />
      <br />
      <br />
    </>
  );
  return unionMission ? (
    <div className="legacy-fleet-union-view">
      {content}
    </div>
  ) : (
    content
  );
}

function fleetMissionsTableHTML(fleet: GameFleet, pending: boolean): string {
  const session = new URLSearchParams(window.location.search).get("session") ?? "";
  const formAction = `index.php?page=flotten1&session=${legacyHTMLAttribute(session)}`;
  let html = '  <table width="519" border="0" cellpadding="0" cellspacing="1" class="legacy-overview-table legacy-fleet-table">\n';
  html += '   <tr height="20">\n';
  html += '  <td colspan="8" class="c">\n';
  html += "  <table border=0 width=100%>\n";
  html += "   <tr>\n\n";
  html += "    <td style='background-color:transparent;'>\n";
  if (fleet.slots.admiral) {
    html += '    <div style="margin-top:2;margin-bottom:2;">';
  }
  html += `Fleets ${formatLegacyNumber(fleet.slots.used)} / ${formatLegacyNumber(fleet.slots.baseMax)}`;
  if (fleet.slots.admiral) {
    html += `<b><font style="color:lime;">+2</font></b> <img border="0" alt="Fleet Admiral" src="${legacyHTMLAttribute(
      `${gameImageBase}/admiral_ikon.gif`
    )}" width="20" height="20" style="vertical-align:middle;">`;
    html += "</div>\n";
  }
  html += "    </td>\n";
  html += "    <td align=right style='background-color:transparent;'>\n";
  html += `      ${formatLegacyNumber(fleet.expeditions.used)}/${formatLegacyNumber(fleet.expeditions.max)} Expeditions    \n`;
  html += "    </td>\n";
  html += "    </tr>\n";
  html += "    </table>\n";
  html += "    </td>\n\n";
  html += "   </tr>\n";
  html += '     <tr height="20">\n';
  for (const label of ["ID", "Mission", "Ships (total)", "Origin", "Departure Time", "Target", "Arrival Time", "Commands"]) {
    html += `    <th>${label}</th>\n`;
  }
  html += "   </tr>\n";

  if (fleet.missions.length === 0) {
    html += '   <tr height="20"> \n';
    for (let index = 0; index < 8; index += 1) {
      html += "    <th>-</th> \n";
    }
    html += "   </tr> \n";
  } else {
    fleet.missions.forEach((mission, index) => {
      html += '     <tr height="20">\n';
      html += `    <th>${index + 1}</th>\n\n`;
      html += "    <th>\n";
      html += fleetMissionTextHTML(mission);
      html += "\n    </th>\n";
      html += `    <th> <a title="${legacyHTMLAttribute(fleetMissionShipsTitle(mission))}">${formatLegacyNumber(mission.totalShips)}</a></th>\n`;
      html += `    <th><a href="${legacyHTMLAttribute(fleetGalaxyLegacyHref(mission.origin, session))}" >[${formatCoordinates(
        mission.origin
      )}]</a></th>\n\n`;
      html += `    <th>${formatFleetTimestamp(mission.departureAt)}</th>\n`;
      html += `    <th><a href="${legacyHTMLAttribute(fleetGalaxyLegacyHref(mission.target, session))}" >[${formatCoordinates(mission.target)}]</a>`;
      if (fleetMissionShowsTargetOwner(mission)) {
        html += `   <br />${legacyHTMLText(mission.targetOwnerName)}`;
      }
      html += "    </th>\n";
      html += `    <th>${formatFleetTimestamp(mission.arrivalAt)}</th>\n`;
      html += "    <th>\n";
      if (mission.canCreateUnion) {
        html += `         <form action="${formAction}" method="POST">\n`;
        html += `    <input type="hidden" name="order_union" value="${mission.id}" />\n`;
        html += '        <input type="submit" value="Union" />\n';
        html += "     </form>\n";
      }
      if (mission.canRecall) {
        html += `         <form action="${formAction}" method="POST">\n`;
        html += `    <input type="hidden" name="order_return" value="${mission.id}" />\n`;
        html += `        <input type="submit" value="Recall"${pending ? ' disabled="disabled"' : ""} />\n`;
        html += "     </form>\n";
      }
      html += "            </th>\n";
      html += "   </tr>\n\n";
    });
  }
  html += "\n  </table>\n";
  return html;
}

function fleetMissionTextHTML(mission: GameFleetMission): string {
  return `<a title="">${legacyHTMLText(mission.missionName)}</a>\n<a title="${legacyHTMLAttribute(mission.stateTitle)}">${legacyHTMLText(
    mission.stateShort
  )}</a>`;
}

function fleetMissionShipsTitle(mission: GameFleetMission): string {
  return mission.ships.map((ship) => `${ship.name}: ${formatLegacyNumber(ship.count)} \n`).join("");
}

function fleetGalaxyLegacyHref(coordinates: Coordinates, session: string): string {
  const query = new URLSearchParams();
  query.set("page", "galaxy");
  query.set("galaxy", String(coordinates.galaxy));
  query.set("system", String(coordinates.system));
  query.set("position", String(coordinates.position));
  if (session) {
    query.set("session", session);
  }
  return `index.php?${query.toString()}`;
}

function FleetUnionManagementTable({ fleet, mission }: { fleet: GameFleet; mission: GameFleetMission }) {
  const unionName = (mission.unionName ?? "").trim() || `KV${mission.id}`;
  const players =
    mission.unionPlayers && mission.unionPlayers.length > 0
      ? mission.unionPlayers
      : [{ id: mission.ownerId, name: mission.ownerName || fleet.commander }];
  return (
    <form
      className="legacy-fleet-union-form"
      onSubmit={(event) => {
        event.preventDefault();
      }}
    >
      <input name="flotten" type="hidden" value={mission.id} />
      <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-fleet-union-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              Union of Fleets {unionName}
            </td>
          </tr>
          <tr>
            <td className="legacy-c c" colSpan={2}>
              Change the name of the union
            </td>
          </tr>
          <tr>
            <th colSpan={2}>
              <input defaultValue={unionName} name="union_name" type="text" /> <br />
              <input type="submit" value="OK" />
            </th>
          </tr>
          <tr>
            <td className="legacy-c c">Invited participants</td>
            <td className="legacy-c c">Invite a participant</td>
          </tr>
          <tr>
            <th style={{ width: "50%" }}>
              <select size={5}>
                {players.map((player, index) => (
                  <option key={`${player.id}-${index}`}>{player.name}</option>
                ))}
              </select>
            </th>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function fleetMissionShowsTargetOwner(mission: GameFleetMission): boolean {
  if (!mission.targetOwnerName) {
    return false;
  }
  return ![10002, 10004, 20000].includes(mission.targetType);
}

function FleetTargetStepTable({
  draft,
  fleet,
  onPrepare,
  pending
}: {
  draft: GameFleetDispatchDraft;
  fleet: GameFleet;
  onPrepare: (draft: GameFleetDispatchPrepare) => void;
  pending: boolean;
}) {
  const targetPlanets = fleet.planetSwitcher.filter(
    (planet) => planet.id !== fleet.currentPlanet.id && planet.type !== 2
  );
  const battleUnions = fleetBattleUnions(fleet);
  const metrics = legacyFleetTargetMetrics(draft, fleet.ships);
  const capacityColor = metrics.storage >= 0 ? "lime" : "red";
  const selectTarget = (form: HTMLFormElement | null, target: GamePlanetSummary) => {
    if (!form) {
      return;
    }
    setLegacyFormInputValue(form, "galaxy", target.coordinates.galaxy);
    setLegacyFormInputValue(form, "system", target.coordinates.system);
    setLegacyFormInputValue(form, "planet", target.coordinates.position);
    setLegacyFormInputValue(form, "planettype", target.type);
  };
  const selectUnionTarget = (form: HTMLFormElement | null, mission: GameFleetMission) => {
    if (!form) {
      return;
    }
    setLegacyFormInputValue(form, "galaxy", mission.target.galaxy);
    setLegacyFormInputValue(form, "system", mission.target.system);
    setLegacyFormInputValue(form, "planet", mission.target.position);
    setLegacyFormInputValue(form, "planettype", mission.targetType);
    setLegacyFormInputValue(form, "union2", mission.unionId);
  };
  const submitTarget = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onPrepare({
      ships: fleetDraftShipsPayload(draft),
      target: {
        galaxy: legacyFormInt(form.get("galaxy"), draft.target.galaxy),
        system: legacyFormInt(form.get("system"), draft.target.system),
        position: legacyFormInt(form.get("planet"), draft.target.position)
      },
      targetType: legacyFormInt(form.get("planettype"), draft.targetType),
      mission: draft.mission,
      speed: legacyFormInt(form.get("speed"), draft.speed)
    });
  };
  return (
    <form className="legacy-fleet-target-form" data-dispatch-action="prepare-target" onSubmit={submitTarget}>
      {battleUnions.length > 0 ? <input name="union2" type="hidden" value={0} /> : null}
      <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-fleet-target-table" width={519}>
        <tbody>
          <tr style={{ height: 20 }}>
            <td className="legacy-c c" colSpan={2}>
              Departure of the fleet
            </td>
          </tr>
          <tr style={{ height: 20 }}>
            <th style={{ width: "50%" }}>Target coordinates</th>
            <th>
              <input defaultValue={draft.target.galaxy} maxLength={2} name="galaxy" size={3} />{" "}
              <input defaultValue={draft.target.system} maxLength={3} name="system" size={3} />{" "}
              <input defaultValue={draft.target.position} maxLength={2} name="planet" size={3} />{" "}
              <select defaultValue={draft.targetType} name="planettype">
                <option value={1}>planet </option>
                <option value={2}>debris field </option>
                <option value={3}>moon </option>
              </select>
            </th>
          </tr>
          <tr style={{ height: 20 }}>
            <th>Speed</th>
            <th>
              <select defaultValue={draft.speed} name="speed">
                {Array.from({ length: 10 }, (_, index) => 10 - index).map((speed) => (
                  <option key={speed} value={speed}>
                    {speed * 10}
                  </option>
                ))}
              </select>{" "}
              %
            </th>
          </tr>
          <tr style={{ height: 20 }}>
            <th>Distance</th>
            <th>
              <div id="distance">{formatLegacyNumber(draft.distance)}</div>
            </th>
          </tr>
          <tr style={{ height: 20 }}>
            <th>Duration (one way)</th>
            <th>
              <div id="duration">{formatLegacyFleetTargetDuration(metrics.durationSeconds)}</div>
            </th>
          </tr>
          <tr style={{ height: 20 }}>
            <th>Fuel consumption</th>
            <th>
              <div id="consumption">
                <span style={{ color: capacityColor }}>{formatLegacyNumber(metrics.fuelConsumption)}</span>
              </div>
            </th>
          </tr>
          <tr style={{ height: 20 }}>
            <th>Maximum speed</th>
            <th>
              <div id="maxspeed">{formatLegacyNumber(draft.maxSpeed)}</div>
            </th>
          </tr>
          <tr style={{ height: 20 }}>
            <th>Load capacity</th>
            <th>
              <div id="storage">
                <span style={{ color: capacityColor }}>{formatLegacySignedNumber(metrics.storage)}</span>
              </div>
            </th>
          </tr>
          <tr style={{ height: 20 }}>
            <td className="legacy-c c" colSpan={2}>
              Planet
            </td>
          </tr>
          {targetPlanets.length === 0
            ? null
            : targetPlanets.reduce<React.ReactNode[]>((rows, planet, index) => {
              if (index % 2 === 0) {
                rows.push(
                  <tr key={planet.id} style={{ height: 20 }}>
                    <th>
                      <a
                        href="#set-target"
                        onClick={(event) => {
                          event.preventDefault();
                          selectTarget(event.currentTarget.closest("form"), planet);
                        }}
                      >
                        {planet.name} {formatCoordinates(planet.coordinates)}
                      </a>
                    </th>
                    {targetPlanets[index + 1] ? (
                      <th>
                        <a
                          href="#set-target"
                          onClick={(event) => {
                            event.preventDefault();
                            selectTarget(event.currentTarget.closest("form"), targetPlanets[index + 1]);
                          }}
                        >
                          {targetPlanets[index + 1].name} {formatCoordinates(targetPlanets[index + 1].coordinates)}
                        </a>
                      </th>
                    ) : (
                      <th>&nbsp; </th>
                    )}
                  </tr>
                );
              }
              return rows;
            }, [])}
          <tr style={{ height: 20 }}>
            <td className="legacy-c c" colSpan={2}>
              Battle unions
            </td>
          </tr>
          {battleUnions.length > 0 ? (
            <>
              {battleUnions.map((mission, index) => {
                const remaining = Math.max(0, mission.arrivalAt - Math.floor(Date.now() / 1000));
                return (
                  <tr key={mission.unionId} style={{ height: 20 }}>
                    <th>
                      <div data-star={mission.arrivalAt} id={`bxx${index + 1}`} title={String(remaining)}>
                        {formatLegacyDuration(remaining)}
                      </div>
                    </th>
                    <th>
                      <a
                        href="#set-union-target"
                        onClick={(event) => {
                          event.preventDefault();
                          selectUnionTarget(event.currentTarget.closest("form"), mission);
                        }}
                      >
                        {(mission.unionName ?? "").trim() || `KV${mission.id}`} [{formatCoordinates(mission.target)}]
                      </a>
                    </th>
                  </tr>
                );
              })}
            </>
          ) : (
            <tr style={{ height: 20 }}>
              <th colSpan={2}>-</th>
            </tr>
          )}
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

function fleetBattleUnions(fleet: GameFleet): GameFleetMission[] {
  const seen = new Set<number>();
  return fleet.missions.filter((mission) => {
    if (mission.unionId <= 0 || seen.has(mission.unionId)) {
      return false;
    }
    seen.add(mission.unionId);
    return true;
  });
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
  const titleRowAttrs = { align: "left", height: 20 } as React.HTMLAttributes<HTMLTableRowElement>;
  const bodyRowAttrs = { align: "left", valign: "top" } as React.HTMLAttributes<HTMLTableRowElement>;
  const heightRowAttrs = { height: 20 } as React.HTMLAttributes<HTMLTableRowElement>;
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
      unionId: legacyFormInt(form.get("union2"), 0)
    });
  };
  return (
    <form
      className="legacy-fleet-dispatch-form"
      data-dispatch-action="launch-dispatch"
      onSubmit={submitLaunch}
    >
      <FleetDispatchHiddenInputs draft={draft} fleet={fleet} />
      <table border={0} cellPadding={0} cellSpacing={1} className="legacy-overview-table legacy-fleet-dispatch-table" width={519}>
        <tbody>
          <tr {...titleRowAttrs}>
            <td className="c" colSpan={2}>
              {`${formatCoordinates(draft.target)} - ${fleetPlanetTypeName(draft.targetType)}`}
            </td>
          </tr>
          <tr {...bodyRowAttrs}>
            <th style={{ width: "50%" }}>
              <FleetDispatchMissionTable draft={draft} />
            </th>
            <th>
              <FleetDispatchResourcesTable draft={draft} fleet={fleet} />
            </th>
          </tr>
          <tr {...heightRowAttrs}>
            <th colSpan={2}>
              <input disabled={pending} type="submit" value="Next" />
            </th>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function FleetDispatchHiddenInputs({ draft, fleet }: { draft: GameFleetDispatchDraft; fleet: GameFleet }) {
  const selectedCounts = new Map(draft.ships.map((ship) => [ship.id, ship.count]));
  return (
    <>
      <input name="thisgalaxy" type="hidden" value={fleet.currentPlanet.coordinates.galaxy} />
      <input name="thissystem" type="hidden" value={fleet.currentPlanet.coordinates.system} />
      <input name="thisplanet" type="hidden" value={fleet.currentPlanet.coordinates.position} />
      <input name="thisplanettype" type="hidden" value={fleet.currentPlanet.type === 2 ? 3 : 1} />
      <input name="speedfactor" type="hidden" value={draft.speedFactor} />
      {draft.resources.map((resource, index) => (
        <input key={resource.id} name={`thisresource${index + 1}`} type="hidden" value={Math.floor(resource.available)} />
      ))}
      <input name="galaxy" type="hidden" value={draft.target.galaxy} />
      <input name="system" type="hidden" value={draft.target.system} />
      <input name="planet" type="hidden" value={draft.target.position} />
      <input name="planettype" type="hidden" value={draft.targetType} />
      {fleet.ships.map((ship) => {
        const count = selectedCounts.get(ship.id) ?? 0;
        if (count <= 0 || ship.id === 212 || !ship.selectable) {
          return null;
        }
        return (
          <React.Fragment key={ship.id}>
            <input name={`ship${ship.id}`} type="hidden" value={count} />
            <input name={`consumption${ship.id}`} type="hidden" value={ship.consumption} />
            <input name={`speed${ship.id}`} type="hidden" value={ship.speed} />
            <input name={`capacity${ship.id}`} type="hidden" value={ship.cargo} />
          </React.Fragment>
        );
      })}
      <input name="speed" type="hidden" value={draft.speed} />
    </>
  );
}

function FleetDispatchMissionTable({ draft }: { draft: GameFleetDispatchDraft }) {
  const heightRowAttrs = { height: 20 } as React.HTMLAttributes<HTMLTableRowElement>;
  return (
    <table border={0} cellPadding={0} cellSpacing={0} width={259}>
      <tbody>
        <tr {...heightRowAttrs}>
          <td className="c" colSpan={2}>
            Mission
          </td>
        </tr>
        {draft.missionOptions.length === 0 ? (
          <tr {...heightRowAttrs}>
            <th>
              <span style={{ color: "red" }}>No suitable missions</span>
            </th>
          </tr>
        ) : (
          draft.missionOptions.map((mission) => (
            <tr key={mission.id} {...heightRowAttrs}>
              <th>
                <input defaultChecked={mission.selected} name="order" type="radio" value={mission.id} />
                {mission.name}
                <br />
                {mission.warning ? (
                  <>
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
  const heightRowAttrs = { height: 20 } as React.HTMLAttributes<HTMLTableRowElement>;
  const expeditionSelected = draft.missionOptions.some((mission) => mission.id === 15);
  const acsSelected = draft.missionOptions.some((mission) => mission.id === 2);
  const battleUnions = fleetBattleUnions(fleet);
  return (
    <table border={0} cellPadding={0} cellSpacing={0} width={259}>
      <tbody>
        <tr {...heightRowAttrs}>
          <td className="c" colSpan={3}>
            Resources
          </td>
        </tr>
        {draft.resources.map((resource, index) => (
          <tr key={resource.id} {...heightRowAttrs}>
            <th>{resource.name}</th>
            <th>
              <a
                href="#max-resource"
                onClick={(event) => {
                  event.preventDefault();
                  setLegacyFleetResourceAmount(event.currentTarget, index + 1);
                }}
              >
                max
              </a>
            </th>
            <th>
              <input
                aria-label={resource.name}
                data-resource-id={resource.id}
                name={`resource${index + 1}`}
                size={10}
                title={`${resource.name} ${formatLegacyNumber(resource.available)}`}
                type="text"
              />
            </th>
          </tr>
        ))}
        <tr {...heightRowAttrs}>
          <th>Residue</th>
          <th colSpan={2}>
            <div id="remainingresources">-</div>
          </th>
        </tr>
        <tr {...heightRowAttrs}>
          <th colSpan={3}>
            <a
              href="#max-resources"
              onClick={(event) => {
                event.preventDefault();
                setLegacyFleetAllResources(event.currentTarget);
              }}
            >
              All resources
            </a>
          </th>
        </tr>
        <tr {...heightRowAttrs}>
          <th>&nbsp; </th>
        </tr>
        {acsSelected && battleUnions.length > 0 ? (
          <>
            <tr {...heightRowAttrs}>
              <td className="c" colSpan={3}>
                Battle unions
              </td>
            </tr>
            <tr {...heightRowAttrs}>
              <th colSpan={3}>
                <select name="union2">
                  {battleUnions.map((mission) => (
                    <option key={mission.unionId} value={mission.unionId}>
                      {(mission.unionName ?? "").trim() || `KV${mission.id}`}
                    </option>
                  ))}
                </select>
              </th>
            </tr>
            <tr {...heightRowAttrs}>
              <th>&nbsp; </th>
            </tr>
          </>
        ) : null}
        {draft.holdHours && draft.holdHours.length > 0 ? (
          <>
            <tr {...heightRowAttrs}>
              <td className="c" colSpan={3}>
                Hold time
              </td>
            </tr>
            <tr {...heightRowAttrs}>
              <th colSpan={3}>
                <select name="holdingtime" defaultValue={1}>
                  {draft.holdHours.map((hour) => (
                    <option key={hour} value={hour}>
                      {hour}
                    </option>
                  ))}
                </select>
                {" \n      Time in hours   "}
              </th>
            </tr>
          </>
        ) : null}
        {expeditionSelected && draft.expeditionHours && draft.expeditionHours.length > 0 ? (
          <>
            <tr {...heightRowAttrs}>
              <td className="c" colSpan={3}>
                Hold time
              </td>
            </tr>
            <tr {...heightRowAttrs}>
              <th colSpan={3}>
                <select name="expeditiontime">
                  {draft.expeditionHours.map((hour) => (
                    <option key={hour} value={hour}>
                      {hour}
                    </option>
                  ))}
                </select>
                {" \n      Time in hours   "}
              </th>
            </tr>
          </>
        ) : null}
      </tbody>
    </table>
  );
}

function legacyFleetTargetMetrics(draft: GameFleetDispatchDraft, availableShips: GameFleetShip[]) {
  const durationSeconds = legacyFleetFlightTime(draft.distance, draft.maxSpeed, draft.speed, draft.speedFactor);
  const selected = new Map(draft.ships.map((ship) => [ship.id, ship.count]));
  const allCargo = availableShips.reduce((total, ship) => total + (selected.get(ship.id) ?? 0) * ship.cargo, 0);
  const fuelConsumption = legacyFleetDisplayConsumption(draft, availableShips, durationSeconds, (ship) => (selected.get(ship.id) ?? 0) > 0);
  const probeCount = selected.get(210) ?? 0;
  const probeShip = availableShips.find((ship) => ship.id === 210);
  const probeCargo = probeShip ? probeShip.cargo * probeCount : 0;
  const probeConsumption =
    probeCount > 0 ? legacyFleetDisplayConsumption(draft, availableShips, durationSeconds, (ship) => ship.id === 210 && probeCount > 0) : 0;
  const unusedProbeStorage = Math.max(0, probeCargo - probeConsumption);
  return {
    durationSeconds,
    fuelConsumption,
    storage: allCargo - fuelConsumption - unusedProbeStorage
  };
}

function legacyFleetFlightTime(distance: number, slowestSpeed: number, speed: number, speedFactor: number) {
  if (distance <= 0 || slowestSpeed <= 0) {
    return 0;
  }
  const normalizedSpeed = clampNumber(speed, 1, 10);
  const normalizedSpeedFactor = Math.max(1, speedFactor);
  return Math.round((35000 / normalizedSpeed * Math.sqrt((distance * 10) / slowestSpeed) + 10) / normalizedSpeedFactor);
}

function legacyFleetDisplayConsumption(
  draft: GameFleetDispatchDraft,
  availableShips: GameFleetShip[],
  durationSeconds: number,
  includeShip: (ship: GameFleetShip) => boolean
) {
  if (draft.distance <= 0 || durationSeconds <= 0) {
    return 0;
  }
  const denominator = durationSeconds * Math.max(1, draft.speedFactor) - 10;
  if (denominator <= 0) {
    return 0;
  }
  const selected = new Map(draft.ships.map((ship) => [ship.id, ship.count]));
  const consumption = availableShips.reduce((total, ship) => {
    const amount = selected.get(ship.id) ?? 0;
    if (amount <= 0 || !includeShip(ship) || ship.speed <= 0 || ship.consumption <= 0) {
      return total;
    }
    const fleetSpeed = 35000 / denominator * Math.sqrt((draft.distance * 10) / ship.speed);
    const basicConsumption = ship.consumption * amount;
    return total + basicConsumption * draft.distance / 35000 * Math.pow(fleetSpeed / 10 + 1, 2);
  }, 0);
  return Math.round(consumption) + 1;
}

function formatLegacyFleetTargetDuration(totalSeconds: number): string {
  const safe = Math.max(0, Math.floor(totalSeconds));
  const hours = Math.floor(safe / 3600);
  const minutes = Math.floor((safe - hours * 3600) / 60);
  const seconds = safe - hours * 3600 - minutes * 60;
  return `${hours}:${minutes.toString().padStart(2, "0")}:${seconds.toString().padStart(2, "0")} h`;
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

function fleetDispatchPrepareKey(draft: GameFleetDispatchPrepare | GameFleetDispatchDraft): string {
  const entries = Array.isArray(draft.ships)
    ? draft.ships.map((ship) => [String(ship.id), ship.count] as const)
    : Object.entries(draft.ships);
  const ships = entries
    .map(([id, count]) => `${id}:${count}`)
    .sort()
    .join(",");
  return [
    ships,
    draft.target.galaxy,
    draft.target.system,
    draft.target.position,
    draft.targetType,
    draft.mission,
    draft.speed
  ].join("|");
}

function setLegacyFleetResourceAmount(anchor: HTMLAnchorElement, resourceIndex: number) {
  const form = anchor.closest("form");
  const input = form?.elements.namedItem(`resource${resourceIndex}`);
  if (input instanceof HTMLInputElement) {
    const resource = legacyFleetInputInt(form, `thisresource${resourceIndex}`, 0);
    const chosen = legacyFleetInputInt(form, `resource${resourceIndex}`, 0);
    const available = resourceIndex === 3 ? resource - legacyFleetFormConsumption(form) : resource;
    const freeCapacity = Math.max(legacyFleetFormStorage(form) - legacyFleetLoadedResources(form, 0), 0);
    input.value = String(Math.min(freeCapacity + chosen, available));
    legacyFleetUpdateTransportCapacity(form);
  }
}

function setLegacyFleetAllResources(anchor: HTMLAnchorElement) {
  const form = anchor.closest("form");
  if (!form) {
    return;
  }
  const storCap = legacyFleetFormStorage(form);
  let metal = legacyFleetInputInt(form, "thisresource1", 0);
  let crystal = legacyFleetInputInt(form, "thisresource2", 0);
  let deuterium = legacyFleetInputInt(form, "thisresource3", 0) - legacyFleetFormConsumption(form);
  const freeCapacity = storCap - metal - crystal - deuterium;
  if (freeCapacity < 0) {
    metal = Math.min(metal, storCap);
    crystal = Math.min(crystal, storCap - metal);
    deuterium = Math.min(deuterium, storCap - metal - crystal);
  }
  [metal, crystal, deuterium].forEach((amount, index) => {
    const input = form.elements.namedItem(`resource${index + 1}`);
    if (input instanceof HTMLInputElement) {
      input.value = String(Math.max(amount, 0));
    }
  });
  legacyFleetUpdateTransportCapacity(form);
}

function legacyFleetFormStorage(form: HTMLFormElement | null): number {
  if (!form) {
    return 0;
  }
  let storage = 0;
  for (let id = 200; id < 300; id += 1) {
    const amount = legacyFleetInputInt(form, `ship${id}`, 0);
    if (amount >= 1) {
      storage += amount * legacyFleetInputInt(form, `capacity${id}`, 0);
    }
  }
  storage -= legacyFleetFormConsumption(form);
  if (form.elements.namedItem("ship210")) {
    storage -= legacyFleetUnusedProbeStorage(form);
  }
  return storage;
}

function legacyFleetUnusedProbeStorage(form: HTMLFormElement): number {
  const storage = legacyFleetInputInt(form, "capacity210", 0) * legacyFleetInputInt(form, "ship210", 0);
  const remaining = storage - legacyFleetFormConsumption(form, (id) => id === 210);
  return remaining > 0 ? remaining : 0;
}

function legacyFleetFormConsumption(form: HTMLFormElement | null, includeShip: (id: number) => boolean = () => true): number {
  if (!form) {
    return 0;
  }
  const distance = legacyFleetFormDistance(form);
  const duration = legacyFleetFormDuration(form);
  const speedFactor = legacyFleetInputInt(form, "speedfactor", 1);
  const denominator = duration * speedFactor - 10;
  if (distance <= 0 || duration <= 0 || denominator <= 0) {
    return 0;
  }
  let consumption = 0;
  for (let id = 200; id < 220; id += 1) {
    const amount = legacyFleetInputInt(form, `ship${id}`, 0);
    const shipSpeed = legacyFleetInputInt(form, `speed${id}`, 0);
    const shipConsumption = legacyFleetInputInt(form, `consumption${id}`, 0);
    if (amount <= 0 || shipSpeed <= 0 || shipConsumption <= 0 || !includeShip(id)) {
      continue;
    }
    const fleetSpeed = 35000 / denominator * Math.sqrt((distance * 10) / shipSpeed);
    consumption += shipConsumption * amount * distance / 35000 * Math.pow(fleetSpeed / 10 + 1, 2);
  }
  return Math.round(consumption) + 1;
}

function legacyFleetFormDuration(form: HTMLFormElement): number {
  const speedFactor = legacyFleetInputInt(form, "speedfactor", 1);
  const maxSpeed = legacyFleetFormMaxSpeed(form);
  const speed = legacyFleetInputInt(form, "speed", 10);
  const distance = legacyFleetFormDistance(form);
  if (maxSpeed <= 0 || distance <= 0) {
    return 0;
  }
  return Math.round((35000 / speed * Math.sqrt((distance * 10) / maxSpeed) + 10) / speedFactor);
}

function legacyFleetFormMaxSpeed(form: HTMLFormElement): number {
  let maxSpeed = 1000000000;
  for (let id = 200; id < 220; id += 1) {
    const amount = legacyFleetInputInt(form, `ship${id}`, 0);
    const speed = legacyFleetInputInt(form, `speed${id}`, 0);
    if (speed >= 1 && amount >= 1) {
      maxSpeed = Math.min(maxSpeed, speed);
    }
  }
  return maxSpeed;
}

function legacyFleetFormDistance(form: HTMLFormElement): number {
  const thisGalaxy = legacyFleetInputInt(form, "thisgalaxy", 0);
  const thisSystem = legacyFleetInputInt(form, "thissystem", 0);
  const thisPlanet = legacyFleetInputInt(form, "thisplanet", 0);
  const targetGalaxy = legacyFleetInputInt(form, "galaxy", 0);
  const targetSystem = legacyFleetInputInt(form, "system", 0);
  const targetPlanet = legacyFleetInputInt(form, "planet", 0);
  if (targetGalaxy - thisGalaxy !== 0) {
    return Math.abs(targetGalaxy - thisGalaxy) * 20000;
  }
  if (targetSystem - thisSystem !== 0) {
    return Math.abs(targetSystem - thisSystem) * 5 * 19 + 2700;
  }
  if (targetPlanet - thisPlanet !== 0) {
    return Math.abs(targetPlanet - thisPlanet) * 5 + 1000;
  }
  return 5;
}

function legacyFleetInputInt(form: HTMLFormElement | null, name: string, fallback: number): number {
  const input = form?.elements.namedItem(name);
  const value = input instanceof HTMLInputElement || input instanceof HTMLSelectElement ? Number.parseInt(input.value, 10) : Number.NaN;
  return Number.isFinite(value) ? value : fallback;
}

function legacyFleetUpdateTransportCapacity(form: HTMLFormElement | null) {
  if (!form) {
    return;
  }
  const residue = legacyFleetFormStorage(form) - Math.abs(legacyFleetInputInt(form, "resource1", 0)) - Math.abs(legacyFleetInputInt(form, "resource2", 0)) - Math.abs(legacyFleetInputInt(form, "resource3", 0));
  const target = form.querySelector<HTMLElement>("#remainingresources");
  if (target) {
    target.innerHTML = `<font color=${residue < 0 ? "red" : "lime"}>${residue}</font>`;
  }
}

function legacyFleetLoadedResources(form: HTMLFormElement | null, excludingIndex: number): number {
  if (!form) {
    return 0;
  }
  let loaded = 0;
  for (let index = 1; index <= 3; index += 1) {
    if (excludingIndex > 0 && index === excludingIndex) {
      continue;
    }
    const input = form.elements.namedItem(`resource${index}`);
    loaded += legacyFormInt(input instanceof HTMLInputElement ? input.value : "", 0);
  }
  return loaded;
}

function setLegacyFormInputValue(form: HTMLFormElement, name: string, value: number) {
  const input = form.elements.namedItem(name);
  if (input instanceof HTMLInputElement || input instanceof HTMLSelectElement) {
    input.value = String(value);
  }
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
      return "debris field";
    case 3:
      return "moon";
    default:
      return "planet";
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
              <td className="legacy-c c" colSpan={4}>
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
              <td className="legacy-c c" colSpan={4}>
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

function GalaxyTable({
  adminLevel,
  galaxy,
  onInstantDispatch,
  onMissileLaunch,
  pending
}: {
  adminLevel: number;
  galaxy: GameGalaxy;
  onInstantDispatch: (draft: GameGalaxyInstantDispatch) => void;
  onMissileLaunch: (draft: GameGalaxyMissileLaunch) => void;
  pending: boolean;
}) {
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
  const hasGalaxyInfo = galaxy.extra.commander || galaxy.remoteSystemCostDue;

  return (
    <>
      {galaxy.notEnoughDeuterium ? (
        <table className="legacy-overview-table legacy-galaxy-error-table" width={569}>
          <tbody>
            <tr>
              <td className="legacy-c c"> Error</td>
            </tr>
            <tr>
              <th>Not enough deuterium!</th>
            </tr>
          </tbody>
        </table>
      ) : null}
      <form className="legacy-galaxy-form" key={`${galaxy.coordinates.galaxy}:${galaxy.coordinates.system}`} onSubmit={submitCoordinates}>
        <table className="legacy-galaxy-nav-table legacy-header-table" id="t1">
          <tbody>
            <tr>
              <td className="legacy-header-cell">
                <table className="legacy-header-table" id="t2">
                  <tbody>
                    <tr>
                      <td className="legacy-c c" colSpan={3}>
                        Galaxy
                      </td>
                    </tr>
                    <tr>
                      <td className="legacy-l l">
                        <input
                          aria-label="Previous galaxy"
                          name="galaxyLeft"
                          onClick={() => navigateTo({ ...galaxy.coordinates, galaxy: galaxy.coordinates.galaxy - 1 })}
                          type="button"
                          value="<-"
                        />
                      </td>
                      <td className="legacy-l l">
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
                      <td className="legacy-l l">
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
                      <td className="legacy-c c" colSpan={3}>
                        Solar system
                      </td>
                    </tr>
                    <tr>
                      <td className="legacy-l l">
                        <input
                          aria-label="Previous system"
                          name="systemLeft"
                          onClick={() => navigateTo({ ...galaxy.coordinates, system: galaxy.coordinates.system - 1 })}
                          type="button"
                          value="<-"
                        />
                      </td>
                      <td className="legacy-l l">
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
                      <td className="legacy-l l">
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
      <GalaxyMissileForm galaxy={galaxy} onLaunch={onMissileLaunch} pending={pending} />
      <table className="legacy-overview-table legacy-galaxy-table" width={569}>
        <tbody>
          <tr>
            <td className="c" colSpan={8}>
              Solar system {galaxy.coordinates.galaxy}:{galaxy.coordinates.system}
            </td>
          </tr>
          <tr>
            {["Coord.", "Planet", "Title (activity)", "Moon", "Debris", "Player", "Alliance", "Actions"].map((label) => (
              <td className="c" key={label}>
                {label}
              </td>
            ))}
          </tr>
          {galaxy.rows.map((row) => (
            <GalaxyTableRow adminLevel={adminLevel} galaxy={galaxy} key={row.position} onInstantDispatch={onInstantDispatch} pending={pending} row={row} />
          ))}
          <tr>
            <th style={{ height: 32 }}>16</th>
            <th colSpan={7}>
              <a href={fleetTargetHref(galaxy.coordinates, 16, 15)}>Far space</a>
            </th>
          </tr>
          <tr>
            <td className="c" colSpan={6} dangerouslySetInnerHTML={{ __html: `(Populated ${formatLegacyNumber(galaxy.populated)} planets)` }} />
            <td className="c" colSpan={2}>
              <a href="#legend" onClick={(event) => event.preventDefault()}>
                Legend
              </a>
            </td>
          </tr>
          <tr id="fleetstatusrow" style={{ display: "none" }}>
            <th colSpan={8}>
              <table id="fleetstatustable" style={{ fontWeight: "bold" }} width="100%">
                <tbody />
              </table>
            </th>
          </tr>
        </tbody>
      </table>
      {hasGalaxyInfo ? (
        <table className="legacy-overview-table legacy-galaxy-info-table" width={569}>
          <tbody>
            <tr>
              <td className="c" colSpan={2}>
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
      ) : null}
      <br />
      <br />
    </>
  );
}

const galaxyMissileTargets = [
  { id: 401, label: "Rocket Launcher" },
  { id: 402, label: "Light Laser" },
  { id: 403, label: "Heavy Laser" },
  { id: 404, label: "Gauss Cannon" },
  { id: 405, label: "Ion Cannon" },
  { id: 406, label: "Plasma Turret" },
  { id: 407, label: "Small Shield Dome" },
  { id: 408, label: "Large Shield Dome" }
];

function GalaxyMissileForm({
  galaxy,
  onLaunch,
  pending
}: {
  galaxy: GameGalaxy;
  onLaunch: (draft: GameGalaxyMissileLaunch) => void;
  pending: boolean;
}) {
  const search = new URLSearchParams(window.location.search);
  if (!search.has("mode")) {
    return null;
  }
  const targetPlanetId = Number(search.get("pdd") ?? 0);
  if (!targetPlanetId) {
    return null;
  }
  const queryTarget: Coordinates = {
    galaxy: Number(search.get("p1") ?? search.get("galaxy") ?? galaxy.coordinates.galaxy) || galaxy.coordinates.galaxy,
    system: Number(search.get("p2") ?? search.get("system") ?? galaxy.coordinates.system) || galaxy.coordinates.system,
    position: Number(search.get("p3") ?? search.get("position") ?? galaxy.coordinates.position) || galaxy.coordinates.position
  };
  const rowTarget =
    galaxy.rows.find((row) => row.planet?.id === targetPlanetId)?.planet?.coordinates ??
    galaxy.rows.find((row) => row.moon?.id === targetPlanetId)?.moon?.coordinates;
  const targetCoordinates = rowTarget ?? queryTarget;
  const actionSearch = new URLSearchParams(window.location.search);
  actionSearch.delete("mode");

  return (
    <form
      action={gameRouteURL("/game/galaxy", actionSearch.toString())}
      method="post"
      onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        onLaunch({
          targetPlanetId,
          amount: Math.abs(Number(data.get("anz")) || 0),
          targetDefenseId: Math.abs(Number(data.get("pziel")) || 0)
        });
      }}
    >
      <table border={0}>
        <tbody>
          <tr>
            <td className="c" colSpan={2}>
              Launch a rocket to{" "}
              <a href={gameRouteURL("/game/galaxy", galaxyTargetSearch(targetCoordinates))}>{formatCoordinates(targetCoordinates)}</a>
            </td>
          </tr>
          <tr>
            <td className="c">
              Number of missiles ({formatLegacyNumber(galaxy.extra.missiles)} available):{" "}
              <input disabled={pending} maxLength={2} name="anz" size={2} type="text" />
            </td>
            <td className="c">
              Target:{" "}
              <select disabled={pending} name="pziel" defaultValue={0}>
                <option value={0}>Target all</option>
                {galaxyMissileTargets.map((target) => (
                  <option key={target.id} value={target.id}>
                    {target.label}
                  </option>
                ))}
              </select>
            </td>
          </tr>
          <tr>
            <td className="c" colSpan={2}>
              <input disabled={pending} name="aktion" type="submit" value="Attack" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function GalaxyTableRow({
  adminLevel,
  galaxy,
  onInstantDispatch,
  pending,
  row
}: {
  adminLevel: number;
  galaxy: GameGalaxy;
  onInstantDispatch: (draft: GameGalaxyInstantDispatch) => void;
  pending: boolean;
  row: GameGalaxyRow;
}) {
  const planet = row.planet;
  const player = planet?.player;
  const debrisCoordinates = row.planet?.coordinates ?? row.moon?.coordinates;
  const cellWidth = (value: number) => ({ width: String(value) }) as unknown as React.ThHTMLAttributes<HTMLTableCellElement>;
  const handleInstantMenuClick = React.useCallback(
    (event: React.MouseEvent<HTMLElement>) => {
      const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>("a[data-galaxy-instant]");
      if (!anchor || !event.currentTarget.contains(anchor)) {
        return;
      }
      event.preventDefault();
      if (pending) {
        return;
      }
      const action = anchor.dataset.galaxyInstant;
      const target = {
        galaxy: Number(anchor.dataset.galaxy),
        system: Number(anchor.dataset.system),
        position: Number(anchor.dataset.position)
      };
      const targetType = Number(anchor.dataset.targetType);
      const amount = Number(anchor.dataset.amount);
      if ((action !== "dispatch-spy" && action !== "dispatch-recycle") || !Number.isFinite(target.galaxy) || !Number.isFinite(target.system) || !Number.isFinite(target.position)) {
        return;
      }
      onInstantDispatch({
        action,
        target,
        targetType: Number.isFinite(targetType) ? targetType : 1,
        amount: Number.isFinite(amount) ? amount : 1
      });
    },
    [onInstantDispatch, pending]
  );

  return (
    <tr data-galaxy-position={row.position}>
      <th {...cellWidth(30)}>
        <a href="#" onClick={(event) => event.preventDefault()}>{row.position}</a>
      </th>
      <th {...cellWidth(30)}>
        {planet && planet.type === 1 ? (
          <GalaxyHoverMenu html={galaxyPlanetHoverHTML(planet, galaxy, adminLevel)} onClick={handleInstantMenuClick}>
            <a href="#" onClick={(event) => event.preventDefault()}>
              <img alt="" height={30} src={galaxyPlanetImagePath(planet, true)} width={30} />
            </a>
          </GalaxyHoverMenu>
        ) : null}
      </th>
      <th className="legacy-galaxy-name" style={{ whiteSpace: "nowrap" }} {...cellWidth(130)}>
        {planet ? (
          <>
            <span className={planet.abandoned ? "longinactive" : planet.destroyed ? "banned" : undefined}>{planet.displayName}</span>
            {planet.activityText ? <> {planet.activityText}</> : null}
          </>
        ) : null}
      </th>
      <th style={{ whiteSpace: "nowrap" }} {...cellWidth(30)}>
        {row.moon ? (
          row.moon.destroyed ? (
            <GalaxyHoverMenu html={`<font color=white><b>Moon destroyed</b></font>`} width={75}>
              <span className="legacy-galaxy-destroyed-moon">
                <img alt={`Moon (size: ${formatLegacyNumber(row.moon.diameter)})`} height={22} src={galaxyPlanetImagePath(row.moon, true)} width={22} />
              </span>
            </GalaxyHoverMenu>
          ) : (
            <GalaxyHoverMenu html={galaxyMoonHoverHTML(row.moon, galaxy, adminLevel)} onClick={handleInstantMenuClick} offsetY={-110}>
              <a
                href="#"
                onClick={(event) => {
                  event.preventDefault();
                  if (!pending && row.moon) {
                    onInstantDispatch({
                      action: "dispatch-spy",
                      target: row.moon.coordinates,
                      targetType: 3,
                      amount: Math.max(1, galaxy.extra.maxSpy || 0)
                    });
                  }
                }}
              >
                <img alt={`Moon (size: ${formatLegacyNumber(row.moon.diameter)})`} height={22} src={galaxyPlanetImagePath(row.moon, true)} width={22} />
              </a>
            </GalaxyHoverMenu>
          )
        ) : null}
      </th>
      <th {...cellWidth(30)}>
        {row.debris?.visible && debrisCoordinates ? (
          <GalaxyHoverMenu html={galaxyDebrisHoverHTML(row.debris, debrisCoordinates, row.position)} onClick={handleInstantMenuClick}>
            <a
              href="#"
              onClick={(event) => {
                event.preventDefault();
                if (pending) {
                  return;
                }
                onInstantDispatch({
                  action: "dispatch-recycle",
                  target: { galaxy: debrisCoordinates.galaxy, system: debrisCoordinates.system, position: row.position },
                  targetType: 2,
                  amount: Math.max(1, row.debris?.harvesters ?? 0)
                });
              }}
            >
              <img alt="" height={22} src={`${skinBase}/planeten/debris.jpg`} title={`${formatLegacyNumber(row.debris.metal)} / ${formatLegacyNumber(row.debris.crystal)}`} width={22} />
            </a>
          </GalaxyHoverMenu>
        ) : null}
      </th>
      {player ? (
        <th {...cellWidth(150)}>
          <GalaxyHoverMenu html={galaxyPlayerHoverHTML(player, adminLevel)} text>
            <span dangerouslySetInnerHTML={{ __html: galaxyPlayerCellHTML(player) }} />
          </GalaxyHoverMenu>
        </th>
      ) : (
        <th {...cellWidth(150)} />
      )}
      <th {...cellWidth(80)}>
        {planet?.alliance ? (
          <GalaxyHoverMenu html={galaxyAllianceHoverHTML(planet.alliance)} text offsetY={-50}>
            <a href="#" onClick={(event) => event.preventDefault()}>
              {planet.alliance.tag}
            </a>
          </GalaxyHoverMenu>
        ) : null}
      </th>
      <th className="legacy-galaxy-actions" style={{ whiteSpace: "nowrap" }} {...cellWidth(125)}>
        {planet ? <GalaxyActionIcons galaxy={galaxy} onInstantDispatch={onInstantDispatch} pending={pending} planet={planet} /> : null}
      </th>
    </tr>
  );
}

function GalaxyHoverMenu({
  children,
  html,
  offsetY = -40,
  onClick,
  text = false,
  width = 240
}: {
  children: React.ReactNode;
  html: string;
  offsetY?: number;
  onClick?: React.MouseEventHandler<HTMLElement>;
  text?: boolean;
  width?: number;
}) {
  const [open, setOpen] = React.useState(false);
  const timerRef = React.useRef<number | null>(null);
  const clearTimer = React.useCallback(() => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);
  const show = React.useCallback(() => {
    clearTimer();
    timerRef.current = window.setTimeout(() => setOpen(true), 750);
  }, [clearTimer]);
  const hide = React.useCallback(() => {
    clearTimer();
    setOpen(false);
  }, [clearTimer]);

  React.useEffect(() => clearTimer, [clearTimer]);

  return (
    <span
      className={`legacy-galaxy-hover${open ? " legacy-galaxy-hover-open" : ""}${text ? " legacy-galaxy-hover-text" : ""}`}
      onBlur={hide}
      onClick={onClick}
      onFocus={show}
      onMouseEnter={show}
      onMouseLeave={hide}
    >
      {children}
      <span className="legacy-galaxy-tooltip" style={{ top: offsetY, width }} dangerouslySetInnerHTML={{ __html: html }} />
    </span>
  );
}

function galaxyPlayerCellHTML(player: GameGalaxyPlayer): string {
  let html = `\n<a style="cursor:pointer">\n<span class="${legacyHTMLAttribute(player.statusClass)}">${legacyHTMLText(player.name)}</span></a>\n`;
  if (player.suffixes.length > 0) {
    html += "(";
    for (const [index, suffix] of player.suffixes.entries()) {
      html += `${index > 0 ? " " : ""}<span class="${legacyHTMLAttribute(suffix.class)}">${legacyHTMLText(suffix.text)}</span>`;
    }
    html += ")\n";
  }
  return html;
}

function galaxyPlanetHoverHTML(planet: GameGalaxyPlanet, galaxy: GameGalaxy, adminLevel: number): string {
  const title = `Planet ${planet.name} [${formatCoordinates(planet.coordinates)}]`;
  let actions = "";
  if (planet.own) {
    actions += galaxyFleetMenuLink(planet.coordinates, planet.coordinates.position, 4, 1, "Deploy");
    actions += galaxyFleetMenuLink(planet.coordinates, planet.coordinates.position, 3, 1, "Transport");
  } else {
    if (planet.actions.spy) {
      actions += galaxyInstantMenuLink("dispatch-spy", planet.coordinates, planet.coordinates.position, 1, Math.max(1, galaxy.extra.maxSpy || 0), "Espionage");
      actions += "<br />";
    }
    if (planet.actions.missile) {
      actions += galaxyAnchor(gameGalaxyMissileURL(planet.coordinates, planet.id, planet.player?.id ?? 0, window.location.search), "Rocket attack");
    }
    if (planet.actions.attack) {
      actions += galaxyFleetMenuLink(planet.coordinates, planet.coordinates.position, 1, 1, "Attack");
    }
    if (planet.actions.defend) {
      actions += galaxyFleetMenuLink(planet.coordinates, planet.coordinates.position, 5, 1, "Defend");
    }
    if (planet.actions.transport) {
      actions += galaxyFleetMenuLink(planet.coordinates, planet.coordinates.position, 3, 1, "Transport");
    }
  }
  if (adminLevel >= 1) {
    actions += galaxyAnchor(adminPlanetHref(planet.id), "Admin");
  }
  return `<table width=240><tr><td class=c colspan=2>${legacyHTMLText(title)}</td></tr><tr><th width=80><img src="${legacyHTMLAttribute(
    galaxyPlanetImagePath(planet, true)
  )}" height=75 width=75 /></th><th align=left>${actions}</th></tr></table>`;
}

function galaxyMoonHoverHTML(moon: GameGalaxyPlanet, galaxy: GameGalaxy, adminLevel: number): string {
  const title = `Moon ${moon.name} [${formatCoordinates(moon.coordinates)}]`;
  let actions = "";
  if (moon.own) {
    actions += galaxyFleetMenuLink(moon.coordinates, moon.coordinates.position, 3, 3, "Transport");
    actions += galaxyFleetMenuLink(moon.coordinates, moon.coordinates.position, 4, 3, "Deploy");
  } else {
    if (moon.actions.spy) {
      actions += galaxyInstantMenuLink("dispatch-spy", moon.coordinates, moon.coordinates.position, 3, Math.max(1, galaxy.extra.maxSpy || 0), "Espionage");
      actions += "<br />";
    }
    if (moon.actions.missile) {
      actions += galaxyAnchor(gameGalaxyMissileURL(moon.coordinates, moon.id, moon.player?.id ?? 0, window.location.search), "Rocket attack");
    }
    if (moon.actions.transport) {
      actions += galaxyFleetMenuLink(moon.coordinates, moon.coordinates.position, 3, 3, "Transport");
    }
    if (moon.actions.attack) {
      actions += galaxyFleetMenuLink(moon.coordinates, moon.coordinates.position, 1, 3, "Attack");
    }
    if (moon.actions.defend) {
      actions += galaxyFleetMenuLink(moon.coordinates, moon.coordinates.position, 5, 3, "Defend");
    }
    if (moon.actions.destroy) {
      actions += galaxyFleetMenuLink(moon.coordinates, moon.coordinates.position, 9, 3, "Destroy");
    }
  }
  if (adminLevel >= 1) {
    actions += galaxyAnchor(adminPlanetHref(moon.id), "Admin");
  }
  return `<table width=240><tr><td class=c colspan=2>${legacyHTMLText(title)}</td></tr><tr><th width=80><img src="${legacyHTMLAttribute(galaxyPlanetImagePath(moon, true))}" height=75 width=75 alt="${legacyHTMLAttribute(
    `Moon (size: ${formatLegacyNumber(moon.diameter)})`
  )}" /></th><th><table width=120><tr><td colspan=2 class=c>Properties</td></tr><tr><th>Size:</th><th>${formatLegacyNumber(moon.diameter)}</th></tr><tr><th>Temperatur:</th><th>${formatLegacyNumber(
    moon.temperature
  )}</th></tr><tr><td colspan=2 class=c>Actions:</td></tr><tr><th align=left colspan=2>${actions}</th></tr></table></th></tr></table>`;
}

function galaxyDebrisHoverHTML(debris: GameGalaxyDebris, coordinates: Coordinates, position: number): string {
  const recycle = galaxyInstantMenuLink("dispatch-recycle", coordinates, position, 2, Math.max(1, debris.harvesters), "Recycle");
  return `<table width=240><tr><td class=c colspan=2></td></tr><tr><th width=80><img src="${skinBase}/planeten/debris.jpg" height=75 width=75 alt=T /></th><th><table><tr><td class=c colspan=2>Resources:</td></tr><tr><th>Metal:</th><th>${formatLegacyNumber(
    debris.metal
  )}</th></tr><tr><th>Crystal:</th><th>${formatLegacyNumber(debris.crystal)}</th></tr><tr><td class=c colspan=2>Actions:</td></tr><tr><th colspan=2 align=left>${recycle}</th></tr></table></th></tr></table>`;
}

function galaxyPlayerHoverHTML(player: GameGalaxyPlayer, adminLevel: number): string {
  let rows = "";
  if (!player.own) {
    rows += `<tr><td>${galaxyAnchor(gameMessageComposeURL(player.id, window.location.search), "Write a message")}</td></tr>`;
    rows += `<tr><td>${galaxyAnchor(gameBuddyRequestURL(player.id, window.location.search), "Invite to become friends")}</td></tr>`;
  }
  rows += `<tr><td>${galaxyAnchor(galaxyStatisticsURL(player.rank, "player"), "Statistics")}</td></tr>`;
  if (adminLevel >= 1) {
    rows += `<tr><td>${galaxyAnchor(adminUserHref(player.id), "Admin")}</td></tr>`;
  }
  return `<table width=240><tr><td class=c>Player ${legacyHTMLText(player.name)}. Place in the rating - ${formatLegacyNumber(player.rank)}</td></tr><th><table>${rows}</table></th></table>`;
}

function galaxyAllianceHoverHTML(alliance: { id: number; tag: string }): string {
  const rows = [
    `<tr><td><a href="${legacyHTMLAttribute(allianceInfoURL(alliance.id))}" target="_ally">Alliance introduction</a></td></tr>`,
    `<tr><td>${galaxyAnchor(allianceURL({ a: "2", allyid: String(alliance.id) }), "Apply")}</td></tr>`,
    `<tr><td>${galaxyAnchor(galaxyStatisticsURL(1, "ally"), "Statistics")}</td></tr>`
  ].join("");
  return `<table width=240><tr><td class=c>Alliance ${legacyHTMLText(alliance.tag)}</td></tr><th><table>${rows}</table></th></table>`;
}

function galaxyFleetMenuLink(coordinates: Coordinates, position: number, mission: number, planetType: number, label: string): string {
  return galaxyAnchor(fleetTargetHref(coordinates, position, mission, planetType), label);
}

function galaxyInstantMenuLink(action: GameGalaxyInstantDispatch["action"], coordinates: Coordinates, position: number, targetType: number, amount: number, label: string): string {
  return `<a href="#" data-galaxy-instant="${action}" data-galaxy="${coordinates.galaxy}" data-system="${coordinates.system}" data-position="${position}" data-target-type="${targetType}" data-amount="${amount}">${legacyHTMLText(
    label
  )}</a><br />`;
}

function galaxyAnchor(href: string, label: string): string {
  return `<a href="${legacyHTMLAttribute(href)}">${legacyHTMLText(label)}</a><br />`;
}

function galaxyStatisticsURL(place: number, who: "player" | "ally"): string {
  const search = new URLSearchParams(window.location.search);
  const safePlace = Math.max(0, Math.floor(place));
  search.set("start", String(Math.floor(safePlace / 100) * 100 + 1));
  if (who === "ally") {
    search.set("who", "ally");
  } else {
    search.delete("who");
  }
  return gameRouteURL("/game/statistics", search.toString());
}

function GalaxyActionIcons({
  galaxy,
  onInstantDispatch,
  pending,
  planet
}: {
  galaxy: GameGalaxy;
  onInstantDispatch: (draft: GameGalaxyInstantDispatch) => void;
  pending: boolean;
  planet: GameGalaxyPlanet;
}) {
  const playerID = planet.player?.id ?? 0;
  const spyAmount = Math.max(1, galaxy.extra.maxSpy || 0);
  const actions: Array<{ enabled: boolean; href: string; icon: string; label: string; onClick?: () => void }> = [
    {
      enabled: planet.actions.spy,
      href: fleetTargetHref(planet.coordinates, planet.coordinates.position, 6),
      icon: "e.gif",
      label: "Espionage",
      onClick: () =>
        onInstantDispatch({
          action: "dispatch-spy",
          target: planet.coordinates,
          targetType: 1,
          amount: spyAmount
        })
    },
    { enabled: planet.actions.message && playerID > 0, href: gameMessageComposeURL(playerID, window.location.search), icon: "m.gif", label: "Write message" },
    { enabled: planet.actions.buddy && playerID > 0, href: gameBuddyRequestURL(playerID, window.location.search), icon: "b.gif", label: "Buddy request" },
    { enabled: planet.actions.missile, href: gameGalaxyMissileURL(planet.coordinates, planet.id, playerID, window.location.search), icon: "r.gif", label: "Rocket attack" }
  ];
  return (
    <>
      {actions.map((action) => {
        const onClick = action.onClick;
        return action.enabled ? (
          <React.Fragment key={action.icon}>
            <a
              data-galaxy-action={action.label}
              href={action.href}
              onClick={
                onClick
                  ? (event) => {
                      event.preventDefault();
                      if (!pending) {
                        onClick();
                      }
                    }
                  : undefined
              }
            >
              <img alt={action.label} src={`${skinBase}/img/${action.icon}`} title={action.label} />
            </a>
            {"\n"}
          </React.Fragment>
        ) : null;
      })}
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
      <>
        <table className="legacy-overview-table legacy-defense-table" width={530}>
          <tbody>
            <tr>
              <td className="legacy-l l" colSpan={2}>
                Description
              </td>
              <td className="legacy-l l">
                <b>Qty.</b>
              </td>
            </tr>
          </tbody>
        </table>
        <div className="legacy-shipyard-required-notice">
          <table>
            <tbody>
              <tr>
                <td className="legacy-c c">In order to do that, you need to build a shipyard!</td>
              </tr>
            </tbody>
          </table>
        </div>
      </>
    );
  }
  return (
    <>
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
            <td className="legacy-l l" colSpan={2}>
              Description
            </td>
            <td className="legacy-l l">
              <b>Qty.</b>
            </td>
          </tr>
          {defense.items.map((item) => (
            <tr data-defense-row={item.id} key={item.id}>
              <td className="legacy-l l legacy-building-image">
                <a href={technologyInfoURL(item.id)}>
                  <img alt="" height={120} src={`${skinBase}/gebaeude/${item.id}.gif`} width={120} />
                </a>
              </td>
              <td
                className="legacy-l l legacy-building-description"
                dangerouslySetInnerHTML={{ __html: shipyardDescriptionHTML(item) }}
              />
              <td className="legacy-k k legacy-building-action">
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
                    {defense.commanderActive && item.maxBuild > 0 && !isDefenseShieldDomeID(item.id) ? (
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
            <td align="center" className="legacy-c c" colSpan={2}>
              <input disabled={pending} type="submit" value="Build" />
            </td>
          </tr>
        </tbody>
      </table>
    </form>
    <ShipyardQueuePanel onComplete={() => undefined} queue={defense.queue} />
    </>
  );
}

function isDefenseShieldDomeID(id: number): boolean {
  return id === 407 || id === 408;
}

function SearchTable({ search }: { search: GameSearch }) {
  const hasExecutableSearch = hasExecutableSearchText(search.text);
  return (
    <>
      <form
        action={gameRouteURL("/game/search", window.location.search)}
        className="legacy-search-form"
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
              <td className="legacy-c c">Search Universe</td>
            </tr>
            <tr>
              <th>
                <select name="type" defaultValue={search.type}>
                  <option value="playername">Player Name</option>
                  <option value="planetname">Planet Name</option>
                  <option value="allytag">Alliance Tag</option>
                  <option value="allyname">Alliance Name</option>
                </select>
                {" \u00a0\u00a0 "}
                <input name="searchtext" type="text" defaultValue={search.text} />
                {" \u00a0\u00a0 "}
                <input type="submit" value="search" />
              </th>
            </tr>
          </tbody>
        </table>
      </form>
      {search.type === "allytag" || search.type === "allyname" ? (
        <AllianceSearchResults rows={search.allianceRows} showEmpty={hasExecutableSearch} />
      ) : (
        <PlayerSearchResults rows={search.playerRows} showEmpty={hasExecutableSearch} />
      )}
    </>
  );
}

function hasExecutableSearchText(text: string): boolean {
  return Array.from(text.trim()).length >= 2;
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
          <td className="legacy-c c" colSpan={6}>
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
          <td className="legacy-c c" />
          <td className="legacy-c c">Name</td>
          <td className="legacy-c c">Alliance</td>
          <td className="legacy-c c">Coords</td>
          <td className="legacy-c c">Status</td>
          <td className="legacy-c c" />
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
          <td className="legacy-c c" colSpan={6}>
            {incoming ? "Request" : "Your requests"}
          </td>
        </tr>
        {buddy.rows.length > 0 ? (
          <>
            <tr>
              <th />
              <th>User</th>
              <th>Alliance</th>
              <th>Coords</th>
              <th>Text</th>
              <th />
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
          <td className="legacy-c c" colSpan={6}>
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
            <td className="legacy-c c" colSpan={2}>
              Buddy request
            </td>
          </tr>
          <tr>
            <th colSpan={2}>Player not found</th>
          </tr>
          <tr>
            <td className="legacy-c c" colSpan={2}>
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
            <td className="legacy-c c" colSpan={2}>
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
            <td className="legacy-c c">
              <a href={buddyURL({})}>back</a>
            </td>
            <td className="legacy-c c">
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
  actionIssue,
  messages,
  onDelete,
  onSend,
  pending
}: {
  actionIssue?: { code: string; message: string };
  messages: GameMessages;
  onDelete: (deleteMode: string, messageIDs: number[], reportIDs: number[]) => void;
  onSend: (targetPlayerID: number, subject: string, text: string) => void;
  pending: boolean;
}) {
  if (messages.action === "compose" && messages.compose) {
    return <MessageComposeTable actionIssue={actionIssue} compose={messages.compose} onSend={onSend} pending={pending} />;
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
            <td className="legacy-c c" colSpan={4}>
              Messages
            </td>
          </tr>
          <tr>
            <th>
              Action
            </th>
            <th>
              Date
            </th>
            <th>
              From
            </th>
            <th>
              Subject
            </th>
          </tr>
          {messages.rows.length > 0 ? (
            messages.rows.map((message) => (
              <React.Fragment key={message.id}>
                <tr data-message-row={message.id}>
                  <th>
                    <input disabled={pending} name={`delmes${message.id}`} type="checkbox" value="on" />
                  </th>
                  <th className={message.unread ? "legacy-message-unread" : undefined}>{formatLegacyMessageDate(message.date)}</th>
                  <th dangerouslySetInnerHTML={{ __html: `${sanitizeLegacyMessageHTML(message.from)} ` }} />
                  <th dangerouslySetInnerHTML={{ __html: `${sanitizeLegacyMessageHTML(message.subject)} ` }} />
                </tr>
                {message.text !== "" ? (
                  <tr>
                    <td className="legacy-b b"> </td>
                    <td
                      className="legacy-b b legacy-message-text"
                      colSpan={3}
                      dangerouslySetInnerHTML={{ __html: sanitizeLegacyMessageHTML(message.text) }}
                    />
                  </tr>
                ) : null}
                {message.reportable ? (
                  <tr>
                    <th colSpan={4}>
                      <input disabled={pending} name={`sneak${message.id}`} type="checkbox" />
                      <input disabled={pending} type="submit" value="Report to operator" />
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
            <th colSpan={4} style={{ padding: "0px 105px" }} />
          </tr>
          <tr>
            <th colSpan={4}>
              <input disabled={pending} name="fullreports" type="checkbox" /> Show intelligence data partially{" "}
            </th>
          </tr>
          <tr>
            <th colSpan={4}>
              <select defaultValue="deletemarked" disabled={pending} name="deletemessages">
                <option value="deletemarked">Delete highlighted messages</option>
                <option value="deletenonmarked">Delete all unselected messages</option>
                <option value="deleteallshown">Delete all displayed messages </option>
                <option value="deleteall">Delete all messages</option>
              </select>
              <input disabled={pending} type="submit" value="ok" />
            </th>
          </tr>
          <tr>
            <td colSpan={4}>
              <center>     </center>
            </td>
          </tr>
          <tr>
            <td className="legacy-c c" colSpan={4}>
              Operators
            </td>
          </tr>
        </tbody>
      </table>
    </form>
  );
}

function MessageComposeTable({
  actionIssue,
  compose,
  onSend,
  pending
}: {
  actionIssue?: { code: string; message: string };
  compose: GameMessageCompose;
  onSend: (targetPlayerID: number, subject: string, text: string) => void;
  pending: boolean;
}) {
  const targetText = `${compose.target.name} [${formatCoordinates(compose.target.coordinates)}]`;
  const submitMessage = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const textArea = event.currentTarget.elements.namedItem("text");
    onSend(compose.target.playerId, String(data.get("betreff") ?? ""), String(data.get("text") ?? ""));
    if (textArea instanceof HTMLTextAreaElement) {
      textArea.value = "";
    }
    if (event.currentTarget.ownerDocument.activeElement instanceof HTMLElement) {
      event.currentTarget.ownerDocument.activeElement.blur();
    }
  };
  return (
    <>
      {actionIssue ? <MessageComposeIssue issue={actionIssue} /> : null}
      <center>
        <form
          action={gameRouteURL("/game/messages", window.location.search)}
          className="legacy-messages-compose-form"
          method="post"
          onSubmit={submitMessage}
        >
          <table className="legacy-overview-table legacy-messages-compose-table" width={519}>
            <tbody>
              <tr>
                <td className="legacy-c c" colSpan={2}>
                  Write message
                </td>
              </tr>
              <tr>
                <th>Recipient</th>
                <th>
                  <input name="to" size={40} type="text" value={targetText} readOnly />
                </th>
              </tr>
              <tr>
                <th>Subject</th>
                <th>
                  <input defaultValue={compose.subject} disabled={pending} maxLength={40} name="betreff" size={40} type="text" />
                </th>
              </tr>
              <tr>
                <th>
                  Message(<span id="cntChars">0</span> / {compose.maxChars} characters)
                </th>
                <th>
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
      </center>
      <br />
      <br />
      <br />
      <br />
    </>
  );
}

function MessageComposeIssue({ issue }: { issue: { code: string; message: string } }) {
  const color = issue.code === "sent" ? "#00FF00" : "#FF0000";
  const breaks = issue.code === "sent" ? "<br>" : "<br><br>";
  return <center dangerouslySetInnerHTML={{ __html: `<font color="${color}">${escapeHTML(issue.message)}</font>${breaks}` }} />;
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

function PhalanxTable({ phalanx }: { phalanx: GamePhalanx }) {
  const source = phalanx.source;
  return (
    <>
      <br />
      <table className="legacy-phalanx-table" width={519}>
        <tbody>
          <tr>
            <td className="c" colSpan={4}>
              {phalanx.reportHeading}{" "}
              <a href={overviewGalaxyHref(source.coordinates)}>[{formatCoordinates(source.coordinates)}]</a> ({phalanx.commander})
            </td>
          </tr>
          <tr>
            <td className="c" colSpan={4}>
              {phalanx.eventsHeading}
            </td>
          </tr>
          {phalanx.actionIssue ? (
            <tr>
              <th colSpan={4}>
                <LegacyFont color="#FF0000">{phalanx.actionIssue.message}</LegacyFont>
              </th>
            </tr>
          ) : (
            <PhalanxEventRows events={phalanx.events} />
          )}
        </tbody>
      </table>
    </>
  );
}

function PhalanxEventRows({ events }: { events: GameFleetMission[] }) {
  const [now, setNow] = React.useState(() => Math.floor(Date.now() / 1000));
  React.useEffect(() => {
    const update = () => setNow(Math.floor(Date.now() / 1000));
    const id = window.setInterval(update, 1000);
    document.addEventListener("visibilitychange", update);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", update);
    };
  }, []);
  if (events.length === 0) {
    return null;
  }
  return (
    <>
      {events.map((event, index) => {
        const remaining = event.arrivalAt - now;
        const groupMissions = overviewEventGroupMissions(event);
        return (
          <tr className={overviewEventRowClass(event)} key={event.id}>
            <th>
              <div id={`bxx${index + 1}`} title={String(Math.max(0, remaining))}>
                {remaining < 0 ? "-" : formatLegacyCountdown(remaining)}
              </div>
            </th>
            <th colSpan={3}>
              {groupMissions.map((groupEvent, groupIndex) => (
                <React.Fragment key={groupEvent.id}>
                  {groupIndex > 0 ? (
                    <>
                      <br />
                      <br />
                    </>
                  ) : null}
                  <span
                    className={`${overviewEventSpanClass(groupEvent)} phalanx_fleet`}
                    dangerouslySetInnerHTML={{ __html: legacyOverviewEventInnerHTML(groupEvent) }}
                  />
                </React.Fragment>
              ))}
            </th>
          </tr>
        );
      })}
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
    oldPassword: string;
    newPassword: string;
    newPasswordRepeat: string;
    email: string;
    vacationMode: boolean;
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
      oldPassword: String(form.get("db_password") ?? ""),
      newPassword: String(form.get("newpass1") ?? ""),
      newPasswordRepeat: String(form.get("newpass2") ?? ""),
      email: String(form.get("db_email") ?? ""),
      vacationMode: form.get("urlaubs_modus") === "on",
      deleteAccount: form.get("db_deaktjava") === "on"
    });
  };

  return (
    <form action={gameRouteURL("/game/options", window.location.search)} method="POST" onSubmit={submitOptions}>
      <table className="legacy-overview-table legacy-options-table" width={519}>
        <tbody>
          <tr>
            <td className="legacy-c c" colSpan={2}>
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
              <input name="db_password" size={20} type="password" />
            </th>
          </tr>
          <tr>
            <th>New password (min. 8 characters)</th>
            <th>
              <input maxLength={40} name="newpass1" size={20} type="password" />
            </th>
          </tr>
          <tr>
            <th>New password (repeat)</th>
            <th>
              <input maxLength={40} name="newpass2" size={20} type="password" />
            </th>
          </tr>
          <tr>
            <th>
              <a title="You can change this email address at any time. This will be entered as a permanent address after 7 days without changes.">
                Email address
              </a>
            </th>
            <th>
              <input defaultValue={options.user.email} maxLength={100} name="db_email" size={20} type="text" />
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
            <td className="legacy-c c" colSpan={2}>
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
            <td className="legacy-c c" colSpan={2}>
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
                <td className="legacy-c c" colSpan={2}>
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
                <td className="legacy-c c" colSpan={2}>
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
                <td className="legacy-c c" colSpan={2}>
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
            <td className="legacy-c c" colSpan={2}>
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
              <input defaultChecked={options.account.vacation} name="urlaubs_modus" type="checkbox" />
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

function legacyAdminHTMLWithSession(value: string): string {
  if (typeof window === "undefined") {
    return value.replaceAll("{PUBLIC_SESSION}", "");
  }
  const session = new URLSearchParams(window.location.search).get("session") ?? "";
  return value.replaceAll("{PUBLIC_SESSION}", session);
}

function escapeHTML(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
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
            <td className="legacy-c c" colSpan={4}>
              Notes
            </td>
          </tr>
          <tr>
            <th colSpan={4}>
              <a href={noteURL({ action: 1 })}>Create a new note</a>
            </th>
          </tr>
          <tr>
            <td className="legacy-c c" />
            <td className="legacy-c c">Date</td>
            <td className="legacy-c c">Subject</td>
            <td className="legacy-c c">Size</td>
          </tr>
          {notes.rows.length > 0 ? (
            notes.rows.map((note) => (
              <tr data-note-row={note.id} key={note.id}>
                <th style={{ width: 20 }}>
                  <input disabled={pending} name={`delmes[${note.id}]`} type="checkbox" value="y" />
                </th>
                <th style={{ width: 150 }}>{formatLegacyAdminDateTime(note.date)}</th>
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
            <td className="legacy-c c" colSpan={2}>
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
            <td className="legacy-c c">
              <a href={noteURL({})}>Back</a>
            </td>
            <td className="legacy-c c">
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

function PlayerSearchResults({ rows, showEmpty }: { rows: GameSearchPlayerRow[]; showEmpty: boolean }) {
  if (rows.length === 0 && !showEmpty) {
    return null;
  }
  return (
    <table className="legacy-overview-table legacy-search-results-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c c">Name</td>
          <td className="legacy-c c">&nbsp;</td>
          <td className="legacy-c c">Alliance</td>
          <td className="legacy-c c">Planet</td>
          <td className="legacy-c c">Position</td>
          <td className="legacy-c c">Place</td>
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
              <a href={gameSearchAllianceHref()} target="_ally">
                {row.alliance?.tag ?? ""}
              </a>
            </th>
            <th>{row.planetName}</th>
            <th>
              <a href={gameSearchGalaxyHref(row.coordinates)}>{formatCoordinates(row.coordinates)}</a>
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

function AllianceSearchResults({ rows, showEmpty }: { rows: GameSearchAllianceRow[]; showEmpty: boolean }) {
  if (rows.length === 0 && !showEmpty) {
    return null;
  }
  return (
    <table className="legacy-overview-table legacy-search-results-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c c">Tag</td>
          <td className="legacy-c c">Name</td>
          <td className="legacy-c c">Member</td>
          <td className="legacy-c c">Points</td>
        </tr>
        {rows.map((row) => (
          <tr data-search-row key={row.allianceId}>
            <th>
              <a href={gameSearchAllianceHref()} target="_ally">
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
  const search = gameSearchBaseParams();
  search.set("messageziel", String(playerID));
  return gameRouteURL("/game/messages", search.toString());
}

function gameSearchBuddyHref(playerID: number): string {
  const search = gameSearchBaseParams();
  search.set("action", "7");
  search.set("buddy_id", String(playerID));
  return gameRouteURL("/game/buddy", search.toString());
}

function gameSearchStatisticsHref(place: number): string {
  const search = gameSearchBaseParams();
  search.set("start", String(Math.floor(place / 100) * 100 + 1));
  return gameRouteURL("/game/statistics", search.toString());
}

function gameSearchGalaxyHref(coordinates: Coordinates): string {
  const search = gameSearchBaseParams();
  search.set("galaxy", String(coordinates.galaxy));
  search.set("system", String(coordinates.system));
  search.set("position", String(coordinates.position));
  return gameRouteURL("/game/galaxy", search.toString());
}

function gameSearchAllianceHref(): string {
  return gameRouteURL("/game/alliance", gameSearchBaseParams().toString());
}

function gameSearchBaseParams(): URLSearchParams {
  const search = new URLSearchParams(window.location.search);
  search.delete("type");
  search.delete("searchtext");
  return search;
}

function TechnologyTable({
  onBuildingAction,
  technology
}: {
  onBuildingAction: (action: "add" | "destroy" | "remove", techID: number, listID?: number) => void;
  technology: GameTechnology;
}) {
  if (technology.info) {
    return <TechnologyInfoTable info={technology.info} />;
  }
  if (technology.details) {
    return <TechnologyDetailsTable details={technology.details} />;
  }
  return <div dangerouslySetInnerHTML={{ __html: technologyTreeHTML(technology) }} />;
}

function ChangelogTable() {
  return <div dangerouslySetInnerHTML={{ __html: changelogHTML() }} />;
}

function changelogHTML(): string {
  let html = "<center>\n";
  html += '  <table width="668">\n';
  html += "   <tr>\n\n";
  html += '    <td class="c">Version</td>\n';
  html += '    <td class="c">Description</td>\n';
  html += "   </tr>\n\n";
  for (const row of legacyChangelogRows) {
    html += "<tr>\n";
    html += `<th>${legacyHTMLText(row.version)}</th>\n`;
    html += `<th style="text-align:left">${row.description}</th>\n`;
    html += "</tr>\n";
  }
  html += "\n</table><br><br><br><br>\n\n";
  html += "</center>";
  return html;
}

function technologyTreeHTML(technology: GameTechnology): string {
  const rows = technology.groups
    .map((group) => {
      const items = group.items
        .map((item) => {
          const details = item.detailsAvailable
            ? `<a href="${legacyHTMLAttribute(technologyDetailURL(item.id))}">[i]</a>`
            : "&nbsp;";
          const requirements = item.requirements
            .map((requirement) => {
              const color = requirement.met ? "#00ff00" : "#ff0000";
              return `<font color="${color}">${legacyHTMLText(requirement.name)} (level ${requirement.level})</font><br /> \n`;
            })
            .join("");
          return `<tr data-technology-row="${item.id}"> \n<td class=l> \n<table width="100%" border=0 cellspacing=0 cellpadding=0><tr><td align=left><a class="legacy-technology-name-link" href="${legacyHTMLAttribute(technologyInfoURL(item.id))}">${legacyHTMLText(item.name)}</a> \n</td><td align=right>${details}</td></tr></table></td> \n<td class=l> \n${requirements}</td> \n`;
        })
        .join("");
      return `<tr><td class=c>${legacyHTMLText(group.name)}</td><td class=c>Requirements</td></tr> \n${items}\n`;
    })
    .join("");
  return `<center> \n<table class="legacy-technology-table" width=470> \n${rows}</table> \n<br><br><br><br>\n`;
}

function TechnologyDetailsTable({ details }: { details: GameTechnologyDetails }) {
  return <div dangerouslySetInnerHTML={{ __html: technologyDetailsHTML(details) }} />;
}

function TechnologyInfoTable({ info }: { info: GameTechnologyInfo }) {
  return <div dangerouslySetInnerHTML={{ __html: technologyInfoHTML(info) }} />;
}

function technologyInfoHTML(info: GameTechnologyInfo): string {
  let html = "<center>\n";
  html += '<table width="519">\n';
  html += `<tr><td class="c">${legacyHTMLText(info.name)}</td></tr>\n`;
  html += "<tr><th><table>\n";
  html += `<tr><td><img border='0' src="${skinBase}/gebaeude/${info.id}.gif" align='top' width='120' height='120'></td>\n`;
  html += `<td>${info.description}</td></tr>\n`;
  html += "</table></th></tr>\n";
  html += technologyInfoRowsHTML(info);
  html += "</table>\n";
  html += "<br><br><br><br>\n";
  html += "</center>";
  return html;
}

function technologyInfoRowsHTML(info: GameTechnologyInfo): string {
  if (info.rows.length === 0) {
    return "";
  }
  if (info.kind === "mine") {
    let html =
      "<tr><th><p><center><table border=1 ><tr><td class='c'>Level</td><td class='c'>Production per hour</td><td class='c'>Difference</td><td class='c'>Energy balance</td><td class='c'>Difference</td> \n";
    for (const row of info.rows) {
      html += `<tr> <th> ${technologyInfoLevelHTML(row)}</th> `;
      html += `<th> ${formatLegacySignedNumber(row.production)}</th> `;
      html += `<th> ${technologyInfoDeltaHTML(row.productionDifference)}</th> `;
      html += `<th> ${formatLegacySignedNumber(row.energy)}</th> `;
      html += `<th> ${technologyInfoDeltaHTML(row.energyDifference)} </th> </tr> \n`;
    }
    html += "</table></center></tr></th>";
    return html;
  }
  if (info.kind === "solar") {
    let html =
      "<tr><th><p><center><table border=1 ><tr><td class='c'>Level</td><td class='c'>Energy balance</td><td class='c'>Difference</td>\n";
    for (const row of info.rows) {
      html += `<tr> <th> ${technologyInfoLevelHTML(row)}</th> `;
      html += `<th> ${formatLegacySignedNumber(row.energy)}</th> `;
      html += `<th> ${technologyInfoDeltaHTML(row.energyDifference)}</th> </tr> \n`;
    }
    html += "</table></center></tr></th>";
    return html;
  }
  if (info.kind === "fusion") {
    let html =
      "<tr><th><p><center><table border=1 ><tr><td class='c'>Level</td><td class='c'>Energy balance</td><td class='c'>Difference</td><td class='c'>Deuterium consumption</td><td class='c'>Difference</td>\n";
    for (const row of info.rows) {
      html += `<tr> <th> ${technologyInfoLevelHTML(row)}</th> `;
      html += `<th> ${formatLegacySignedNumber(row.energy)}</th> `;
      html += `<th> ${technologyInfoDeltaHTML(row.energyDifference)}</th> \n`;
      html += `<th> ${formatLegacySignedNumber(row.deuteriumConsumption)}</th> `;
      html += `<th> ${technologyInfoDeltaHTML(row.deuteriumDifference)}</th> </tr> \n`;
    }
    html += "</table></center></tr></th>";
    return html;
  }
  if (info.kind === "storage") {
    let html =
      "<tr><th><p><center><table border=1 ><tr><td class='c'>Level</td><td class='c'>Capacity</td><td class='c'>Difference</td></tr>\n";
    for (const row of info.rows) {
      const diff =
        row.storageDifference === 0
          ? "0"
          : `<font color="#00FF00">${formatLegacySignedNumber(row.storageDifference)} k</font>`;
      html += `<tr> <th> ${technologyInfoLevelHTML(row)}</th> <th>${formatLegacySignedNumber(row.storage)} k</th> <th>${diff}</th> </tr>\n`;
    }
    html += "</table>";
    return html;
  }
  return "";
}

function technologyInfoLevelHTML(row: GameTechnologyInfoRow): string {
  if (!row.current) {
    return String(row.level);
  }
  return `<font color=#FF0000>${row.level}</font>`;
}

function technologyInfoDeltaHTML(value: number): string {
  const text = formatLegacySignedNumber(value);
  if (value < 0) {
    return `<font color="#FF0000">${text}</font>`;
  }
  if (value > 0) {
    return `<font color="#00FF00">${text}</font>`;
  }
  return text;
}

function technologyDetailsHTML(details: GameTechnologyDetails): string {
  let html = "<center> \n";
  html += '<table class="legacy-technology-details-table" width=270> \n';
  html += "<tr> \n";
  html += "<td class=c align=center nowrap> \n";
  html += `Building conditions for <a href="${legacyHTMLAttribute(technologyInfoURL(details.target.id))}">'${legacyHTMLText(details.target.name)}'</a></td> \n`;
  html += "</tr> \n";
  if (details.levels.length === 0) {
    html += '<tr data-technology-detail-row="none"><td class=l align=center>No conditions</td></tr> ';
  }
  for (const level of details.levels) {
    html += `<tr><td class=c>${level.step}</td></tr>`;
    for (const requirement of level.requirements) {
      const color = requirement.met ? "#00ff00" : "#ff0000";
      html += `<tr data-technology-detail-row="${requirement.id}">\n`;
      html += "    <td class=l align=center> \n";
      html += '    <table width="100%" border=0> \n';
      html += "    <tr> \n";
      html += `        <td align=left> <font color="${color}"> ${legacyHTMLText(requirement.name)} (level ${requirement.level}) </font> </td> \n`;
      html += `        <td align=right> <a href="${legacyHTMLAttribute(technologyDetailURL(requirement.id))}">[i]</a> </td> \n`;
      html += "    </tr> \n";
      html += "    </td> \n";
      html += "    </table> \n";
      html += "</tr>";
    }
  }
  html += "</table> \n";
  html += "</center>";
  html += "<br><br><br><br>\n";
  return html;
}

function legacyFont(color: string, children: React.ReactNode): React.ReactElement {
  return React.createElement("font", { color }, children);
}

const resourceColumns: { key: keyof Pick<ResourceProductionValues, "metal" | "crystal" | "deuterium" | "energy">; label: string }[] = [
  { key: "metal", label: "Metal" },
  { key: "crystal", label: "Crystal" },
  { key: "deuterium", label: "Deuterium" },
  { key: "energy", label: "Energy" }
];

const empireWidth75Attrs = { width: 75 } as React.ThHTMLAttributes<HTMLTableCellElement>;
const empireRowHeight20Attrs = { height: 20 } as React.HTMLAttributes<HTMLTableRowElement>;
const empireRowHeight75Attrs = { height: 75 } as React.HTMLAttributes<HTMLTableRowElement>;

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
          <tr {...empireRowHeight20Attrs}>
            <td className="legacy-c c" colSpan={colSpan}>
              Empire Overview
            </td>
          </tr>
          {empire.moonEnabled && empire.hasMoons ? (
            <tr {...empireRowHeight20Attrs}>
              <th colSpan={Math.ceil(planets.length / 2)}>
                <a href={empirePlanetTypeURL(1)}>Planets</a>
              </th>
              <th colSpan={Math.ceil(planets.length / 2) + (1 - (planets.length % 2))}>
                <a href={empirePlanetTypeURL(3)}>Moons</a>
              </th>
              <th>&nbsp;</th>
            </tr>
          ) : null}
          <tr {...empireRowHeight75Attrs}>
            <th {...empireWidth75Attrs}></th>
            {planets.map((planet) => (
              <th
                key={planet.id}
                dangerouslySetInnerHTML={{ __html: empirePlanetImageCellHTML(planet) }}
                style={{ padding: 20 }}
              />
            ))}
            <th {...empireWidth75Attrs}>Sum</th>
          </tr>
          <tr {...empireRowHeight20Attrs}>
            <th {...empireWidth75Attrs}>Name</th>
            {planets.map((planet) => (
              <th key={planet.id} {...empireWidth75Attrs}>
                {planet.name}
              </th>
            ))}
            <th {...empireWidth75Attrs}>&nbsp;</th>
          </tr>
          <tr {...empireRowHeight20Attrs}>
            <th {...empireWidth75Attrs}>Coordinates</th>
            {planets.map((planet) => (
              <th key={planet.id} {...empireWidth75Attrs}>
                <a href={galaxyHref(planet.coordinates)}>[{formatCoordinates(planet.coordinates)}]</a>
              </th>
            ))}
            <th {...empireWidth75Attrs}>&nbsp;</th>
          </tr>
          <tr {...empireRowHeight20Attrs}>
            <th {...empireWidth75Attrs}>Fields</th>
            {planets.map((planet) => (
              <th key={planet.id} {...empireWidth75Attrs}>
                {planet.fields}/{planet.maxFields}
              </th>
            ))}
            <th
              {...empireWidth75Attrs}
              dangerouslySetInnerHTML={{
                __html: empireFieldsSummaryHTML(sumFields, avgFields, sumMaxFields, avgMaxFields)
              }}
            />
          </tr>
          <EmpireSectionTitle colSpan={colSpan} title="Resources" />
          {empire.resources.map((row) => (
            <EmpireResourceRow key={row.id} planets={planets} row={row} />
          ))}
          <EmpireSectionTitle colSpan={colSpan} title="Buildings" />
          {empire.buildings.map((row) => (
            <EmpireLevelRow key={row.id} planets={planets} row={row} showAverage />
          ))}
          <EmpireSectionTitle colSpan={colSpan} title="Research" />
          {empire.research.map((row) => (
            <EmpireLevelRow key={row.id} planets={planets} row={row} />
          ))}
          <EmpireSectionTitle colSpan={colSpan} title="Ships" />
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
    <tr {...empireRowHeight20Attrs}>
      <td align="left" className="legacy-c c" colSpan={colSpan}>
        {title}
      </td>
    </tr>
  );
}

function EmpireResourceRow({ planets, row }: { planets: GameEmpirePlanet[]; row: GameEmpireResourceRow }) {
  const energy = row.id === 703;
  return (
    <tr data-empire-resource-row={row.id} {...empireRowHeight20Attrs}>
      <th {...empireWidth75Attrs}>{row.name}</th>
      {planets.map((planet) => {
        const value = empireResourceValue(row, planet.id);
        return (
          <th
            key={planet.id}
            {...empireWidth75Attrs}
            {...(energy
              ? {
                  dangerouslySetInnerHTML: {
                    __html: empireEnergyCellHTML(value.amount, value.production)
                  }
                }
              : {})}
          >
            {energy ? null : (
              <>
                <a href={gameRouteURL("/game/resources", withPlanetSearch(planet.id))}>{formatLegacyPlainNumber(value.amount)}</a> /{" "}
                {formatLegacyPlainNumber(value.production)}
              </>
            )}
          </th>
        );
      })}
      <th
        {...empireWidth75Attrs}
        {...(energy
          ? {
              dangerouslySetInnerHTML: {
                __html: empireEnergyCellHTML(row.total, row.production)
              }
            }
          : {})}
      >
        {energy ? null : (
          <>
            {formatLegacyPlainNumber(row.total)}
            {"\u00a0/\u00a0"}
            {formatLegacyPlainNumber(row.production)}
          </>
        )}
      </th>
    </tr>
  );
}

function empireFieldsSummaryHTML(sumFields: number, avgFields: number, sumMaxFields: number, avgMaxFields: number) {
  const averageLink = (value: number) =>
    `<a href='#' onMouseOver="return overlib('<font color=white>Average per planet</font>');" onMouseOut="return nd();">(${formatLegacyNumber(
      value
    )})</a>`;
  return `${formatLegacyNumber(sumFields)}&nbsp;${averageLink(avgFields)}&nbsp;/&nbsp;${formatLegacyNumber(
    sumMaxFields
  )}&nbsp;${averageLink(avgMaxFields)}`;
}

function empirePlanetImageCellHTML(planet: GameEmpirePlanet) {
  return [
    "  ",
    `                    <a href="${legacyHTMLAttribute(planetHref(planet.id))}">`,
    `                        <img src="${legacyHTMLAttribute(planetImagePath(planet, false))}" width="75" height="71" border="0">`,
    "                    </a>",
    "            "
  ].join("\n");
}

function empireEnergyCellHTML(amount: number, production: number) {
  const amountText = formatLegacyPlainNumber(amount);
  const amountHTML = amount < 0 ? `<font color="red">${amountText}</font>` : amountText;
  return `${amountHTML} / ${formatLegacyPlainNumber(production)} `;
}

function EmpireLevelRow({ planets, row, showAverage = false }: { planets: GameEmpirePlanet[]; row: GameEmpireLevelRow; showAverage?: boolean }) {
  return (
    <tr data-empire-level-row={row.id} {...empireRowHeight20Attrs}>
      <th {...empireWidth75Attrs}>
        <a href={technologyInfoURL(row.id)}>{row.name}</a>
      </th>
      {planets.map((planet) => {
        const value = empireLevelValue(row, planet.id);
        return (
          <th key={planet.id} {...empireWidth75Attrs}>
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
      <th {...empireWidth75Attrs}>
        {formatLegacyPlainNumber(row.total)}
        {showAverage ? ` (${formatEmpireAverage(row.average)})` : ""}
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
    <tr data-empire-count-row={row.id} {...empireRowHeight20Attrs}>
      <th {...empireWidth75Attrs}>
        <a href={technologyInfoURL(row.id)}>{row.name}</a>
      </th>
      {planets.map((planet) => {
        const value = empireCountValue(row, planet.id);
        return (
          <th key={planet.id} {...empireWidth75Attrs}>
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
      <th {...empireWidth75Attrs}>{formatLegacyPlainNumber(row.total)}</th>
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
  onSubmit
}: {
  resources: GameResourceProduction;
  pending: boolean;
  onSubmit: (production: Record<string, string>) => void;
}) {
  return (
    <form
      action={gameRouteURL("/game/resources", window.location.search)}
      className="legacy-resources-form"
      id="ressourcen"
      method="post"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(resourceProductionFormValues(event.currentTarget));
      }}
    >
      <input id="screen" name="screen" type="hidden" />
      <div className="legacy-center">
        <br />
        <br />
        Production factor: {formatProductionFactor(resources.factor)}
        <table
          className="legacy-overview-table legacy-resources-table"
          dangerouslySetInnerHTML={{ __html: legacyResourcesTableHTML(resources) }}
          width={550}
        />
        <br />
        <br />
        <br />
        <br />
      </div>
    </form>
  );
}

function legacyResourcesTableHTML(resources: GameResourceProduction): string {
  let html = "";
  html += "  <tr> \n";
  html += '    <td class="c" colspan="6"> \n';
  html += `    Resource settings on planet &quot;${escapeLegacyHTML(resources.currentPlanet.name)}&quot;\n`;
  html += "    </td> \n";
  html += "  </tr>\n";

  html += "  <tr> \n";
  html += '   <th colspan="2"></th>';
  for (const column of resourceColumns) {
    html += `    <th>${escapeLegacyHTML(column.label)}</th>`;
  }
  html += "</th> \n";
  html += "  </tr>\n";

  html += "  <tr> \n";
  html += '   <th colspan="2">Basic Income</th> \n';
  for (const column of resourceColumns) {
    html += `    <td class="k">${formatLegacyRawInteger(resourceValue(resources.natural, column.key))}</td>\n`;
  }
  html += "  </tr>\n";

  for (const row of resources.rows) {
    html += "  <tr> \n";
    const label = row.id === 212 ? `${row.level} available` : `Level ${row.level}`;
    html += `<th>${escapeLegacyHTML(row.name)} (${escapeLegacyHTML(label)})</th>`;
    html += legacyResourceBonusCellHTML(row);
    html += "\n";
    for (const column of resourceColumns) {
      html += legacyResourceProductionCellHTML(column.key, row.values);
    }
    html += " \n";
    html += legacyProductionSelectHTML(row.id, row.percent);
    html += "  </tr>\n";
  }

  html += "    <tr>   <tr> \n";
  html += '    <th colspan="2">Storage capacity</th> \n';
  for (const column of resourceColumns) {
    html += `    <td class="k"><font color="#00ff00">${escapeLegacyHTML(formatStorageValue(resources.storage, column.key))}</font></td> \n`;
  }
  html += '    <td class="k"> \n';
  html += '    <input type="submit" name="action" value="Recalculate"></td> \n';
  html += "  </tr> \n";
  html += '  <tr>     <th colspan="6" height="4"></th>   </tr> \n';
  html += legacyResourceTotalRowHTML("Total per hour:", resources.totals.hour);
  html += legacyResourceTotalRowHTML("Total per day:", resources.totals.day);
  html += legacyResourceTotalRowHTML("Total per week:", resources.totals.week);
  return html;
}

function legacyResourceBonusCellHTML(row: ResourceProductionRow): string {
  const iconByImage = new Map((row.bonusIcons ?? []).map((icon) => [icon.image, icon]));
  const slots: (ResourceProductionBonusIcon | undefined)[] =
    row.id === 1 || row.id === 2 || row.id === 3
      ? [iconByImage.get("geologe_ikon.gif"), undefined]
      : row.id === 12
        ? [undefined, iconByImage.get("ingenieur_ikon.gif")]
        : row.id === 4 || row.id === 212
          ? [iconByImage.get("ingenieur_ikon.gif")]
          : [undefined];
  return `<th>${slots.map((slot) => (slot ? legacyResourceBonusIconHTML(slot) : "&nbsp;")).join("")}</th>`;
}

function legacyResourceBonusIconHTML(icon: ResourceProductionBonusIcon): string {
  return `<img border="0" src="${escapeLegacyAttribute(`${gameImageBase}/${icon.image}`)}" alt="${escapeLegacyAttribute(icon.alt)}" width="20" height="20">`;
}

function legacyResourceProductionCellHTML(
  column: keyof Pick<ResourceProductionValues, "metal" | "crystal" | "deuterium" | "energy">,
  values: ResourceProductionValues
): string {
  const value = resourceValue(values, column);
  const raw = column === "energy" ? values.energyRaw : value;
  const text =
    column === "energy" && raw <= 0
      ? `${formatLegacyPlainNumber(Math.abs(value))}/${formatLegacyPlainNumber(Math.abs(raw))}`
      : formatLegacySignedNumber(value);
  const color = raw > 0 || value > 0 ? "#00FF00" : raw < 0 || value < 0 ? "#FF0000" : "#FFFFFF";
  return `   <th><font color="${color}">${escapeLegacyHTML(text)}</font></th>\n`;
}

function legacyProductionSelectHTML(rowID: number, selectedPercent: number): string {
  let html = `  <th> <select name="last${rowID}" size="1">\n`;
  for (const percent of productionPercentOptions()) {
    html += `      <option value="${percent}" ${percent === selectedPercent ? "selected" : ""}>${percent}%</option>\n`;
  }
  html += "        </select>\n";
  html += "   </th>\n";
  return html;
}

function legacyResourceTotalRowHTML(label: string, values: ResourceProductionValues): string {
  let html = "  <tr> \n";
  html += `    <th colspan="2">${escapeLegacyHTML(label)}</th> \n`;
  for (const column of resourceColumns) {
    const value = resourceValue(values, column.key);
    const color = value > 0 ? "#00ff00" : "#ff0000";
    html += `    <td class="k"><font color="${color}">${escapeLegacyHTML(formatLegacySignedNumber(value))}</font></td> \n`;
  }
  html += "  </tr> \n";
  return html;
}

function escapeLegacyHTML(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function escapeLegacyAttribute(value: string): string {
  return escapeLegacyHTML(value);
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

function OverviewPage({ overview, onBuildQueueComplete }: { overview: GameOverview; onBuildQueueComplete: () => void }) {
  return (
    <>
      {overview.news ? <OverviewNewsBox news={overview.news} /> : null}
      <OverviewTable onBuildQueueComplete={onBuildQueueComplete} overview={overview} />
    </>
  );
}

function OverviewNewsBox({ news }: { news: GameOverviewNews }) {
  return (
    <div id="combox_container">
      <a href={news.url} id="combox" target="_blank" rel="noreferrer">
        <div id="anfang">{news.start}</div>
        <div id="ende">{news.end}</div>
      </a>
    </div>
  );
}

function OverviewTable({ overview, onBuildQueueComplete }: { overview: GameOverview; onBuildQueueComplete: () => void }) {
  const planet = overview.currentPlanet;
  const planetTitle =
    planet.type === 0
      ? `Moon "${planet.name}" at orbit of [${formatCoordinates(planet.coordinates)}]`
      : `Planet "${planet.name}"`;
  const moon =
    planet.type === 0
      ? undefined
      : overview.planetSwitcher.find(
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
          <td className="legacy-c c" colSpan={4}>
            <a href={overviewRenameHref(planet.id)} title="Planet menu">
              {planetTitle}
            </a>
            {`     (${overview.commander})`}
          </td>
        </tr>
        {overview.unreadMessages > 0 ? (
          <tr>
            <th colSpan={4}>
              <a href={overviewMessagesHref()}>
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
          <td className="legacy-c c" colSpan={4}>
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
                {"\n"}
              </>
            ) : null}
          </th>
          <th colSpan={2}>
            <img alt="" height={200} src={planetImagePath(planet, false)} width={200} />
            <br />
            <LegacyCenter>
              <OverviewBuildQueue includeLevel={true} onComplete={onBuildQueueComplete} queue={planet.buildQueue} />
            </LegacyCenter>
            <br />
          </th>
          <th className="legacy-s s">
            <table className="legacy-planet-list s">
              <tbody>
                {otherPlanets.length === 0
                  ? null
                  : rowsOfTwo(otherPlanets).map((row, index) => (
                    <tr key={index}>
                      {row.map((item) => (
                        <th key={item.id}>
                          {item.name}
                          <br />
                          {" "}
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
                          {" "}
                          <LegacyCenter>
                            <OverviewBuildQueue includeLevel={false} onComplete={onBuildQueueComplete} queue={item.buildQueue} />
                          </LegacyCenter>
                        </th>
                      ))}
                    </tr>
                  ))}
                <tr />
              </tbody>
            </table>
          </th>
        </tr>
        <tr>
          <th> Diameter</th>
          <th colSpan={3}>
            {`${formatLegacyNumber(planet.diameter)} км     (`}
            <a title="Developed fields">{`${planet.fields} `}</a>
            {" / "}
            <a title="max. developed fields">{`${planet.maxFields} `}</a>
            {" fields)   "}
          </th>
        </tr>
        <tr>
          <th> Temperature </th>
          <th colSpan={3}>
            {`approx. ${planet.temperature}°C to ${planet.temperature + 40}°C`}
          </th>
        </tr>
        <tr>
          <th> Position</th>
          <th colSpan={3}>
            <a className="legacy-overview-position-link" href={overviewGalaxyHref(planet.coordinates)}>
              [{formatCoordinates(planet.coordinates)}]
            </a>
          </th>
        </tr>
        <tr>
          <th> Points</th>
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
  const [now, setNow] = React.useState(() => Math.floor(Date.now() / 1000));
  React.useEffect(() => {
    const update = () => setNow(Math.floor(Date.now() / 1000));
    const id = window.setInterval(update, 1000);
    document.addEventListener("visibilitychange", update);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", update);
    };
  }, []);
  return (
    <>
      {events.map((event, index) => {
        const remaining = event.arrivalAt - now;
        const groupMissions = overviewEventGroupMissions(event);
        return (
          <tr className={overviewEventRowClass(event)} key={event.id}>
            <th>
              <div
                className="legacy-overview-event-timer"
                data-time={String(event.arrivalAt)}
                id={`bxx${index + 1}`}
                title={String(Math.max(0, remaining))}
              >
                {remaining < 0 ? "-" : formatLegacyCountdown(remaining)}
              </div>
              {"\n"}
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
                  <span
                    className={overviewEventSpanClass(groupEvent)}
                    dangerouslySetInnerHTML={{ __html: legacyOverviewEventInnerHTML(groupEvent) }}
                  />
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

function overviewEventSpanClass(event: GameFleetMission): string {
  const baseMission = overviewEventBaseMission(event.mission);
  const returning = event.mission >= 100 && event.mission < 200;
  const holding = event.mission >= 200;
  const own = event.own !== false;
  switch (baseMission) {
    case 1:
      if (returning) return "return ownattack";
      return own ? "flight ownattack" : "attack";
    case 2:
      if (returning) return "return ownfederation";
      if (!own && event.unionId > 0) return "ownfederation";
      return "federation";
    case 3:
      return `${returning ? "return" : "flight"} ${own ? "owntransport" : "transport"}`;
    case 4:
      return `${returning ? "return" : "flight"} owndeploy`;
    case 5:
      if (holding) return `holding ${own ? "ownhold" : "hold"}`;
      if (returning) return "return ownhold";
      return `flight ${own ? "ownhold" : "hold"}`;
    case 6:
      return `${returning ? "return" : "flight"} ${own ? "ownespionage" : "espionage"}`;
    case 7:
      return `${returning ? "return" : "flight"} owncolony`;
    case 8:
      return `${returning ? "return" : "flight"} ownharvest`;
    case 9:
      return `${returning ? "return" : "flight"} ${own ? "owndestroy" : "destroy"}`;
    case 15:
      return `${holding ? "holding" : returning ? "return" : "flight"} owntransport`;
    case 20:
      return own ? "ownmissile" : "missile";
    case 21:
      if (returning) return "return ownattack";
      if (!own && event.unionId > 0) return "ownattack";
      return "attack";
    default:
      return `${holding ? "holding" : returning ? "return" : "flight"} ${own ? "owntransport" : "transport"}`;
  }
}

function overviewEventEndpointName(name: string): string {
  const normalized = name.trim();
  return normalized && normalized.toLowerCase() !== "space" ? normalized : "";
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
    case 15:
      return "ownexpedition";
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

function overviewEventShipTitle(event: GameFleetMission): string {
  return event.ships.map((ship) => `${ship.name} ${formatLegacyNumber(ship.count)}`).join(" ");
}

function legacyOverviewEventInnerHTML(event: GameFleetMission): string {
  const baseMission = overviewEventBaseMission(event.mission);
  const missionClass = overviewEventMissionClass(event);
  const shipAnchor = legacyOverviewFleetAnchor(event, event.own === false ? 1 : 0, missionClass);
  const shipTitle = legacyOverviewFleetTitle(event, event.own === false ? 1 : 0);
  const returnMission = event.mission >= 100 && event.mission < 200;
  const originType = returnMission ? event.targetType : 1;
  const targetType = returnMission ? 1 : event.targetType;
  const originFrom = legacyOverviewPlanetFrom(event.origin, event.originName, missionClass, originType);
  const targetTo = legacyOverviewPlanetTo(event.target, event.targetName, missionClass, targetType);
  const targetFrom = legacyOverviewPlanetFrom(event.target, event.targetName, missionClass, targetType);
  const missionText = legacyOverviewMissionText(baseMission);

  if (baseMission === 1 || baseMission === 21) {
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromReturnTo(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    if (event.own === false) {
      return `Battle ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewPlayerFromTo(
        legacyOverviewPlayerDetails(event),
        originFrom,
        targetTo
      )}. Mission: ${missionText}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 2) {
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromReturnTo(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    if (event.own === false) {
      return `Alliance ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewPlayerFromTo(
        legacyOverviewPlayerDetails(event),
        originFrom,
        targetTo
      )}. Mission: ${missionText}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 3) {
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromReturnTo(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    if (event.own === false) {
      return `Peaceful ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewPlayerFromTo(
        legacyOverviewPlayerDetails(event),
        originFrom,
        targetTo
      )}. Mission: ${missionText}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 4) {
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 5) {
    if (event.mission >= 200) {
      if (event.own === false) {
        return `${legacyOverviewPlayerDetails(event)} holds the alliance's ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(
          shipTitle
        )}"></a>${legacyOverviewFromToOrbit(originFrom, targetFrom)}. Mission: ${missionText}`;
      }
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewHoldFromOnto(
        originFrom,
        targetFrom
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromReturnTo(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    if (event.own === false) {
      return `Peaceful ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewPlayerFromTo(
        legacyOverviewPlayerDetails(event),
        originFrom,
        targetTo
      )}. Mission: <span class='ownclass'>${missionText}</span>`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 6) {
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromReturnTo(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    if (event.own === false) {
      return `Battle ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewPlayerFromTo(
        legacyOverviewPlayerDetails(event),
        originFrom,
        targetTo
      )}. Mission: ${missionText}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 7) {
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewColonyReturn(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewColonyFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 8) {
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 9) {
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromReturnTo(
        originFrom,
        targetTo
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    if (event.own === false) {
      return `Battle ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewPlayerFromTo(
        legacyOverviewPlayerDetails(event),
        originFrom,
        targetTo
      )}. Mission: ${missionText}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 15) {
    if (event.mission >= 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewExpeditionHolding(
        originFrom,
        targetFrom
      )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
    }
    if (event.mission >= 100 && event.mission < 200) {
      return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewExpeditionReturn(
        targetTo,
        legacyOverviewCargoHTML(event, missionClass, missionText)
      )}`;
    }
    return `Your ${shipAnchor}fleet</a><a href="#" title="${escapeLegacyAttribute(shipTitle)}"></a>${legacyOverviewExpeditionFromTo(
      originFrom,
      targetTo
    )}. Mission: ${legacyOverviewCargoHTML(event, missionClass, missionText)}`;
  }

  if (baseMission === 20) {
    const origin = legacyOverviewPlanetFrom(event.origin, event.originName, "", 1);
    const target = legacyOverviewPlanetTo(event.target, event.targetName, event.own === false ? "" : "own", event.targetType);
    const targetText =
      event.missileTargetId > 0
        ? ` Main target ${escapeLegacyHTML(event.missileTarget || `NAME_${event.missileTargetId}`)}`
        : "";
    return `Rocket Attack (${formatLegacyNumber(event.missileAmount)}) from ${origin} to ${target} ${targetText}`.trim();
  }

  return `Mission Type:${event.mission}, Fleet: ${escapeLegacyAttribute(shipTitle)}, from ${originFrom} to ${targetTo}, ${legacyOverviewCargoHTML(
    event,
    missionClass,
    "Cargo"
  )}`;
}

function legacyOverviewFleetAnchor(event: GameFleetMission, summary: 0 | 1, className: string): string {
  void event;
  void summary;
  return `<a href="#" class="${escapeLegacyAttribute(className)}">`;
}

function legacyOverviewFleetTitle(event: GameFleetMission, summary: 0 | 1): string {
  const parts: string[] = [];
  if (summary) {
    parts.push(`Number of ships: ${formatLegacyNumber(event.totalShips)}`);
  }
  const shipTitle = overviewEventShipTitle(event);
  if (shipTitle) {
    parts.push(shipTitle);
  }
  return parts.join(" ");
}

function legacyOverviewPlayerDetails(event: GameFleetMission): string {
  const owner = event.ownerName.trim() || event.targetOwnerName.trim() || "Enemy";
  const writeTitle = "Write message";
  const messageHref = event.ownerId > 0 ? gameMessageComposeURL(event.ownerId, window.location.search) : "#";
  return `${escapeLegacyHTML(owner)} <a href="${legacyHTMLAttribute(messageHref)}"><img src="${skinBase}/img/m.gif" title="${writeTitle}" alt="${writeTitle}"></a>`;
}

function legacyOverviewFromTo(origin: string, target: string): string {
  return ` from ${origin} sent to ${target}`;
}

function legacyOverviewFromReturnTo(origin: string, target: string): string {
  return `, sent from ${origin} is returned to ${target}`;
}

function legacyOverviewPlayerFromTo(player: string, origin: string, target: string): string {
  return ` player ${player} from ${origin} has been sent to ${target}`;
}

function legacyOverviewHoldFromOnto(origin: string, target: string): string {
  return `, sent from ${origin} is in orbit ${target}.`;
}

function legacyOverviewFromToOrbit(origin: string, target: string): string {
  return ` from the ${origin} in orbit ${target}`;
}

function legacyOverviewColonyFromTo(origin: string, target: string): string {
  return ` from ${origin} sent to position ${target}`;
}

function legacyOverviewColonyReturn(origin: string, target: string): string {
  return `, sent from position ${origin} is returned to ${target}`;
}

function legacyOverviewExpeditionFromTo(origin: string, target: string): string {
  return ` sent from ${origin} reaches position ${target}`;
}

function legacyOverviewExpeditionReturn(target: string, cargo: string): string {
  return ` returns to ${target} after order ${cargo}`;
}

function legacyOverviewExpeditionHolding(origin: string, target: string): string {
  return `, sent from ${origin} is investigating position ${target}`;
}

function legacyOverviewMissionText(baseMission: number): string {
  switch (baseMission) {
    case 1:
    case 21:
      return "Attack";
    case 2:
      return "ACS Attack";
    case 3:
      return "Transport";
    case 4:
      return "Deploy";
    case 5:
      return "ACS Hold";
    case 6:
      return "Espionage";
    case 7:
      return "Colonize";
    case 8:
      return "Recycle";
    case 9:
      return "Destroy";
    case 15:
      return "Expedition";
    case 20:
      return "Missile Attack";
    default:
      return "Custom task";
  }
}

function legacyOverviewPlanetFrom(coordinates: Coordinates, name: string, missionClass: string, planetType = 1): string {
  const label = overviewEventEndpointName(name);
  const href = legacyHTMLAttribute(overviewGalaxyHref(coordinates));
  if (legacyOverviewSpecialTargetType(planetType)) {
    return ` <a href="${href}" ${escapeLegacyAttribute(missionClass)}>[${formatCoordinates(coordinates)}]</a>`;
  }
  const prefix = legacyOverviewGamePlanetType(planetType) ? "planets " : "";
  return `${label ? `${prefix}${escapeLegacyHTML(label)} ` : ""}<a href="${href}" ${escapeLegacyAttribute(missionClass)}>[${formatCoordinates(coordinates)}]</a>`;
}

function legacyOverviewPlanetTo(coordinates: Coordinates, name: string, missionClass: string, planetType = 1): string {
  const label = overviewEventEndpointName(name);
  const href = legacyHTMLAttribute(overviewGalaxyHref(coordinates));
  if (legacyOverviewSpecialTargetType(planetType)) {
    return ` <a href="${href}" ${escapeLegacyAttribute(missionClass)}>[${formatCoordinates(coordinates)}]</a>`;
  }
  const prefix = legacyOverviewGamePlanetType(planetType) ? "planet " : "";
  return `${label ? `${prefix}${escapeLegacyHTML(label)} ` : ""}<a href="${href}" ${escapeLegacyAttribute(missionClass)}>[${formatCoordinates(coordinates)}]</a>`;
}

function legacyOverviewSpecialTargetType(planetType: number): boolean {
  return planetType === 10002 || planetType === 20000;
}

function legacyOverviewGamePlanetType(planetType: number): boolean {
  return planetType > 0 && planetType !== 10000;
}

function legacyOverviewEndpointHTML(coordinates: Coordinates, name: string): string {
  const label = overviewEventEndpointName(name);
  return `${label ? `${escapeLegacyHTML(label)} ` : ""}<a href="${escapeLegacyAttribute(galaxyHref(coordinates))}">[${formatCoordinates(coordinates)}]</a>`;
}

function legacyOverviewCargoHTML(event: GameFleetMission, missionClass: string, text: string): string {
  const escapedText = escapeLegacyHTML(text);
  const resources = [
    { id: 700, name: "Metal" },
    { id: 701, name: "Crystal" },
    { id: 702, name: "Deuterium" }
  ].map((resource) => ({
    ...resource,
    amount: Math.max(0, Math.trunc(Number(event.loadedResources?.[String(resource.id)] ?? 0)))
  }));
  const total = resources.reduce((sum, resource) => sum + resource.amount, 0);
  if (total <= 0) {
    return `<span class='class'>${escapedText}</span>`;
  }
  const title = `Transport:${resources.map((resource) => ` ${resource.name}: ${formatLegacyNumber(resource.amount)}`).join("")}`;
  return `<a href="#" class="${escapeLegacyAttribute(missionClass)}">${escapedText}</a><a href="#" title="${escapeLegacyAttribute(
    title
  )}"></a>`;
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
    <LegacyCenter>
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
                <td className="legacy-c c" colSpan={3}>
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
    </LegacyCenter>
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
    <LegacyCenter>
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
                <td className="legacy-c c" colSpan={3}>
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
    </LegacyCenter>
  );
}

function LegacyMessage({ tone, text }: { tone: "error" | "neutral"; text: string }) {
  return (
    <table className="legacy-overview-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c c">{tone === "error" ? "Error" : "Overview"}</td>
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

function overviewSessionSearch(): URLSearchParams {
  const current = new URLSearchParams(window.location.search);
  const search = new URLSearchParams();
  const session = current.get("session");
  if (session) {
    search.set("session", session);
  }
  return search;
}

function overviewRenameHref(planetID: number): string {
  const search = overviewSessionSearch();
  search.set("pl", String(planetID));
  return gameRouteURL("/game/rename-planet", search.toString());
}

function overviewMessagesHref(): string {
  const search = overviewSessionSearch();
  search.set("dsp", "1");
  return gameRouteURL("/game/messages", search.toString());
}

function overviewRankHref(rank: number): string {
  const search = overviewSessionSearch();
  const start = Math.floor(Math.max(0, rank) / 100) * 100 + 1;
  search.set("start", String(start));
  return gameRouteURL("/game/statistics", search.toString());
}

function overviewGalaxyHref(coordinates: Coordinates): string {
  const search = overviewSessionSearch();
  search.set("galaxy", String(coordinates.galaxy));
  search.set("system", String(coordinates.system));
  search.set("position", String(coordinates.position));
  return gameRouteURL("/game/galaxy", search.toString());
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

function formatLegacyCountdown(totalSeconds: number): string {
  const safe = Math.max(0, Math.floor(totalSeconds));
  const hours = Math.floor(safe / 3600);
  const minutes = Math.floor(safe / 60) % 60;
  const seconds = safe % 60;
  return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
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

function formatLegacyServerDateTime(seconds: number): string {
  return formatLegacyDateTime(seconds + 3 * 60 * 60);
}

function formatLegacyAdminDateTime(seconds: number): string {
  return formatLegacyServerDateTime(seconds);
}

function formatLegacyAdminFleetLogDateParts(seconds: number): { date: string; time: string } {
  const date = new Date((seconds + 3 * 60 * 60) * 1000);
  return {
    date: `${String(date.getUTCDate()).padStart(2, "0")}.${String(date.getUTCMonth() + 1).padStart(2, "0")}.${date.getUTCFullYear()}`,
    time: `${String(date.getUTCHours()).padStart(2, "0")}:${String(date.getUTCMinutes()).padStart(2, "0")}:${String(date.getUTCSeconds()).padStart(2, "0")}`,
  };
}

function formatLegacyMessageDate(seconds: number): string {
  const date = new Date((seconds + 3 * 60 * 60) * 1000);
  return `${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")} ${String(
    date.getUTCHours()
  ).padStart(2, "0")}:${String(date.getUTCMinutes()).padStart(2, "0")}:${String(date.getUTCSeconds()).padStart(2, "0")}`;
}

function formatLegacyAdminMessageDate(seconds: number): string {
  const date = new Date((seconds + 3 * 60 * 60) * 1000);
  return `${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")} ${String(
    date.getUTCHours()
  ).padStart(2, "0")}:${String(date.getUTCMinutes()).padStart(2, "0")}:${String(date.getUTCSeconds()).padStart(2, "0")}`;
}

function formatLegacyAdminUserLogDate(seconds: number): string {
  const date = new Date((seconds + 3 * 60 * 60) * 1000);
  return `${String(date.getUTCDate()).padStart(2, "0")}.${String(date.getUTCMonth() + 1).padStart(2, "0")}.${date.getUTCFullYear()} ${String(
    date.getUTCHours()
  ).padStart(2, "0")}:${String(date.getUTCMinutes()).padStart(2, "0")}:${String(date.getUTCSeconds()).padStart(2, "0")}`;
}

function formatLegacyAdminQueueDate(seconds: number): string {
  const date = new Date((seconds + 3 * 60 * 60) * 1000);
  return `${String(date.getUTCDate()).padStart(2, "0")}.${String(date.getUTCMonth() + 1).padStart(2, "0")}.${date.getUTCFullYear()} ${String(
    date.getUTCHours()
  ).padStart(2, "0")}:${String(date.getUTCMinutes()).padStart(2, "0")}:${String(date.getUTCSeconds()).padStart(2, "0")}`;
}

function formatLegacyAdminBattleReportDate(seconds: number): string {
  const date = new Date((seconds + 3 * 60 * 60) * 1000);
  return `${date.getUTCFullYear()}.${String(date.getUTCMonth() + 1).padStart(2, "0")}.${String(date.getUTCDate()).padStart(2, "0")} ${String(
    date.getUTCHours()
  ).padStart(2, "0")}:${String(date.getUTCMinutes()).padStart(2, "0")}:${String(date.getUTCSeconds()).padStart(2, "0")}`;
}

function formatLegacyStatisticsDateTime(seconds: number): string {
  return formatLegacyDateTime(seconds).replace(" ", ", ");
}

function formatFleetTimestamp(seconds: number): string {
  const date = new Date((seconds + 3 * 60 * 60) * 1000);
  const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
  return `${weekdays[date.getUTCDay()]} ${months[date.getUTCMonth()]} ${date.getUTCDate()} ${date.getUTCHours()}:${String(
    date.getUTCMinutes()
  ).padStart(2, "0")}:${String(date.getUTCSeconds()).padStart(2, "0")}`;
}

function formatLegacyNumber(value: number): string {
  return Math.floor(Math.max(0, value)).toLocaleString("de-DE");
}

function formatLegacyPlainNumber(value: number): string {
  return Math.round(Math.max(0, value)).toLocaleString("de-DE");
}

function formatLegacyRawInteger(value: number): string {
  return String(Math.round(Math.max(0, value)));
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
