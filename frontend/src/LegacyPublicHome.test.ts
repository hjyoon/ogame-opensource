import { describe, expect, test } from "bun:test";
import { legacyPublicUniverseActionURL } from "./LegacyPublicHome";

describe("legacy public home route helpers", () => {
  test("keeps local legacy universe actions on the migrated origin", () => {
    expect(legacyPublicUniverseActionURL("http://localhost:8888", "/game/reg/mail.php", "http://127.0.0.1:8890/home")).toBe(
      "/game/reg/mail.php"
    );
    expect(legacyPublicUniverseActionURL("http://127.0.0.1:8888", "/game/reg/mail.php", "http://10.8.0.2:8890/home")).toBe(
      "/game/reg/mail.php"
    );
  });

  test("preserves explicit remote universe action origins", () => {
    expect(legacyPublicUniverseActionURL("https://uni9902.example.test", "/game/reg/mail.php", "http://127.0.0.1:8890/home")).toBe(
      "https://uni9902.example.test/game/reg/mail.php"
    );
  });
});
