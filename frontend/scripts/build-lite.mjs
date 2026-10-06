import { createHash } from "node:crypto";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { gzipSync } from "node:zlib";
import { build } from "esbuild";

await mkdir("internal/site/assets", { recursive: true });
await build({
  entryPoints: { appearance: "frontend/src/appearance.ts", lite: "frontend/src/lite.tsx" },
  outdir: "internal/site/assets", bundle: true, minify: true, format: "esm", target: ["es2022"],
  legalComments: "eof", define: { "process.env.NODE_ENV": '"production"' }, logLevel: "info",
});
const colors = JSON.parse(await readFile("frontend/src/colors.json", "utf8"));
let html = await readFile("frontend/pages/lite.html", "utf8");
html = html.replaceAll("__LIGHT_BACKGROUND__", colors.light.background.default).replaceAll("__DARK_BACKGROUND__", colors.dark.background.default);
for (const name of ["lite", "appearance"]) {
  const data = await readFile(`internal/site/assets/${name}.js`);
  await writeFile(`internal/site/assets/${name}.js.gz`, gzipSync(data, {level: 9}));
  html = html.replaceAll(`__${name.toUpperCase()}_VERSION__`, createHash("sha256").update(data).digest("hex").slice(0,12));
}
await writeFile("internal/site/assets/lite.html", html);
