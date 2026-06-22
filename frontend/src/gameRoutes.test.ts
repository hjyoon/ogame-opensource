import { describe, expect, test } from "bun:test";
import {
  gameBuddyRequestURL,
  gameFleetTargetPrefillFromSearch,
  gameFleetTargetURL,
  gameGalaxyMissileURL,
  gameMessageComposeURL,
  gamePlanetSwitchURL,
  gameRouteURL,
  gameRoutes,
  normalizeGamePath,
  resolveGameRoute
} from "./gameRoutes";

describe("game route model", () => {
  test("uses natural game route paths without php suffixes", () => {
    expect(gameRoutes.every((route) => route.path.startsWith("/game/"))).toBe(true);
    expect(gameRoutes.every((route) => !route.path.endsWith(".php"))).toBe(true);
    expect(gameRoutes.map((route) => route.path)).toContain("/game/buildings");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/rename-planet");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/fleet");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/fleet-templates");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/report");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/merchant");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/officers");
  });

  test("normalizes game paths", () => {
    expect(normalizeGamePath("/game")).toBe("/game/overview");
    expect(normalizeGamePath("/game/overview/")).toBe("/game/overview");
    expect(normalizeGamePath("/game/resources?session=abc")).toBe("/game/resources");
    expect(normalizeGamePath("/game/index.php", "?page=b_building&session=abc")).toBe("/game/buildings");
    expect(normalizeGamePath("/game/index.php", "?page=buildings&mode=Forschung&session=abc")).toBe("/game/research");
    expect(normalizeGamePath("/game/index.php", "?page=buildings&mode=Flotte&session=abc")).toBe("/game/shipyard");
    expect(normalizeGamePath("/game/index.php", "?page=buildings&mode=Verteidigung&session=abc")).toBe("/game/defense");
    expect(normalizeGamePath("/game/index.php", "?page=fleet_templates&session=abc")).toBe("/game/fleet-templates");
    expect(normalizeGamePath("/game/index.php", "?page=flottenversand&session=abc")).toBe("/game/fleet");
    expect(normalizeGamePath("/game/index.php?page=flotten1&session=abc")).toBe("/game/fleet");
    expect(normalizeGamePath("/game/index.php?page=writemessages&messageziel=42&session=abc")).toBe("/game/messages");
    expect(normalizeGamePath("/game/index.php?page=bericht&bericht=11&session=abc")).toBe("/game/report");
    expect(normalizeGamePath("/game/index.php?page=notizen&session=abc")).toBe("/game/notes");
    expect(normalizeGamePath("/game/index.php?page=options&session=abc")).toBe("/game/options");
    expect(normalizeGamePath("/game/index.php?page=suche&session=abc")).toBe("/game/search");
    expect(normalizeGamePath("/game/index.php?page=techtree&session=abc")).toBe("/game/technology");
  });

  test("resolves natural authenticated game routes", () => {
    expect(resolveGameRoute("/game").key).toBe("overview");
    expect(resolveGameRoute("/game/overview").migrated).toBe(true);
    expect(resolveGameRoute("/game/rename-planet")).toMatchObject({ key: "renamePlanet", migrated: true });
    expect(resolveGameRoute("/game/buildings")).toMatchObject({ key: "buildings", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=buildings&mode=Forschung")).toMatchObject({ key: "research", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=buildings&mode=Flotte")).toMatchObject({ key: "shipyard", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=buildings&mode=Verteidigung")).toMatchObject({ key: "defense", migrated: true });
    expect(resolveGameRoute("/game/resources")).toMatchObject({ key: "resources", migrated: true });
    expect(resolveGameRoute("/game/merchant")).toMatchObject({ key: "merchant", migrated: true });
    expect(resolveGameRoute("/game/research")).toMatchObject({ key: "research", migrated: true });
    expect(resolveGameRoute("/game/shipyard")).toMatchObject({ key: "shipyard", migrated: true });
    expect(resolveGameRoute("/game/fleet")).toMatchObject({ key: "fleet", migrated: true });
    expect(resolveGameRoute("/game/fleet-templates")).toMatchObject({ key: "fleetTemplates", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=fleet_templates")).toMatchObject({ key: "fleetTemplates", migrated: true });
    expect(resolveGameRoute("/game/galaxy")).toMatchObject({ key: "galaxy", migrated: true });
    expect(resolveGameRoute("/game/defense")).toMatchObject({ key: "defense", migrated: true });
    expect(resolveGameRoute("/game/technology")).toMatchObject({ key: "technology", migrated: true });
    expect(resolveGameRoute("/game/statistics")).toMatchObject({ key: "statistics", migrated: true });
    expect(resolveGameRoute("/game/search")).toMatchObject({ key: "search", migrated: true });
    expect(resolveGameRoute("/game/buddy")).toMatchObject({ key: "buddy", migrated: true });
    expect(resolveGameRoute("/game/notes")).toMatchObject({ key: "notes", migrated: true });
    expect(resolveGameRoute("/game/logout")).toMatchObject({ key: "logout", migrated: true });
    expect(resolveGameRoute("/game/options")).toMatchObject({ key: "options", label: "Options", migrated: true });
    expect(resolveGameRoute("/game/messages")).toMatchObject({ key: "messages", migrated: true });
    expect(resolveGameRoute("/game/report")).toMatchObject({ key: "report", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=bericht&bericht=11")).toMatchObject({ key: "report", migrated: true });
  });

  test("maps legacy php pages to migrated or pending natural routes", () => {
    expect(resolveGameRoute("/game/index.php", "?page=b_building")).toMatchObject({ key: "buildings", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=notizen")).toMatchObject({ key: "notes", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=options")).toMatchObject({ key: "options", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=suche")).toMatchObject({ key: "search", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=techtree")).toMatchObject({ key: "technology", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=allianzen")).toMatchObject({ key: "alliance", migrated: false });
    expect(resolveGameRoute("/game/index.php", "?page=imperium")).toMatchObject({ key: "empire", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=trader")).toMatchObject({ key: "merchant", migrated: true });
    expect(resolveGameRoute("/game/index.php", "?page=micropayment")).toMatchObject({ key: "officers", migrated: false });
    expect(resolveGameRoute("/game/index.php", "?page=admin")).toMatchObject({ key: "admin", migrated: false });
  });

  test("falls back unknown game paths to overview", () => {
    expect(resolveGameRoute("/game/does-not-exist").key).toBe("overview");
  });

  test("preserves active session query parameters in menu links", () => {
    expect(gameRouteURL("/game/buildings", "?session=abc&cp=42")).toBe("/game/buildings?session=abc&cp=42");
    expect(gameRouteURL("/game/fleet", "session=abc")).toBe("/game/fleet?session=abc");
    expect(gameRouteURL("/game/buildings", "?session=abc&lgn=1&cp=42")).toBe("/game/buildings?session=abc&cp=42");
    expect(gameRouteURL("/game/overview", "")).toBe("/game/overview");
  });

  test("switches planets without leaving the active game page", () => {
    expect(gamePlanetSwitchURL("/game/buildings", "?session=abc&cp=1", 42)).toBe("/game/buildings?session=abc&cp=42");
    expect(gamePlanetSwitchURL("/game/technology", "?session=abc&tid=206", "7")).toBe("/game/technology?session=abc&tid=206&cp=7");
    expect(gamePlanetSwitchURL("/game/does-not-exist", "?session=abc", 9)).toBe("/game/overview?session=abc&cp=9");
  });

  test("builds galaxy action links with legacy fleet target parameters", () => {
    expect(gameFleetTargetURL({ galaxy: 1, system: 2, position: 3, mission: 1 }, "?session=abc&cp=99")).toBe(
      "/game/fleet?session=abc&cp=99&galaxy=1&system=2&position=3&planet=3&planettype=1&target_mission=1"
    );
    expect(gameFleetTargetURL({ galaxy: 1, system: 2, position: 3, mission: 3, planetType: 3 }, "?session=abc")).toBe(
      "/game/fleet?session=abc&galaxy=1&system=2&position=3&planet=3&planettype=3&target_mission=3"
    );
    expect(gameFleetTargetURL({ galaxy: 1, system: 2, position: 3, mission: 8, planetType: 2 }, "?session=abc")).toBe(
      "/game/fleet?session=abc&galaxy=1&system=2&position=3&planet=3&planettype=2&target_mission=8"
    );
    expect(gameFleetTargetURL({ galaxy: 1, system: 2, position: 16, mission: 15 }, "?session=abc&lgn=1")).toBe(
      "/game/fleet?session=abc&galaxy=1&system=2&position=16&planet=16&planettype=1&target_mission=15"
    );
  });

  test("parses legacy fleet target prefill values from galaxy links", () => {
    expect(gameFleetTargetPrefillFromSearch("?session=abc&galaxy=1&system=2&planet=3&planettype=2&target_mission=8")).toEqual({
      targetGalaxy: 1,
      targetSystem: 2,
      targetPlanet: 3,
      targetPlanetType: 2,
      targetMission: 8
    });
    expect(gameFleetTargetPrefillFromSearch("?session=abc&galaxy=-1&system=bad&position=16")).toEqual({
      targetGalaxy: 1,
      targetSystem: 0,
      targetPlanet: 16,
      targetPlanetType: 0,
      targetMission: 0
    });
    expect(gameFleetTargetPrefillFromSearch("?session=abc")).toBeNull();
  });

  test("builds migrated galaxy user action links", () => {
    expect(gameBuddyRequestURL(42, "?session=abc&cp=99&lgn=1")).toBe("/game/buddy?session=abc&cp=99&action=7&buddy_id=42");
    expect(gameMessageComposeURL(42, "?session=abc&cp=99")).toBe("/game/messages?session=abc&cp=99&messageziel=42");
    expect(gameGalaxyMissileURL({ galaxy: 1, system: 2, position: 3 }, 77, 42, "?session=abc&cp=99")).toBe(
      "/game/galaxy?session=abc&cp=99&mode=1&p1=1&p2=2&p3=3&pdd=77&zp=42&galaxy=1&system=2&position=3"
    );
  });
});
