export type SideName = "legacy" | "migrated";

export type GameDynamicAction = {
  type: "click" | "fill" | "type" | "select" | "hover" | "press" | "wait" | "popup";
  selector?: string;
  legacySelector?: string;
  migratedSelector?: string;
  value?: string;
  waitForSelector?: string;
  legacyWaitForSelector?: string;
  migratedWaitForSelector?: string;
  popupWaitForSelector?: string;
  legacyPopupWaitForSelector?: string;
  migratedPopupWaitForSelector?: string;
  dispatchClick?: boolean;
  legacyDispatchClick?: boolean;
  migratedDispatchClick?: boolean;
  waitMs?: number;
};

export type GameDynamicAssertion = {
  name: string;
  type: "text" | "html" | "value" | "visible" | "count" | "checked" | "evaluate";
  selector?: string;
  legacySelector?: string;
  migratedSelector?: string;
  expression?: string;
  compareSides?: boolean;
  expected?: string;
  contains?: string;
  tolerance?: number;
};

export type GameDynamicBehaviorSpec = {
  name: string;
  fixtureProfile?: "admin" | "max_fleet" | "no_ships" | "low_fuel" | "no_cargo" | "queue_short" | "research_short" | "shipyard_short";
  fixedClock?: boolean;
  legacyPage: string;
  legacyQuery?: Record<string, string>;
  migratedPath: string;
  migratedQuery?: Record<string, string>;
  legacyReady: string;
  migratedReady: string;
  applicabilitySelector?: string;
  legacyApplicabilitySelector?: string;
  migratedApplicabilitySelector?: string;
  requiredFixtureFeatures?: Array<"acs" | "alliance" | "commander" | "phalanx" | "report">;
  isolateSides?: boolean;
  actions: GameDynamicAction[];
  assertions: GameDynamicAssertion[];
  notes?: string[];
};

export const gameDynamicBehaviorSpecs: GameDynamicBehaviorSpec[] = [
  {
    name: "messages-compose-text-counter",
    legacyPage: "writemessages",
    legacyQuery: { messageziel: "1" },
    migratedPath: "/game/messages",
    migratedQuery: { messageziel: "1" },
    legacyReady: "#content form textarea[name='text']",
    migratedReady: ".legacy-messages-compose-table textarea[name='text']",
    actions: [{ type: "type", selector: "textarea[name='text']", value: "ab" }],
    assertions: [{ name: "counter", type: "text", selector: "#cntChars", expected: "2" }],
    notes: ["Covers legacy cntchar-style keyup behavior for React-rendered compose forms."]
  },
  {
    name: "notes-create-text-counter",
    legacyPage: "notizen",
    legacyQuery: { a: "1" },
    migratedPath: "/game/notes",
    migratedQuery: { a: "1" },
    legacyReady: "#content form textarea[name='text']",
    migratedReady: ".legacy-notes-form-table textarea[name='text']",
    actions: [{ type: "type", selector: "textarea[name='text']", value: "note" }],
    assertions: [{ name: "counter", type: "text", selector: "#cntChars", expected: "4" }],
    notes: ["Covers cntchar-style keyup behavior on the notes form."]
  },
  {
    name: "buddy-request-text-counter",
    legacyPage: "buddy",
    legacyQuery: { action: "7", buddy_id: "$fixture.galaxy_hover.target_player_id" },
    migratedPath: "/game/buddy",
    migratedQuery: { action: "7", buddy_id: "$fixture.galaxy_hover.target_player_id" },
    legacyReady: "#content form textarea[name='text']",
    migratedReady: ".legacy-buddy-table textarea[name='text']",
    actions: [{ type: "type", selector: "textarea[name='text']", value: "buddy" }],
    assertions: [{ name: "counter", type: "text", selector: "#cntChars", expected: "5" }],
    notes: ["Covers cntchar-style keyup behavior on the buddy request form."]
  },
  {
    name: "galaxy-planet-hover-tooltip",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='Planet Visual Hover Planet']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='planet'] a",
        waitMs: 850
      }
    ],
    assertions: [
      {
        name: "tooltip",
        type: "text",
        legacySelector: "#overDiv",
        migratedSelector: ".legacy-galaxy-tooltip",
        contains: "Visual Hover Planet"
      }
    ],
    notes: ["Proves the overLib replacement opens and carries the expected planet tooltip content."]
  },
  {
    name: "galaxy-action-message-compose-link",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content a[href*='page=writemessages'][href*='messageziel=']",
        migratedSelector: ".legacy-galaxy-actions a[href*='/game/messages'][href*='messageziel=']",
        legacyWaitForSelector: "#content form textarea[name='text']",
        migratedWaitForSelector: ".legacy-messages-compose-table textarea[name='text']"
      }
    ],
    assertions: [
      { name: "compose-visible", type: "visible", selector: "textarea[name='text']", expected: "true" },
      { name: "counter", type: "text", selector: "#cntChars", expected: "0" }
    ],
    notes: ["Covers a galaxy action icon navigating to the message compose screen without DB mutation."]
  },
  {
    name: "galaxy-action-buddy-request-link",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content a[href*='page=buddy'][href*='action=7'][href*='buddy_id=']",
        migratedSelector: ".legacy-galaxy-actions a[href*='/game/buddy'][href*='action=7'][href*='buddy_id=']",
        legacyWaitForSelector: "#content form textarea[name='text']",
        migratedWaitForSelector: ".legacy-buddy-table textarea[name='text']"
      }
    ],
    assertions: [
      { name: "request-visible", type: "visible", selector: "textarea[name='text']", expected: "true" },
      { name: "counter", type: "text", selector: "#cntChars", expected: "0" }
    ],
    notes: ["Covers a galaxy action icon navigating to the buddy request screen without DB mutation."]
  },
  {
    name: "galaxy-report-planet-popup-window",
    legacyPage: "galaxy",
    legacyQuery: { galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    migratedPath: "/game/galaxy",
    migratedQuery: { galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredFixtureFeatures: ["report"],
    actions: [
      {
        type: "popup",
        legacySelector: "#content a[onclick*='page=bericht']",
        migratedSelector: ".legacy-galaxy-actions a[data-galaxy-action='Spy Report']",
        legacyPopupWaitForSelector: "body",
        migratedPopupWaitForSelector: ".legacy-report-table"
      }
    ],
    assertions: [
      {
        name: "report-buttons",
        type: "count",
        legacySelector: "#content a[onclick*='page=bericht']",
        migratedSelector: ".legacy-galaxy-actions a[data-galaxy-action='Spy Report']",
        expected: "2"
      },
      {
        name: "popup-body",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.bodyText ?? ''",
        contains: "Visual Spy Report"
      },
      {
        name: "popup-width",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.innerWidth ?? 0",
        compareSides: true,
        tolerance: 4
      },
      {
        name: "popup-height",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.innerHeight ?? 0",
        compareSides: true,
        tolerance: 4
      },
      {
        name: "popup-report-param",
        type: "evaluate",
        expression: "/bericht=\\d+/.test(window.__ogameDynamicPopup?.url ?? '')",
        expected: "true"
      }
    ],
    notes: ["Covers legacy fenster(..., Bericht_Spionage) behavior for the planet spy-report action icon."]
  },
  {
    name: "galaxy-report-moon-popup-window",
    legacyPage: "galaxy",
    legacyQuery: { galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    migratedPath: "/game/galaxy",
    migratedQuery: { galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredFixtureFeatures: ["report"],
    actions: [
      {
        type: "popup",
        legacySelector: ":nth-match(#content a[onclick*='page=bericht'], 2)",
        migratedSelector: ":nth-match(.legacy-galaxy-actions a[data-galaxy-action='Spy Report'], 2)",
        legacyPopupWaitForSelector: "body",
        migratedPopupWaitForSelector: ".legacy-report-table"
      }
    ],
    assertions: [
      {
        name: "report-buttons",
        type: "count",
        legacySelector: "#content a[onclick*='page=bericht']",
        migratedSelector: ".legacy-galaxy-actions a[data-galaxy-action='Spy Report']",
        expected: "2"
      },
      {
        name: "popup-body",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.bodyText ?? ''",
        contains: "Visual Spy Report"
      },
      {
        name: "popup-width",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.innerWidth ?? 0",
        compareSides: true,
        tolerance: 4
      },
      {
        name: "popup-height",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.innerHeight ?? 0",
        compareSides: true,
        tolerance: 4
      },
      {
        name: "popup-report-param",
        type: "evaluate",
        expression: "/bericht=\\d+/.test(window.__ogameDynamicPopup?.url ?? '')",
        expected: "true"
      }
    ],
    notes: ["Covers the second legacy spy-report popup button when both planet and moon reports are shared."]
  },
  {
    name: "galaxy-phalanx-name-popup-window",
    legacyPage: "galaxy",
    legacyQuery: { cp: "$fixture.phalanx.source_moon_id", galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    migratedPath: "/game/galaxy",
    migratedQuery: { cp: "$fixture.phalanx.source_moon_id", galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredFixtureFeatures: ["phalanx"],
    actions: [
      {
        type: "popup",
        legacySelector: "#content th[width='130'] a[onclick*='page=phalanx']",
        migratedSelector: ".legacy-galaxy-name a[data-galaxy-action='Phalanx']",
        legacyPopupWaitForSelector: "body",
        migratedPopupWaitForSelector: ".legacy-phalanx-table"
      }
    ],
    assertions: [
      {
        name: "popup-body",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.bodyText ?? ''",
        contains: "Sensor report"
      },
      {
        name: "popup-width",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.innerWidth ?? 0",
        compareSides: true,
        tolerance: 4
      },
      {
        name: "popup-height",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.innerHeight ?? 0",
        compareSides: true,
        tolerance: 4
      },
      {
        name: "popup-target-param",
        type: "evaluate",
        expression: "/spid=\\d+/.test(window.__ogameDynamicPopup?.url ?? '')",
        expected: "true"
      }
    ],
    notes: ["Covers legacy fenster(..., Bericht_Phalanx) behavior from the galaxy planet-name phalanx link."]
  },
  {
    name: "galaxy-phalanx-hover-popup-window",
    legacyPage: "galaxy",
    legacyQuery: { cp: "$fixture.phalanx.source_moon_id", galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    migratedPath: "/game/galaxy",
    migratedQuery: { cp: "$fixture.phalanx.source_moon_id", galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    requiredFixtureFeatures: ["phalanx"],
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='Planet Visual Hover Planet']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='planet'] a",
        waitMs: 850
      },
      {
        type: "popup",
        legacySelector: "#overDiv a[onclick*='page=phalanx']",
        migratedSelector: ".legacy-galaxy-tooltip a[data-galaxy-popup='Bericht_Phalanx']",
        legacyPopupWaitForSelector: "body",
        migratedPopupWaitForSelector: ".legacy-phalanx-table",
        migratedDispatchClick: true
      }
    ],
    assertions: [
      {
        name: "popup-body",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.bodyText ?? ''",
        contains: "Sensor report"
      },
      {
        name: "popup-width",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.innerWidth ?? 0",
        compareSides: true,
        tolerance: 4
      },
      {
        name: "popup-height",
        type: "evaluate",
        expression: "window.__ogameDynamicPopup?.innerHeight ?? 0",
        compareSides: true,
        tolerance: 4
      },
      {
        name: "popup-target-param",
        type: "evaluate",
        expression: "/spid=\\d+/.test(window.__ogameDynamicPopup?.url ?? '')",
        expected: "true"
      }
    ],
    notes: [
      "Covers the legacy planet hover-menu Phalanx popup action.",
      "The migrated portal tooltip uses dispatched click after hover to avoid Playwright mouseleave timing while still executing the popup handler."
    ]
  },
  {
    name: "galaxy-keyboard-system-left",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [{ type: "press", selector: "body", value: "ArrowLeft", waitMs: 700 }],
    assertions: [
      { name: "galaxy", type: "value", selector: "input[name='galaxy']", compareSides: true },
      { name: "system", type: "value", selector: "input[name='system']", compareSides: true }
    ],
    notes: ["Covers legacy document.onkeyup systemLeft behavior."]
  },
  {
    name: "galaxy-keyboard-system-right",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [{ type: "press", selector: "body", value: "ArrowRight", waitMs: 700 }],
    assertions: [
      { name: "galaxy", type: "value", selector: "input[name='galaxy']", compareSides: true },
      { name: "system", type: "value", selector: "input[name='system']", compareSides: true }
    ],
    notes: ["Covers legacy document.onkeyup systemRight behavior."]
  },
  {
    name: "galaxy-keyboard-galaxy-up",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [{ type: "press", selector: "body", value: "ArrowUp", waitMs: 700 }],
    assertions: [
      { name: "galaxy", type: "value", selector: "input[name='galaxy']", compareSides: true },
      { name: "system", type: "value", selector: "input[name='system']", compareSides: true }
    ],
    notes: ["Covers legacy document.onkeyup galaxyRight behavior."]
  },
  {
    name: "galaxy-keyboard-galaxy-down",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [{ type: "press", selector: "body", value: "ArrowDown", waitMs: 700 }],
    assertions: [
      { name: "galaxy", type: "value", selector: "input[name='galaxy']", compareSides: true },
      { name: "system", type: "value", selector: "input[name='system']", compareSides: true }
    ],
    notes: ["Covers legacy document.onkeyup galaxyLeft behavior."]
  },
  {
    name: "galaxy-instant-spy-noob-failure",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content tr:has-text('Visual Noob Planet') a[onclick*='doit(6']",
        migratedSelector: "tr[data-galaxy-position='3'] .legacy-galaxy-actions a[data-galaxy-action='Espionage']",
        waitForSelector: "#fleetstatustable tr"
      }
    ],
    assertions: [
      { name: "status-row", type: "text", selector: "#fleetstatustable tr:first-child", compareSides: true },
      {
        name: "status-result",
        type: "text",
        selector: "#fleetstatustable tr:first-child td:nth-child(2)",
        expected: "Error! It is impossible to fly to the player, because he is under noob protection!"
      }
    ],
    notes: ["Covers legacy galaxy doit(6) noob-protection failure row parity."]
  },
  {
    name: "galaxy-instant-spy-vacation-failure",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content tr:has-text('Visual Vacation') a[onclick*='doit(6']",
        migratedSelector: "tr[data-galaxy-position='4'] .legacy-galaxy-actions a[data-galaxy-action='Espionage']",
        waitForSelector: "#fleetstatustable tr"
      }
    ],
    assertions: [
      { name: "status-row", type: "text", selector: "#fleetstatustable tr:first-child", compareSides: true },
      { name: "status-result", type: "text", selector: "#fleetstatustable tr:first-child td:nth-child(2)", expected: "Impossible, the player is in vacation mode" },
      { name: "status-result-html", type: "html", selector: "#fleetstatustable tr:first-child td:nth-child(2)", compareSides: true }
    ],
    notes: ["Covers legacy galaxy doit(6) vacation failure message and class parity."]
  },
  {
    name: "galaxy-instant-spy-dispatch-success",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content tr:has-text('Visual Hover Planet') a[onclick*='doit(6']",
        migratedSelector: "tr[data-galaxy-position='1'] .legacy-galaxy-actions a[data-galaxy-action='Espionage']",
        waitForSelector: "#fleetstatustable tr"
      }
    ],
    assertions: [
      { name: "status-row", type: "text", selector: "#fleetstatustable tr:first-child", compareSides: true },
      { name: "status-result", type: "text", selector: "#fleetstatustable tr:first-child td:nth-child(2)", expected: "done" }
    ],
    notes: ["Covers successful legacy galaxy doit(6) espionage dispatch status row parity."]
  },
  {
    name: "galaxy-instant-recycle-dispatch-success",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content a[onclick*='doit(8']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='debris'] > a",
        waitForSelector: "#fleetstatustable tr"
      }
    ],
    assertions: [
      { name: "status-row", type: "text", selector: "#fleetstatustable tr:first-child", compareSides: true },
      { name: "status-result", type: "text", selector: "#fleetstatustable tr:first-child td:nth-child(2)", expected: "done" }
    ],
    notes: ["Covers successful legacy galaxy doit(8) recycle dispatch status row parity."]
  },
  {
    name: "galaxy-instant-spy-max-fleet-failure",
    fixtureProfile: "max_fleet",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content tr:has-text('Visual Max Target') a[onclick*='doit(6']",
        migratedSelector: "tr[data-galaxy-position='5'] .legacy-galaxy-actions a[data-galaxy-action='Espionage']",
        waitForSelector: "#fleetstatustable tr"
      }
    ],
    assertions: [
      { name: "status-row", type: "text", selector: "#fleetstatustable tr:first-child", compareSides: true },
      { name: "status-result", type: "text", selector: "#fleetstatustable tr:first-child td:nth-child(2)", expected: "Not enough room for a fleet" }
    ],
    notes: ["Covers legacy galaxy doit(6) max-fleet failure after slot exhaustion."]
  },
  {
    name: "galaxy-instant-spy-no-ships-failure",
    fixtureProfile: "no_ships",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content tr:has-text('Visual Empty Target') a[onclick*='doit(6']",
        migratedSelector: "tr[data-galaxy-position='7'] .legacy-galaxy-actions a[data-galaxy-action='Espionage']",
        waitForSelector: "#fleetstatustable tr"
      }
    ],
    assertions: [
      { name: "status-row", type: "text", selector: "#fleetstatustable tr:first-child", compareSides: true },
      { name: "status-result", type: "text", selector: "#fleetstatustable tr:first-child td:nth-child(2)", expected: "Error! No ships to send" }
    ],
    notes: ["Covers legacy galaxy doit(6) no-probe failure row parity."]
  },
  {
    name: "galaxy-instant-recycle-no-fuel-failure",
    fixtureProfile: "low_fuel",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content a[onclick*='doit(8']",
        migratedSelector: ".legacy-galaxy-hover[data-galaxy-hover='debris'] > a",
        waitForSelector: "#fleetstatustable tr"
      }
    ],
    assertions: [
      { name: "status-row", type: "text", selector: "#fleetstatustable tr:first-child", compareSides: true },
      { name: "status-result", type: "text", selector: "#fleetstatustable tr:first-child td:nth-child(2)", expected: "You don't have enough deuterium" }
    ],
    notes: ["Covers legacy galaxy doit(8) no-deuterium failure row parity."]
  },
  {
    name: "galaxy-instant-spy-no-cargo-failure",
    fixtureProfile: "no_cargo",
    legacyPage: "galaxy",
    legacyQuery: { galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    migratedPath: "/game/galaxy",
    migratedQuery: { galaxy: "$fixture.galaxy_hover.galaxy", system: "$fixture.galaxy_hover.system" },
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content tr:has-text('Visual Hover Planet') a[onclick*='doit(6']",
        migratedSelector: "tr[data-galaxy-position='1'] .legacy-galaxy-actions a[data-galaxy-action='Espionage']",
        waitForSelector: "#fleetstatustable tr"
      }
    ],
    assertions: [
      { name: "status-row", type: "text", selector: "#fleetstatustable tr:first-child", compareSides: true },
      { name: "status-result", type: "text", selector: "#fleetstatustable tr:first-child td:nth-child(2)", expected: "Error! Insufficient carrying capacity!" }
    ],
    notes: ["Covers legacy galaxy doit(6) probe fuel cargo-capacity failure row parity."]
  },
  {
    name: "alliance-circular-text-counter",
    legacyPage: "allianzen",
    legacyQuery: { a: "17" },
    migratedPath: "/game/alliance",
    migratedQuery: { a: "17" },
    legacyReady: "#content textarea[name='text']",
    migratedReady: ".legacy-alliance-circular-table textarea[name='text']",
    requiredFixtureFeatures: ["alliance"],
    actions: [{ type: "type", selector: "textarea[name='text']", value: "ally" }],
    assertions: [{ name: "counter", type: "text", selector: "#cntChars", expected: "4" }],
    notes: ["Covers cntchar-style keyup behavior on the alliance circular message form."]
  },
  {
    name: "alliance-application-reject-text-counter",
    legacyPage: "bewerbungen",
    legacyQuery: { show: "$fixture.alliance.application_id", sort: "1" },
    migratedPath: "/game/alliance",
    migratedQuery: { page: "bewerbungen", show: "$fixture.alliance.application_id", sort: "1" },
    legacyReady: "#content textarea[name='text']",
    migratedReady: ".legacy-alliance-applications-table textarea[name='text']",
    requiredFixtureFeatures: ["alliance"],
    actions: [{ type: "type", selector: "textarea[name='text']", value: "reject" }],
    assertions: [{ name: "counter", type: "text", selector: "#cntChars", expected: "6" }],
    notes: ["Covers cntchar-style keyup behavior on the alliance application rejection reason form."]
  },
  {
    name: "statistics-player-delta-tooltip",
    legacyPage: "statistics",
    legacyQuery: { type: "ressources", start: "1" },
    migratedPath: "/game/statistics",
    migratedQuery: { type: "ressources", start: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-statistics-player-table",
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='From']",
        migratedSelector: ".legacy-statistics-player-table .legacy-statistics-delta",
        legacyWaitForSelector: "#overDiv",
        migratedWaitForSelector: ".legacy-statistics-player-table .legacy-statistics-tooltip"
      }
    ],
    assertions: [
      {
        name: "tooltip",
        type: "text",
        legacySelector: "#overDiv",
        migratedSelector: ".legacy-statistics-player-table .legacy-statistics-tooltip",
        compareSides: true,
        contains: "From"
      }
    ],
    notes: ["Covers statistics.php rank delta overlib text parity for player rows."]
  },
  {
    name: "statistics-alliance-delta-tooltip",
    legacyPage: "statistics",
    legacyQuery: { who: "ally", type: "ressources", start: "1" },
    migratedPath: "/game/statistics",
    migratedQuery: { who: "ally", type: "ressources", start: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-statistics-alliance-table",
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='From']",
        migratedSelector: ".legacy-statistics-alliance-table .legacy-statistics-delta",
        legacyWaitForSelector: "#overDiv",
        migratedWaitForSelector: ".legacy-statistics-alliance-table .legacy-statistics-tooltip"
      }
    ],
    assertions: [
      {
        name: "tooltip",
        type: "text",
        legacySelector: "#overDiv",
        migratedSelector: ".legacy-statistics-alliance-table .legacy-statistics-tooltip",
        compareSides: true,
        contains: "From"
      }
    ],
    notes: ["Covers statistics.php rank delta overlib text parity for alliance rows."]
  },
  {
    name: "empire-average-tooltip",
    legacyPage: "imperium",
    legacyQuery: { planettype: "1", no_header: "1" },
    migratedPath: "/game/empire",
    migratedQuery: { planettype: "1" },
    legacyReady: "#content table",
    migratedReady: ".legacy-empire-table",
    requiredFixtureFeatures: ["commander"],
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='Average per planet']",
        migratedSelector: ".legacy-empire-table a[onmouseover*='Average per planet']",
        waitForSelector: "#overDiv"
      }
    ],
    assertions: [
      { name: "tooltip", type: "text", selector: "#overDiv", compareSides: true, contains: "Average per planet" },
      { name: "tooltip-html", type: "html", selector: "#overDiv", compareSides: true }
    ],
    notes: ["Covers imperium.php average overlib text and frame parity."]
  },
  {
    name: "admin-battlesim-slot-sync",
    fixtureProfile: "admin",
    legacyPage: "admin",
    legacyQuery: { mode: "BattleSim" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BattleSim" },
    legacyReady: "#battle_source",
    migratedReady: ".legacy-admin-battlesim-table #battle_source",
    actions: [
      { type: "type", selector: "#a_202", value: "3" },
      { type: "type", selector: "#a_weap", value: "5" },
      { type: "select", selector: "select[name='dslot']", value: "2" },
      { type: "type", selector: "#d_401", value: "7" }
    ],
    assertions: [
      { name: "attacker-small-cargo-hidden", type: "value", selector: "#a0_202", compareSides: true },
      { name: "attacker-weapons-hidden", type: "value", selector: "#a0_weap", compareSides: true },
      { name: "defender-slot2-rocket-hidden", type: "value", selector: "#d1_401", compareSides: true },
      { name: "attacker-slot-count", type: "value", selector: "#anum", expected: "1" },
      { name: "defender-slot-count", type: "value", selector: "#dnum", expected: "2" }
    ],
    notes: ["Covers BattleSim visible inputs, hidden slot state, tech state, and attacker/defender counters."]
  },
  {
    name: "admin-botedit-init-palette",
    fixtureProfile: "admin",
    legacyPage: "admin",
    legacyQuery: { mode: "BotEdit" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BotEdit" },
    legacyReady: "#myDiagram canvas",
    migratedReady: "#myDiagram canvas",
    actions: [{ type: "wait", waitMs: 100 }],
    assertions: [
      { name: "diagram-canvas-count", type: "count", selector: "#myDiagram canvas", compareSides: true },
      { name: "palette-canvas-count", type: "count", selector: "#myPalette canvas", compareSides: true },
      {
        name: "diagram-and-palette-ready",
        type: "evaluate",
        expression: "Boolean(window.myDiagram && window.myPalette)",
        expected: "true"
      },
      {
        name: "palette-node-count",
        type: "evaluate",
        expression: "window.myPalette && window.myPalette.model && window.myPalette.model.nodeDataArray ? window.myPalette.model.nodeDataArray.length : 0",
        compareSides: true,
        expected: "7"
      }
    ],
    notes: ["Covers BotEdit GoJS init and palette model state; legacy load() uses a flaky SACK race in headless browsers."]
  },
  {
    name: "admin-botedit-load-strategy",
    fixtureProfile: "admin",
    legacyPage: "admin",
    legacyQuery: { mode: "BotEdit" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BotEdit" },
    legacyReady: "#strategyId",
    migratedReady: ".legacy-admin-botedit-table #strategyId",
    actions: [
      { type: "select", selector: "#strategyId", value: "$fixture.botedit.strategy_id" },
      { type: "click", selector: "button:has-text('Load')", waitMs: 900 }
    ],
    assertions: [
      { name: "strategy-name", type: "value", selector: "#strategyName", compareSides: true, expected: "Visual BotEdit" },
      { name: "import-strategy-id", type: "value", selector: "#strategyId_ForImport", compareSides: true }
    ],
    notes: ["Covers BotEdit legacy SACK load action and strategy name/import id DOM updates."]
  },
  {
    name: "admin-botedit-save-strategy",
    fixtureProfile: "admin",
    isolateSides: true,
    legacyPage: "admin",
    legacyQuery: { mode: "BotEdit" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BotEdit" },
    legacyReady: "#strategyId",
    migratedReady: ".legacy-admin-botedit-table #strategyId",
    actions: [
      { type: "select", selector: "#strategyId", value: "$fixture.botedit.strategy_id" },
      { type: "click", selector: "button:has-text('Load')", waitMs: 900 },
      { type: "click", selector: "button:has-text('Save')", waitMs: 700 }
    ],
    assertions: [
      { name: "saved-model", type: "value", selector: "#mySavedModel", compareSides: true, contains: "Visual Start" }
    ],
    notes: ["Covers BotEdit legacy SACK save action and mySavedModel refresh from the GoJS model."]
  },
  {
    name: "admin-botedit-rename-strategy",
    fixtureProfile: "admin",
    isolateSides: true,
    legacyPage: "admin",
    legacyQuery: { mode: "BotEdit" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BotEdit" },
    legacyReady: "#strategyId",
    migratedReady: ".legacy-admin-botedit-table #strategyId",
    actions: [
      { type: "select", selector: "#strategyId", value: "$fixture.botedit.strategy_id" },
      { type: "fill", selector: "#strategyName", value: "Visual BotEdit Renamed" },
      { type: "click", selector: "button:has-text('Rename')", waitMs: 700 }
    ],
    assertions: [
      {
        name: "renamed-option",
        type: "text",
        selector: "#strategyId option:checked",
        compareSides: true,
        expected: "Visual BotEdit Renamed"
      }
    ],
    notes: ["Covers BotEdit legacy SACK rename action and returned option list replacement."]
  },
  {
    name: "admin-botedit-new-strategy",
    fixtureProfile: "admin",
    isolateSides: true,
    legacyPage: "admin",
    legacyQuery: { mode: "BotEdit" },
    migratedPath: "/game/admin",
    migratedQuery: { mode: "BotEdit" },
    legacyReady: "#strategyId",
    migratedReady: ".legacy-admin-botedit-table #strategyId",
    actions: [
      { type: "fill", selector: "#strategyName", value: "Visual BotEdit New" },
      {
        type: "click",
        selector: "button:has-text('New')",
        waitMs: 1500
      }
    ],
    assertions: [
      {
        name: "new-option-count",
        type: "count",
        selector: "#strategyId option:has-text('Visual BotEdit New')",
        compareSides: true,
        expected: "1"
      }
    ],
    notes: ["Covers BotEdit legacy SACK new action and reload with the inserted strategy option."]
  },
  {
    name: "admin-botedit-preview-popup",
    fixtureProfile: "admin",
    legacyPage: "admin",
    legacyQuery: { mode: "BotEdit", action: "preview", strat: "$fixture.botedit.strategy_id" },
    migratedPath: "/game/index.php",
    migratedQuery: { page: "admin", mode: "BotEdit", action: "preview", strat: "$fixture.botedit.strategy_id" },
    legacyReady: "#preview_img",
    migratedReady: "#preview_img",
    actions: [{ type: "wait", waitMs: 1200 }],
    assertions: [
      {
        name: "preview-image-data-url",
        type: "evaluate",
        expression: "document.querySelector('#preview_img')?.getAttribute('src')?.startsWith('data:image/png') ?? false",
        compareSides: true,
        expected: "true"
      },
      {
        name: "preview-strategy-name",
        type: "value",
        selector: "#strategyName",
        compareSides: true
      },
      {
        name: "preview-source",
        type: "value",
        selector: "#mySavedModel",
        compareSides: true,
        contains: "Visual Start"
      }
    ],
    notes: ["Covers BotEdit Show popup URL, hidden editor bootstrap, AJAX load, and generated preview image."]
  },
  {
    name: "admin-botedit-export-popup",
    fixtureProfile: "admin",
    legacyPage: "admin",
    legacyQuery: { mode: "BotEdit", action: "export", strat: "$fixture.botedit.strategy_id" },
    migratedPath: "/game/index.php",
    migratedQuery: { page: "admin", mode: "BotEdit", action: "export", strat: "$fixture.botedit.strategy_id" },
    legacyReady: "body",
    migratedReady: "body",
    actions: [],
    assertions: [
      { name: "export-source", type: "text", selector: "body", compareSides: true, contains: "Visual Start" },
      { name: "export-model-class", type: "text", selector: "body", contains: "go.GraphLinksModel" }
    ],
    notes: ["Covers BotEdit Export popup URL and raw strategy JSON response parity."]
  },
  {
    name: "fleet-select-all-ships",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      {
        type: "click",
        legacySelector: "a[href*='maxShips']",
        migratedSelector: "a[href='#all-ships']"
      }
    ],
    assertions: [{ name: "small-cargo", type: "value", selector: "input[name='ship202']", compareSides: true }],
    notes: ["Skipped when the shared fixture has no small cargo row available."]
  },
  {
    name: "fleet-continue-short-info",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      { type: "select", selector: "select[name='speed']", value: "5" }
    ],
    assertions: [
      { name: "distance", type: "text", selector: "#distance", compareSides: true },
      { name: "duration", type: "text", selector: "#duration", compareSides: true },
      { name: "consumption", type: "html", selector: "#consumption", compareSides: true },
      { name: "storage", type: "html", selector: "#storage", compareSides: true }
    ],
    notes: ["Covers flotten2 shortInfo recalculation after changing fleet speed."]
  },
  {
    name: "fleet-target-coordinate-short-info",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      { type: "type", selector: "input[name='galaxy']", value: "$fixture.galaxy_hover.galaxy" },
      { type: "type", selector: "input[name='system']", value: "$fixture.galaxy_hover.system" },
      { type: "type", selector: "input[name='planet']", value: "$fixture.galaxy_hover.target_position" },
      { type: "select", selector: "select[name='planettype']", value: "1" },
      { type: "select", selector: "select[name='speed']", value: "5" }
    ],
    assertions: [
      { name: "distance", type: "text", selector: "#distance", compareSides: true },
      { name: "duration", type: "text", selector: "#duration", compareSides: true },
      { name: "consumption", type: "html", selector: "#consumption", compareSides: true },
      { name: "storage", type: "html", selector: "#storage", compareSides: true }
    ],
    notes: ["Covers flotten2 shortInfo recalculation after changing target coordinates and planet type."]
  },
  {
    name: "fleet-transport-residue",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#remainingresources",
        migratedWaitForSelector: "#remainingresources"
      },
      {
        type: "click",
        legacySelector: "#content a[href*='maxResource'][href*='1']",
        migratedSelector: ".legacy-fleet-dispatch-table a[href='#max-resource']"
      }
    ],
    assertions: [{ name: "remaining-resources", type: "html", selector: "#remainingresources", compareSides: true }],
    notes: ["Covers flotten3 resource residue color/text update."]
  },
  {
    name: "fleet-transport-overcapacity-residue",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#remainingresources",
        migratedWaitForSelector: "#remainingresources"
      },
      { type: "fill", selector: "input[name='resource1']", value: "999999" },
      { type: "press", selector: "input[name='resource1']", value: "Tab" }
    ],
    assertions: [{ name: "remaining-resources", type: "html", selector: "#remainingresources", compareSides: true }],
    notes: ["Covers red residue output when flotten3 cargo input exceeds available capacity."]
  },
  {
    name: "fleet-transport-all-resources",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      { type: "type", selector: "input[name='galaxy']", value: "$fixture.galaxy_hover.galaxy" },
      { type: "type", selector: "input[name='system']", value: "$fixture.galaxy_hover.system" },
      { type: "type", selector: "input[name='planet']", value: "$fixture.galaxy_hover.target_position" },
      { type: "select", selector: "select[name='planettype']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#remainingresources",
        migratedWaitForSelector: "#remainingresources"
      },
      {
        type: "click",
        legacySelector: "#content a[href^='javascript:maxResources']",
        migratedSelector: ".legacy-fleet-dispatch-table a[href='#max-resources']"
      }
    ],
    assertions: [
      { name: "metal", type: "value", selector: "input[name='resource1']", compareSides: true },
      { name: "crystal", type: "value", selector: "input[name='resource2']", compareSides: true },
      { name: "deuterium", type: "value", selector: "input[name='resource3']", compareSides: true },
      { name: "remaining-resources", type: "html", selector: "#remainingresources", compareSides: true }
    ],
    notes: ["Covers flotten3 maxResources cargo distribution and residue update."]
  },
  {
    name: "fleet-attack-mission-radio-selection",
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      { type: "type", selector: "input[name='galaxy']", value: "$fixture.galaxy_hover.galaxy" },
      { type: "type", selector: "input[name='system']", value: "$fixture.galaxy_hover.system" },
      { type: "type", selector: "input[name='planet']", value: "$fixture.galaxy_hover.target_position" },
      { type: "select", selector: "select[name='planettype']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content input[name='order'][value='1']",
        migratedWaitForSelector: ".legacy-fleet-dispatch-table input[name='order'][value='1']"
      },
      { type: "click", selector: "input[name='order'][value='1']" }
    ],
    assertions: [
      { name: "attack-option-count", type: "count", selector: "input[name='order'][value='1']", expected: "1" },
      { name: "transport-option-count", type: "count", selector: "input[name='order'][value='3']", expected: "1" },
      { name: "attack-selected", type: "checked", selector: "input[name='order'][value='1']", compareSides: true, expected: "true" }
    ],
    notes: ["Covers flotten3 mission radio availability and selected-state parity for enemy planet dispatch."]
  },
  {
    name: "fleet-attack-launch-noob-protection-error",
    isolateSides: true,
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      { type: "type", selector: "input[name='galaxy']", value: "$fixture.galaxy_hover.galaxy" },
      { type: "type", selector: "input[name='system']", value: "$fixture.galaxy_hover.system" },
      { type: "type", selector: "input[name='planet']", value: "$fixture.galaxy_hover.noob_target_position" },
      { type: "select", selector: "select[name='planettype']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content input[name='order'][value='1']",
        migratedWaitForSelector: ".legacy-fleet-dispatch-table input[name='order'][value='1']"
      },
      { type: "click", selector: "input[name='order'][value='1']" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-dispatch-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content span.error:has-text('The planet is protected for newbies!')",
        migratedWaitForSelector: ".legacy-overview-table:has-text('The planet is protected for newbies!')"
      }
    ],
    assertions: [
      {
        name: "launch-error",
        type: "text",
        legacySelector: "#content span.error:has-text('The planet is protected for newbies!')",
        migratedSelector: ".legacy-overview-table:has-text('The planet is protected for newbies!')",
        contains: "The planet is protected for newbies!"
      }
    ],
    notes: ["Covers final flottenversand/fleet launch-submit noob-protection parity."]
  },
  {
    name: "fleet-attack-launch-vacation-error",
    isolateSides: true,
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      { type: "type", selector: "input[name='galaxy']", value: "$fixture.galaxy_hover.galaxy" },
      { type: "type", selector: "input[name='system']", value: "$fixture.galaxy_hover.system" },
      { type: "type", selector: "input[name='planet']", value: "$fixture.galaxy_hover.vacation_target_position" },
      { type: "select", selector: "select[name='planettype']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content input[name='order'][value='1']",
        migratedWaitForSelector: ".legacy-fleet-dispatch-table input[name='order'][value='1']"
      },
      { type: "click", selector: "input[name='order'][value='1']" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-dispatch-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content span.error:has-text('This player is in vacation mode!')",
        migratedWaitForSelector: ".legacy-overview-table:has-text('This player is in vacation mode!')"
      }
    ],
    assertions: [
      {
        name: "launch-error",
        type: "text",
        legacySelector: "#content span.error:has-text('This player is in vacation mode!')",
        migratedSelector: ".legacy-overview-table:has-text('This player is in vacation mode!')",
        contains: "This player is in vacation mode!"
      }
    ],
    notes: ["Covers final flottenversand/fleet launch-submit vacation target parity."]
  },
  {
    name: "fleet-attack-launch-success-result",
    isolateSides: true,
    fixedClock: false,
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      { type: "type", selector: "input[name='galaxy']", value: "$fixture.galaxy_hover.galaxy" },
      { type: "type", selector: "input[name='system']", value: "$fixture.galaxy_hover.system" },
      { type: "type", selector: "input[name='planet']", value: "$fixture.galaxy_hover.target_position" },
      { type: "select", selector: "select[name='planettype']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content input[name='order'][value='1']",
        migratedWaitForSelector: ".legacy-fleet-dispatch-table input[name='order'][value='1']"
      },
      { type: "click", selector: "input[name='order'][value='1']" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-dispatch-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content table:has(span.success:has-text('Fleet dispatched:'))",
        migratedWaitForSelector: ".legacy-fleet-launch-result-table span.success:has-text('Fleet dispatched:')"
      }
    ],
    assertions: [
      {
        name: "launch-heading",
        type: "text",
        legacySelector: "#content table:has(span.success) span.success",
        migratedSelector: ".legacy-fleet-launch-result-table span.success",
        compareSides: true,
        expected: "Fleet dispatched:"
      },
      {
        name: "launch-mission",
        type: "text",
        legacySelector: "#content table:has(span.success) tr:nth-child(2) th:nth-child(2)",
        migratedSelector: ".legacy-fleet-launch-result-table tr:nth-child(2) th:nth-child(2)",
        compareSides: true,
        expected: "Attack"
      },
      {
        name: "launch-ship",
        type: "text",
        legacySelector: "#content table:has(span.success) tr:has-text('Small Cargo')",
        migratedSelector: ".legacy-fleet-launch-result-table tr[data-fleet-launch-ship-row='202']",
        compareSides: true,
        contains: "Small Cargo"
      }
    ],
    notes: ["Covers successful final flottenversand/fleet launch-submit result table parity."]
  },
  {
    name: "fleet-expedition-launch-success-result",
    isolateSides: true,
    fixedClock: false,
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      { type: "type", selector: "input[name='planet']", value: "16" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content input[name='order'][value='15']",
        migratedWaitForSelector: ".legacy-fleet-dispatch-table input[name='order'][value='15']"
      },
      { type: "click", selector: "input[name='order'][value='15']" },
      { type: "select", selector: "select[name='expeditiontime']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-dispatch-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content table:has(span.success:has-text('Fleet dispatched:'))",
        migratedWaitForSelector: ".legacy-fleet-launch-result-table span.success:has-text('Fleet dispatched:')"
      }
    ],
    assertions: [
      {
        name: "launch-heading",
        type: "text",
        legacySelector: "#content table:has(span.success) span.success",
        migratedSelector: ".legacy-fleet-launch-result-table span.success",
        compareSides: true,
        expected: "Fleet dispatched:"
      },
      {
        name: "launch-mission",
        type: "text",
        legacySelector: "#content table:has(span.success) tr:nth-child(2) th:nth-child(2)",
        migratedSelector: ".legacy-fleet-launch-result-table tr:nth-child(2) th:nth-child(2)",
        compareSides: true,
        expected: "Expedition"
      },
      {
        name: "launch-ship",
        type: "text",
        legacySelector: "#content table:has(span.success) tr:has-text('Small Cargo')",
        migratedSelector: ".legacy-fleet-launch-result-table tr[data-fleet-launch-ship-row='202']",
        compareSides: true,
        contains: "Small Cargo"
      }
    ],
    notes: ["Covers final flottenversand/fleet launch-submit expedition result table and expedition hold-time field parity."]
  },
  {
    name: "fleet-acs-attack-launch-success-result",
    isolateSides: true,
    fixedClock: false,
    legacyPage: "flotten1",
    migratedPath: "/game/fleet",
    legacyReady: "#content table",
    migratedReady: ".legacy-fleet-table",
    applicabilitySelector: "input[name='ship202']",
    requiredFixtureFeatures: ["acs"],
    actions: [
      { type: "fill", selector: "input[name='ship202']", value: "1" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='continue']",
        migratedSelector: ".legacy-fleet-select-table input[type='submit'][value='continue']",
        legacyWaitForSelector: "#content input[name='galaxy']",
        migratedWaitForSelector: ".legacy-fleet-target-table input[name='galaxy']"
      },
      {
        type: "click",
        legacySelector: "#content a[href*='setUnion']",
        migratedSelector: ".legacy-fleet-target-table a[href='#set-union-target']"
      },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-target-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content input[name='order'][value='2']",
        migratedWaitForSelector: ".legacy-fleet-dispatch-table input[name='order'][value='2']"
      },
      { type: "click", selector: "input[name='order'][value='2']" },
      {
        type: "click",
        legacySelector: "#content input[type='submit'][value='Next']",
        migratedSelector: ".legacy-fleet-dispatch-table input[type='submit'][value='Next']",
        legacyWaitForSelector: "#content table:has(span.success:has-text('Fleet dispatched:'))",
        migratedWaitForSelector: ".legacy-fleet-launch-result-table span.success:has-text('Fleet dispatched:')"
      }
    ],
    assertions: [
      {
        name: "launch-heading",
        type: "text",
        legacySelector: "#content table:has(span.success) span.success",
        migratedSelector: ".legacy-fleet-launch-result-table span.success",
        compareSides: true,
        expected: "Fleet dispatched:"
      },
      {
        name: "launch-mission",
        type: "text",
        legacySelector: "#content table:has(span.success) tr:nth-child(2) th:nth-child(2)",
        migratedSelector: ".legacy-fleet-launch-result-table tr:nth-child(2) th:nth-child(2)",
        compareSides: true,
        expected: "Joint attack"
      },
      {
        name: "launch-ship",
        type: "text",
        legacySelector: "#content table:has(span.success) tr:has-text('Small Cargo')",
        migratedSelector: ".legacy-fleet-launch-result-table tr[data-fleet-launch-ship-row='202']",
        compareSides: true,
        contains: "Small Cargo"
      }
    ],
    notes: ["Requires OGAME_GAME_VISUAL_ACS_FIXTURE=1; covers selecting a battle union, mission 2, and final ACS launch-submit result parity."]
  },
  {
    name: "buildings-short-queue-completion-refresh",
    fixtureProfile: "queue_short",
    isolateSides: true,
    fixedClock: false,
    legacyPage: "b_building",
    migratedPath: "/game/buildings",
    legacyReady: "#bxx",
    migratedReady: "#bxx",
    actions: [{ type: "wait", waitMs: 23000 }],
    assertions: [
      { name: "queue-countdown-gone", type: "count", selector: "#bxx", compareSides: true, expected: "0" },
      {
        name: "buildings-ready-after-refresh",
        type: "evaluate",
        expression: "Boolean(document.body.textContent && document.body.textContent.includes('Metal Mine') && !document.querySelector('#bxx'))",
        compareSides: true,
        expected: "true"
      }
    ],
    notes: ["Covers short building countdown completion, automatic refresh, and queue removal without freezing Date."]
  },
  {
    name: "research-short-queue-completion-done",
    fixtureProfile: "research_short",
    isolateSides: true,
    fixedClock: false,
    legacyPage: "buildings",
    legacyQuery: { mode: "Forschung" },
    migratedPath: "/game/research",
    legacyReady: "#bxx",
    migratedReady: "#bxx",
    actions: [{ type: "wait", waitMs: 19000 }],
    assertions: [
      { name: "research-countdown", type: "text", selector: "#bxx", compareSides: true, contains: "Done" },
      {
        name: "research-next-link",
        type: "count",
        selector: "#bxx a",
        compareSides: true,
        expected: "1"
      }
    ],
    notes: ["Covers short research countdown completion and Done/next state without freezing Date."]
  },
  {
    name: "shipyard-short-queue-completion-tasks-completed",
    fixtureProfile: "shipyard_short",
    isolateSides: true,
    fixedClock: false,
    legacyPage: "buildings",
    legacyQuery: { mode: "Flotte" },
    migratedPath: "/game/shipyard",
    legacyReady: "#bx",
    migratedReady: "#bx",
    actions: [{ type: "wait", waitMs: 19000 }],
    assertions: [
      { name: "shipyard-countdown", type: "text", selector: "#bx", compareSides: true, contains: "Tasks completed" },
      {
        name: "shipyard-completed-option",
        type: "text",
        selector: "select[name='auftr'] option",
        compareSides: true,
        contains: "Tasks completed"
      }
    ],
    notes: ["Covers short shipyard countdown completion and Expected tasks completed state without freezing Date."]
  },
  {
    name: "phalanx-event-countdown-decrements",
    fixedClock: false,
    legacyPage: "phalanx",
    legacyQuery: { cp: "$fixture.phalanx.source_moon_id", spid: "$fixture.phalanx.target_planet_id" },
    migratedPath: "/game/phalanx",
    migratedQuery: { cp: "$fixture.phalanx.source_moon_id", spid: "$fixture.phalanx.target_planet_id" },
    legacyReady: "#bxx1",
    migratedReady: "#bxx1",
    requiredFixtureFeatures: ["phalanx"],
    actions: [{ type: "wait", waitMs: 1500 }],
    assertions: [
      { name: "phalanx-event-count", type: "count", selector: ".phalanx_fleet", compareSides: true, expected: "2" },
      {
        name: "phalanx-countdown-seconds",
        type: "evaluate",
        expression:
          "(() => { const text = document.querySelector('#bxx1')?.textContent?.trim() ?? ''; const match = text.match(/^(\\d+):(\\d\\d):(\\d\\d)$/); return match ? Number(match[1]) * 3600 + Number(match[2]) * 60 + Number(match[3]) : -1; })()",
        compareSides: true,
        tolerance: 5
      },
      {
        name: "phalanx-countdown-decremented",
        type: "evaluate",
        expression:
          "(() => { const el = document.querySelector('#bxx1'); const text = el?.textContent?.trim() ?? ''; const match = text.match(/^(\\d+):(\\d\\d):(\\d\\d)$/); const shown = match ? Number(match[1]) * 3600 + Number(match[2]) * 60 + Number(match[3]) : -1; const initial = Number(el?.getAttribute('title') ?? '-1'); return initial > 0 && shown >= 0 && shown < initial; })()",
        expected: "true"
      }
    ],
    notes: ["Requires OGAME_GAME_VISUAL_PHALANX_FIXTURE=1; covers phalanx bxx countdown text changing while the legacy title stays at the initial duration."]
  },
  {
    name: "merchant-exchange-rate-tooltip",
    legacyPage: "trader",
    migratedPath: "/game/merchant",
    legacyReady: "#content table",
    migratedReady: ".legacy-merchant-call-table",
    applicabilitySelector: "input[name='2_value']",
    actions: [
      {
        type: "hover",
        legacySelector: "#content a[onmouseover*='One Metal gives'][onmouseover*='Crystal']",
        migratedSelector: ".legacy-merchant-exchange-table a[data-merchant-rate-id='2']",
        waitForSelector: "#overDiv"
      }
    ],
    assertions: [
      { name: "tooltip", type: "text", selector: "#overDiv", compareSides: true, contains: "One Metal gives" },
      { name: "tooltip-html", type: "html", selector: "#overDiv", compareSides: true }
    ],
    notes: ["Covers trader.php exchange-rate overlib tooltip text and frame parity."]
  },
  {
    name: "merchant-exchange-max-clamp",
    legacyPage: "trader",
    migratedPath: "/game/merchant",
    legacyReady: "#content table",
    migratedReady: ".legacy-merchant-call-table",
    applicabilitySelector: "input[name='2_value']",
    actions: [
      {
        type: "click",
        legacySelector: "input[name='2_value'] + a",
        migratedSelector: "input[name='2_value'] + a"
      }
    ],
    assertions: [
      { name: "crystal-value", type: "value", selector: "input[name='2_value']", compareSides: true, tolerance: 1 },
      { name: "crystal-storage", type: "text", selector: "[id='2_storage']", compareSides: true, tolerance: 1 }
    ],
    notes: ["Skipped unless the fixture currently has an active merchant exchange table."]
  },
  {
    name: "merchant-exchange-negative-clamp",
    legacyPage: "trader",
    migratedPath: "/game/merchant",
    legacyReady: "#content table",
    migratedReady: ".legacy-merchant-call-table",
    applicabilitySelector: "input[name='2_value']",
    actions: [{ type: "type", selector: "input[name='2_value']", value: "-123" }],
    assertions: [
      { name: "crystal-value", type: "value", selector: "input[name='2_value']", compareSides: true },
      { name: "crystal-storage", type: "text", selector: "[id='2_storage']", compareSides: true, tolerance: 1 }
    ],
    notes: ["Covers trader checkValue negative-input normalization."]
  },
  {
    name: "merchant-exchange-submit-success",
    isolateSides: true,
    legacyPage: "trader",
    migratedPath: "/game/merchant",
    legacyReady: "#content table",
    migratedReady: ".legacy-merchant-call-table",
    applicabilitySelector: "input[name='2_value']",
    actions: [
      { type: "fill", selector: "input[name='2_value']", value: "1000" },
      { type: "fill", selector: "input[name='3_value']", value: "500" },
      {
        type: "click",
        legacySelector: "#content input[name='trade']",
        migratedSelector: ".legacy-merchant-exchange-table input[name='trade']",
        legacyWaitForSelector: "#content form[name='TraderForm']",
        migratedWaitForSelector: ".legacy-merchant-call-table"
      }
    ],
    assertions: [
      { name: "merchant-not-found", type: "text", legacySelector: "#content form[name='TraderForm']", migratedSelector: ".legacy-merchant-call-table", contains: "Merchant not found!" },
      { name: "trade-button-count", type: "count", legacySelector: "#content input[name='trade']", migratedSelector: ".legacy-merchant-exchange-table input[name='trade']", expected: "0" }
    ],
    notes: ["Covers successful trader exchange submission consuming the active merchant offer."]
  }
];

export function selectGameDynamicBehaviorSpecs(filterValue: string): GameDynamicBehaviorSpec[] {
  const filter = filterValue
    .split(",")
    .map((name) => name.trim())
    .filter(Boolean);
  if (filter.length === 0) {
    return gameDynamicBehaviorSpecs;
  }
  const selected = gameDynamicBehaviorSpecs.filter((spec) => filter.includes(spec.name));
  const selectedNames = new Set(selected.map((spec) => spec.name));
  const missing = filter.filter((name) => !selectedNames.has(name));
  if (missing.length > 0) {
    throw new Error(`unknown authenticated game dynamic behavior filter: ${missing.join(", ")}`);
  }
  return selected;
}
