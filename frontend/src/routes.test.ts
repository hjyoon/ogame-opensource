import { describe, expect, test } from "bun:test";
import {
  legacyPublicBootstrapPaths,
  legacyPublicCssHrefs,
  legacyPublicRouteKeys,
  normalizePath,
  publicRouteAliases,
  publicRoutes,
  resolvePublicRoute
} from "./routes";
import { publicRouteManifest } from "./publicRouteManifest";
import { gameMenuRouteURL, gamePlanetSwitchURL, gameRouteURL } from "./gameRoutes";

describe("public route model", () => {
  test("uses natural route paths without php suffixes", () => {
    expect(publicRoutes.every((route) => !route.path.endsWith(".php"))).toBe(true);
    expect(publicRoutes.map((route) => route.path)).toContain("/register");
    expect(publicRoutes.map((route) => route.path)).toContain("/universes");
  });

  test("normalizes public paths", () => {
    expect(normalizePath("/register/")).toBe("/register");
    expect(normalizePath("/universes?from=legacy")).toBe("/universes");
    expect(normalizePath("")).toBe("/");
  });

  test("resolves natural routes", () => {
    expect(resolvePublicRoute("/").route.key).toBe("home");
    expect(resolvePublicRoute("/home").route.key).toBe("home");
    expect(resolvePublicRoute("/rules").route.key).toBe("rules");
  });

  test("keeps legacy php paths as aliases only", () => {
    const home = resolvePublicRoute("/home.php");
    const register = resolvePublicRoute("/register.php");
    const impressum = resolvePublicRoute("/impressum.php");

    expect(home.route.key).toBe("home");
    expect(home.canonicalPath).toBe("/home");
    expect(home.isLegacyAlias).toBe(true);
    expect(register.route.key).toBe("register");
    expect(register.canonicalPath).toBe("/register");
    expect(impressum.route.key).toBe("legal");
  });

  test("derives aliases and bootstrap chrome from the route manifest", () => {
    expect(publicRouteAliases.get("/regeln.php")).toBe("/rules");
    expect(publicRouteAliases.get("/unis.php")).toBe("/universes");
    expect(publicRouteAliases.get("/impressum.php")).toBe("/legal");
    expect(legacyPublicRouteKeys.has("home")).toBe(true);
    expect(legacyPublicRouteKeys.has("legal")).toBe(false);
    for (const path of ["/", "/home", "/home.php", "/register", "/register.php", "/about.php", "/regeln.php", "/unis.php"]) {
      expect(legacyPublicBootstrapPaths).toContain(path);
    }
    expect(legacyPublicBootstrapPaths).not.toContain("/legal");
    expect(legacyPublicBootstrapPaths).not.toContain("/impressum.php");
    expect(new Set(legacyPublicBootstrapPaths).size).toBe(legacyPublicBootstrapPaths.length);
    expect(legacyPublicCssHrefs).toEqual(["/public-assets/css/styles.css", "/public-assets/css/about.css"]);
  });

  test("keeps visual parity targets tied to legacy aliases", () => {
    const visualRoutes = publicRouteManifest.filter((route) => route.legacyVisualPath !== undefined);
    expect(visualRoutes.map((route) => route.key)).toEqual(["home", "register", "universes", "about", "story", "screenshots", "rules", "legal"]);
    for (const route of visualRoutes) {
      const visualPath = route.legacyVisualPath ?? "";
      expect(route.legacyAliases).toContain(visualPath);
      expect(publicRouteAliases.get(visualPath)).toBe(route.path);
    }
  });

  test("unknown routes fall back to the migration console", () => {
    expect(resolvePublicRoute("/does-not-exist").route.key).toBe("migration");
  });
});

describe("game route URL model", () => {
  test("keeps game menu links free of stale screen-specific query parameters", () => {
    const search = "?session=abc&cp=42&mode=Users&player_id=7&tid=206&start=401";

    expect(gameMenuRouteURL("/game/buildings", search)).toBe("/game/buildings?session=abc&cp=42");
    expect(gameMenuRouteURL("/game/admin", search)).toBe("/game/admin?session=abc&cp=42");
  });

  test("keeps only the target route query parameters for action links", () => {
    expect(gameRouteURL("/game/admin", "?session=abc&cp=42&mode=Users&player_id=7&tid=206&a=5")).toBe(
      "/game/admin?session=abc&cp=42&mode=Users&player_id=7"
    );
    expect(gameRouteURL("/game/technology", "?session=abc&cp=42&tid=206&mode=Users")).toBe("/game/technology?session=abc&cp=42&tid=206");
    expect(gameRouteURL("/game/galaxy", "?session=abc&cp=42&galaxy=1&system=2&position=3&start=401")).toBe(
      "/game/galaxy?session=abc&cp=42&galaxy=1&system=2&position=3"
    );
    expect(gameRouteURL("/game/galaxy", "?session=abc&mode=Planets&galaxy=1&system=2")).toBe("/game/galaxy?session=abc&galaxy=1&system=2");
    expect(gameRouteURL("/game/galaxy", "?session=abc&mode=1&p1=1&p2=2&p3=3&pdd=4&zp=5")).toBe(
      "/game/galaxy?session=abc&mode=1&p1=1&p2=2&p3=3&pdd=4&zp=5"
    );
  });

  test("keeps planet selector URLs aligned with legacy drop-list query retention", () => {
    expect(gamePlanetSwitchURL("/game/alliance", "?session=abc&a=5&t=3&cp=1", 42)).toBe("/game/alliance?session=abc&cp=42");
    expect(gamePlanetSwitchURL("/game/messages", "?session=abc&messageziel=7&re=8", 42)).toBe("/game/messages?session=abc&cp=42");
    expect(gamePlanetSwitchURL("/game/admin", "?session=abc&mode=Users&player_id=7", 42)).toBe("/game/admin?session=abc&mode=Users&cp=42");
    expect(gamePlanetSwitchURL("/game/technology", "?session=abc&gid=206&tid=113", 42)).toBe(
      "/game/technology?session=abc&gid=206&tid=113&cp=42"
    );
  });
});
