import { describe, expect, test } from "bun:test";
import { normalizePath, publicRoutes, resolvePublicRoute } from "./routes";

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

  test("unknown routes fall back to the migration console", () => {
    expect(resolvePublicRoute("/does-not-exist").route.key).toBe("migration");
  });
});
