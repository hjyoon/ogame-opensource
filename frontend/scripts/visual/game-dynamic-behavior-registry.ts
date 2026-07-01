export type SideName = "legacy" | "migrated";

export type GameDynamicAction = {
  type: "click" | "fill" | "type" | "select" | "hover" | "press" | "wait";
  selector?: string;
  legacySelector?: string;
  migratedSelector?: string;
  value?: string;
  waitForSelector?: string;
  legacyWaitForSelector?: string;
  migratedWaitForSelector?: string;
  waitMs?: number;
};

export type GameDynamicAssertion = {
  name: string;
  type: "text" | "html" | "value" | "visible" | "count";
  selector?: string;
  legacySelector?: string;
  migratedSelector?: string;
  compareSides?: boolean;
  expected?: string;
  contains?: string;
  tolerance?: number;
};

export type GameDynamicBehaviorSpec = {
  name: string;
  legacyPage: string;
  legacyQuery?: Record<string, string>;
  migratedPath: string;
  migratedQuery?: Record<string, string>;
  legacyReady: string;
  migratedReady: string;
  applicabilitySelector?: string;
  legacyApplicabilitySelector?: string;
  migratedApplicabilitySelector?: string;
  requiredFixtureFeatures?: Array<"alliance" | "commander" | "phalanx" | "report">;
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
    name: "galaxy-instant-spy-dispatch-success",
    legacyPage: "galaxy",
    migratedPath: "/game/galaxy",
    legacyReady: "#content",
    migratedReady: ".legacy-galaxy-table",
    actions: [
      {
        type: "click",
        legacySelector: "#content a[onclick*='doit(6']",
        migratedSelector: ".legacy-galaxy-actions a[data-galaxy-action='Espionage']",
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
    name: "admin-battlesim-slot-sync",
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
