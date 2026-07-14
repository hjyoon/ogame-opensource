const [mode, file] = process.argv.slice(2);

if (!mode || !file) {
  throw new Error("usage: admin-simulators-normalize.ts <rocket|expedition> <html>");
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
} else {
  throw new Error(`unsupported mode: ${mode}`);
}

function attribute(attributes: string, name: string): string {
  const quoted = attributes.match(new RegExp(`(?:^|\\s)${name}\\s*=\\s*(["'])(.*?)\\1`, "i"));
  if (quoted) {
    return quoted[2] ?? "";
  }
  return attributes.match(new RegExp(`(?:^|\\s)${name}\\s*=\\s*([^\\s>]+)`, "i"))?.[1] ?? "";
}
