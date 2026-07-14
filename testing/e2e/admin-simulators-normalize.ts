const [mode, file] = process.argv.slice(2);

if (!mode || !file) {
  throw new Error("usage: admin-simulators-normalize.ts <rocket|expedition|battle-legacy|battle-go|battle-report> <file>");
}

const html = await Bun.file(file).text();

if (mode === "rocket") {
  const names = ["a_weap", "d_armor", "anz", "pziel", "d_401", "d_402", "d_403", "d_404", "d_405", "d_406", "d_407", "d_408", "d_502", "d_503"];
  const values: Record<string, number> = {};
  for (const name of names) {
    if (name === "pziel") {
      const select = html.match(/<select\s+name=["']?pziel["']?[^>]*>([\s\S]*?)<\/select>/i)?.[1] ?? "";
      const selected = [...select.matchAll(/<option\s+([^>]*)>/gi)].find((match) => /\bselected\b/i.test(match[1] ?? ""));
      values[name] = Number(attribute(selected?.[1] ?? "", "value")) || 0;
      continue;
    }
    const input = [...html.matchAll(/<input\s+([^>]*)>/gi)].find((match) => attribute(match[1] ?? "", "name") === name);
    values[name] = Number(attribute(input?.[1] ?? "", "value")) || 0;
  }
  console.log(JSON.stringify(values));
} else if (mode === "expedition") {
  const raw = html.match(/var\s+yValues\s*=\s*\[([^\]]*)\]/i)?.[1] ?? "";
  const series = raw === "" ? [] : raw.split(",").map((value) => Number(value.trim()) || 0);
  console.log(JSON.stringify(series));
} else if (mode === "battle-legacy") {
  console.log(JSON.stringify(battleResult(html, battleLegacyValues(html), html)));
} else if (mode === "battle-go") {
  const payload = JSON.parse(html);
  const result = payload?.actionIssue?.result ?? {};
  console.log(JSON.stringify(battleResult(String(result.html ?? ""), battleValues(result.values ?? {}), String(result.diagnosticsHtml ?? ""))));
} else if (mode === "battle-report") {
  const [encoded = "", pm = "", from = "", subject = "", shown = "", planetID = "", count = "", battleRows = ""] = html.trim().split("\t");
  const report = encoded === "" ? "" : Buffer.from(encoded, "base64").toString("utf8");
  console.log(JSON.stringify({
    pm: Number(pm), from, subject, shown: Number(shown), planetId: Number(planetID), count: Number(count), battleRows: Number(battleRows),
    report: report
      .replace(/At \d{2}-\d{2} \d{2}:\d{2}:\d{2}/g, "At <TIME>")
      .replace(/showGalaxy\(\d+,\d+,\d+\)/g, "showGalaxy(<COORD>)")
      .replace(/\[\d+:\d+:\d+\]/g, "[<COORD>]")
  }));
} else {
  throw new Error(`unsupported mode: ${mode}`);
}

function battleResult(resultHTML: string, values: Record<string, number>, diagnosticsHTML: string) {
  const match = resultHTML.match(/<span\s+class=["']?([^"'\s>]+)["']?[^>]*>[^<]*\[[^\]]+\]\s*\(V:([^,]+),A:([^\)]+)\)<\/span>/i);
  return {
    values,
    link: match ? { className: match[1] ?? "", defenderLoss: legacyNumber(match[2] ?? "0"), attackerLoss: legacyNumber(match[3] ?? "0") } : null,
    debug: values.debug ? battleDebugContract(diagnosticsHTML) : null
  };
}

function battleDebugContract(source: string) {
  const normalized = source.replaceAll("&gt;", ">").replaceAll("&#34;", '"').replaceAll("&quot;", '"');
  return {
    attacker: /\[oname\]\s*=>\s*Attacker0/.test(normalized),
    defender: /\[oname\]\s*=>\s*Defender0/.test(normalized),
    source: /<pre[^>]*>[\s\S]*MaxRound\s*=\s*\d+[\s\S]*Attacker0\s*=/i.test(normalized),
    battle: /\[source\]\s*=>[\s\S]*\[title\]\s*=>[\s\S]*\[date\]\s*=>\s*\d+/i.test(normalized),
    rounds: /\[rounds\]\s*=>\s*Array/i.test(normalized),
    result: /\[result\]\s*=>\s*(?:awon|dwon|draw)/i.test(normalized),
    separators: (normalized.match(/<hr\b/gi) ?? []).length >= 4
  };
}

function battleLegacyValues(source: string): Record<string, number> {
  const values: Record<string, number> = {};
  for (const match of source.matchAll(/<input\s+([^>]*)>/gi)) {
    const attrs = match[1] ?? "";
    const name = attribute(attrs, "name");
    if (!battleValueName(name)) continue;
    const type = attribute(attrs, "type").toLowerCase();
    values[name] = type === "checkbox" ? (/\bchecked\b/i.test(attrs) ? 1 : 0) : (Number(attribute(attrs, "value")) || 0);
  }
  return battleValues(values);
}

function battleValues(source: Record<string, unknown>): Record<string, number> {
  const result: Record<string, number> = {};
  for (const [name, raw] of Object.entries(source)) {
    const value = Number(raw) || 0;
    if (!battleValueName(name)) continue;
    if (["anum", "dnum", "rapid", "debug", "fid", "did", "max_round"].includes(name) || value !== 0) result[name] = value;
  }
  return Object.fromEntries(Object.entries(result).sort(([left], [right]) => left.localeCompare(right)));
}

function battleValueName(name: string): boolean {
  return ["anum", "dnum", "rapid", "debug", "fid", "did", "max_round"].includes(name) || /^[ad]\d+_(?:weap|shld|armor|\d+)$/.test(name);
}

function legacyNumber(value: string): number {
  return Number(value.replaceAll(".", "").replaceAll(",", "")) || 0;
}

function attribute(attributes: string, name: string): string {
  const quoted = attributes.match(new RegExp(`(?:^|\\s)${name}\\s*=\\s*(["'])(.*?)\\1`, "i"));
  if (quoted) {
    return quoted[2] ?? "";
  }
  return attributes.match(new RegExp(`(?:^|\\s)${name}\\s*=\\s*([^\\s>]+)`, "i"))?.[1] ?? "";
}
