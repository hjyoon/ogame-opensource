import { describe, expect, test } from "bun:test";
import { browserTimeZone, localTimeParts } from "./localTime";

describe("browser-local Unix timestamp conversion", () => {
  const timestamp = Date.parse("2026-08-05T00:15:30Z") / 1000;

  test("uses the requested IANA timezone including date rollover", () => {
    expect(localTimeParts(timestamp, "Asia/Seoul")).toEqual({
      year: 2026,
      month: 8,
      day: 5,
      hour: 9,
      minute: 15,
      second: 30,
      weekday: "Wed",
    });
    expect(localTimeParts(timestamp, "America/New_York")).toEqual({
      year: 2026,
      month: 8,
      day: 4,
      hour: 20,
      minute: 15,
      second: 30,
      weekday: "Tue",
    });
  });

  test("defaults to the browser timezone", () => {
    const timeZone = browserTimeZone();
    expect(timeZone.length).toBeGreaterThan(0);
    expect(localTimeParts(timestamp)).toEqual(localTimeParts(timestamp, timeZone));
  });

  test("rejects invalid timestamps", () => {
    expect(() => localTimeParts(Number.NaN, "UTC")).toThrow(RangeError);
  });
});
