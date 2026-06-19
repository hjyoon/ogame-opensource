import { copyFile, cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { legacyPublicBootstrapPaths, legacyPublicCssHrefs } from "../src/routes";

const root = new URL("../", import.meta.url);
const dist = new URL("dist/", root);
const assets = new URL("assets/", dist);
const publicAssets = new URL("public-assets/", dist);
const legacyPublicImages = new URL("../wwwroot/img/", root);
const legacyPublicCss = new URL("../wwwroot/css/", root);
const legacyEvolution = new URL("../wwwroot/evolution/", root);
const legacyGameCss = new URL("../game/css/", root);
const legacyGameImages = new URL("../game/img/", root);
const legacyFavicon = new URL("../wwwroot/favicon.ico", root);

await rm(fileURLToPath(dist), { force: true, recursive: true });
await mkdir(fileURLToPath(assets), { recursive: true });
await mkdir(fileURLToPath(publicAssets), { recursive: true });

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

const indexHtml = await readFile(fileURLToPath(new URL("src/index.html", root)), "utf8");
const legacyPublicBootstrapMap = Object.fromEntries(legacyPublicBootstrapPaths.map((path) => [path, true]));
await writeFile(
  fileURLToPath(new URL("index.html", dist)),
  indexHtml
    .replace(
      '/* __OGAME_LEGACY_PUBLIC_BOOTSTRAP_PATHS__ */ { "/": true }',
      JSON.stringify(legacyPublicBootstrapMap, null, 10)
    )
    .replace(
      '/* __OGAME_LEGACY_PUBLIC_CSS_HREFS__ */ ["/public-assets/css/styles.css", "/public-assets/css/about.css"]',
      JSON.stringify(legacyPublicCssHrefs, null, 10)
    )
);
await copyFile(
  fileURLToPath(legacyFavicon),
  fileURLToPath(new URL("favicon.ico", dist))
);

await cp(
  fileURLToPath(legacyPublicImages),
  fileURLToPath(new URL("img/", publicAssets)),
  { recursive: true }
);

await cp(
  fileURLToPath(legacyPublicCss),
  fileURLToPath(new URL("css/", publicAssets)),
  { recursive: true }
);

await cp(
  fileURLToPath(legacyEvolution),
  fileURLToPath(new URL("evolution/", publicAssets)),
  { recursive: true }
);

await cp(
  fileURLToPath(legacyEvolution),
  fileURLToPath(new URL("evolution/", dist)),
  { recursive: true }
);

await cp(
  fileURLToPath(legacyGameCss),
  fileURLToPath(new URL("game/css/", publicAssets)),
  { recursive: true }
);

await cp(
  fileURLToPath(legacyGameImages),
  fileURLToPath(new URL("game/img/", publicAssets)),
  { recursive: true }
);

await cp(
  fileURLToPath(legacyGameImages),
  fileURLToPath(new URL("game-img/", publicAssets)),
  { recursive: true }
);
