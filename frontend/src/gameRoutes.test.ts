import { describe, expect, test } from "bun:test";
import { gameRouteURL, gameRoutes, normalizeGamePath, resolveGameRoute } from "./gameRoutes";

describe("game route model", () => {
  test("uses natural game route paths without php suffixes", () => {
    expect(gameRoutes.every((route) => route.path.startsWith("/game/"))).toBe(true);
    expect(gameRoutes.every((route) => !route.path.endsWith(".php"))).toBe(true);
    expect(gameRoutes.map((route) => route.path)).toContain("/game/buildings");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/fleet");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/merchant");
    expect(gameRoutes.map((route) => route.path)).toContain("/game/officers");
  });

  test("normalizes game paths", () => {
    expect(normalizeGamePath("/game")).toBe("/game/overview");
    expect(normalizeGamePath("/game/overview/")).toBe("/game/overview");
    expect(normalizeGamePath("/game/resources?session=abc")).toBe("/game/resources");
  });

  test("resolves natural authenticated game routes", () => {
    expect(resolveGameRoute("/game").key).toBe("overview");
    expect(resolveGameRoute("/game/overview").migrated).toBe(true);
    expect(resolveGameRoute("/game/buildings")).toMatchObject({ key: "buildings", migrated: true });
    expect(resolveGameRoute("/game/resources")).toMatchObject({ key: "resources", migrated: true });
    expect(resolveGameRoute("/game/options").label).toBe("Options");
    expect(resolveGameRoute("/game/messages").label).toBe("Messages");
  });

  test("falls back unknown game paths to overview", () => {
    expect(resolveGameRoute("/game/does-not-exist").key).toBe("overview");
  });

  test("preserves active session query parameters in menu links", () => {
    expect(gameRouteURL("/game/buildings", "?session=abc&cp=42")).toBe("/game/buildings?session=abc&cp=42");
    expect(gameRouteURL("/game/fleet", "session=abc")).toBe("/game/fleet?session=abc");
    expect(gameRouteURL("/game/overview", "")).toBe("/game/overview");
  });
});
