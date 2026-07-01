export type SideName = "legacy" | "migrated";

export type GameDynamicAction = {
  type: "click" | "fill" | "type" | "select" | "hover" | "press" | "wait";
  selector?: string;
  legacySelector?: string;
  migratedSelector?: string;
  value?: string;
  waitMs?: number;
};

export type GameDynamicAssertion = {
  name: string;
  type: "text" | "value" | "visible" | "count";
  selector?: string;
  legacySelector?: string;
  migratedSelector?: string;
  compareSides?: boolean;
  expected?: string;
  contains?: string;
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
      { name: "crystal-value", type: "value", selector: "input[name='2_value']", compareSides: true },
      { name: "crystal-storage", type: "text", selector: "#2_storage", compareSides: true }
    ],
    notes: ["Skipped unless the fixture currently has an active merchant exchange table."]
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
