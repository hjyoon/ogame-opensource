export type LocalTimeParts = {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
  weekday: string;
};

const formatterCache = new Map<string, Intl.DateTimeFormat>();
const localTimeLocale = "en-US-u-ca-gregory-nu-latn";

export function browserTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

export function localTimeParts(unixSeconds: number, timeZone = browserTimeZone()): LocalTimeParts {
  const date = new Date(unixSeconds * 1000);
  if (!Number.isFinite(unixSeconds) || Number.isNaN(date.getTime())) {
    throw new RangeError("invalid Unix timestamp");
  }

  const formatter = localTimeFormatter(timeZone);
  const values = new Map(formatter.formatToParts(date).map((part) => [part.type, part.value]));
  return {
    year: numberPart(values, "year"),
    month: numberPart(values, "month"),
    day: numberPart(values, "day"),
    hour: numberPart(values, "hour"),
    minute: numberPart(values, "minute"),
    second: numberPart(values, "second"),
    weekday: values.get("weekday") ?? "",
  };
}

function localTimeFormatter(timeZone: string): Intl.DateTimeFormat {
  const cached = formatterCache.get(timeZone);
  if (cached) {
    return cached;
  }
  const formatter = new Intl.DateTimeFormat(localTimeLocale, {
    timeZone,
    calendar: "gregory",
    numberingSystem: "latn",
    weekday: "short",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  });
  formatterCache.set(timeZone, formatter);
  return formatter;
}

function numberPart(values: Map<string, string>, key: string): number {
  const value = Number(values.get(key));
  if (!Number.isInteger(value)) {
    throw new RangeError(`missing ${key} date part`);
  }
  return value;
}
