const path = Bun.argv[2];
if (!path) {
  throw new Error("usage: bun admin-loca-normalize.ts <html-file>");
}

const html = await Bun.file(path).text();
const files = [...html.matchAll(/<h2>([\s\S]*?)<\/h2>([\s\S]*?)(?=<h2>|$)/gi)]
  .map((match) => {
    const rows = [...match[2].matchAll(
      /<tr><td\s+style="background-color:\s*(green|orange|red);">([\s\S]*?)<\/td><td\s+style="background-color:\s*\1;">\s*<pre>([\s\S]*?)<\/pre><\/td><td\s+style="background-color:\s*\1;">\s*<pre>([\s\S]*?)<\/pre><\/td><\/tr>/gi
    )].map((row) => ({
      key: decodeHTML(row[2]),
      source: decodeHTML(row[3]),
      target: decodeHTML(row[4]),
      status: row[1] === "orange" ? "same" : row[1] === "red" ? "missing" : "ok"
    }));
    return {
      name: decodeHTML(match[1]),
      targetMissing: /The file is not localized!/i.test(match[2]),
      rows
    };
  })
  .filter((file) => file.targetMissing || file.rows.length > 0);

process.stdout.write(JSON.stringify(files));

function decodeHTML(value: string): string {
  return value.replace(/&(?:#(\d+)|#x([0-9a-f]+)|([a-z]+));/gi, (entity, decimal, hexadecimal, named) => {
    if (decimal) return String.fromCodePoint(Number(decimal));
    if (hexadecimal) return String.fromCodePoint(Number.parseInt(hexadecimal, 16));
    switch (String(named).toLowerCase()) {
      case "amp": return "&";
      case "lt": return "<";
      case "gt": return ">";
      case "quot": return '"';
      case "apos": return "'";
      default: return entity;
    }
  });
}
