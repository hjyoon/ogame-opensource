import { describe, expect, test } from "bun:test";
import { readAPIJSON } from "./apiResponse";

describe("readAPIJSON", () => {
  test("reads successful JSON responses", async () => {
    const payload = await readAPIJSON<{ ok: boolean }>(Response.json({ ok: true }), "options");
    expect(payload).toEqual({ ok: true });
  });

  test("surfaces JSON and legacy text errors without a JSON syntax exception", async () => {
    await expect(
      readAPIJSON(new Response('{"error":"Game options are temporarily unavailable."}', { status: 503 }), "options")
    ).rejects.toThrow("Game options are temporarily unavailable.");
    await expect(readAPIJSON(new Response("game options unavailable\n", { status: 503 }), "options")).rejects.toThrow(
      "game options unavailable"
    );
  });

  test("can parse explicitly allowed error statuses", async () => {
    const payload = await readAPIJSON<{ authenticated: boolean }>(
      Response.json({ authenticated: false }, { status: 401 }),
      "options",
      [401]
    );
    expect(payload.authenticated).toBe(false);
  });

  test("does not expose JSON parser implementation details", async () => {
    await expect(readAPIJSON(new Response("not-json"), "options")).rejects.toThrow("options returned an invalid JSON response");
  });
});
