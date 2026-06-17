import { copyFile, mkdir, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const dist = new URL("dist/", root);
const assets = new URL("assets/", dist);

await rm(fileURLToPath(dist), { force: true, recursive: true });
await mkdir(fileURLToPath(assets), { recursive: true });

const result = await Bun.build({
  entrypoints: [fileURLToPath(new URL("src/main.tsx", root))],
  minify: true,
  naming: {
    asset: "[name]-[hash].[ext]",
    chunk: "[name]-[hash].[ext]",
    entry: "[name].[ext]"
  },
  outdir: fileURLToPath(assets),
  sourcemap: "external",
  target: "browser"
});

if (!result.success) {
  for (const log of result.logs) {
    console.error(log);
  }
  process.exit(1);
}

await copyFile(
  fileURLToPath(new URL("src/index.html", root)),
  fileURLToPath(new URL("index.html", dist))
);
