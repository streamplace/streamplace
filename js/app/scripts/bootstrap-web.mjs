import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import {
  borderRadius,
  motion,
  spacing,
  surfaces,
  textAlphas,
  touchTargets,
  typeScale,
} from "../../components/src/lib/theme/tokens.ts";

// Expo's shared chunk is eager even when every app screen is lazy. Keep its
// generated bundles intact, but let a tiny bootloader start them after paint.
const output = process.argv[2] ?? "dist";
const htmlPath = path.join(output, "index.html");
let html = await readFile(htmlPath, "utf8");
const scripts = [];
html = html.replace(
  /<script\b[^>]*\bsrc="([^"]+)"[^>]*>\s*<\/script>/g,
  (_, src) => {
    scripts.push(src);
    return "";
  },
);
if (!scripts.length || !html.includes('<div id="root"></div>')) {
  throw new Error("Expected Expo SPA HTML with scripts and an empty #root");
}

const code = await readFile(new URL("./web-bootstrap.js", import.meta.url));
const hash = createHash("sha256").update(code).digest("hex").slice(0, 16);
const filename = `bootstrap-${hash}.js`;
await writeFile(path.join(output, "_expo/static/js/web", filename), code);

const css = `
#web-bootstrap {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: ${spacing[4]}px; padding: ${spacing[6]}px;
  background: ${surfaces.dark[0]}; color: ${textAlphas.dark[1]};
  font: ${typeScale.base.fontSize}px/${typeScale.base.lineHeight}px system-ui, sans-serif;
  text-align: center;
}
#bootstrap-spinner {
  width: ${spacing[8]}px; height: ${spacing[8]}px;
  border: ${spacing[1]}px solid ${textAlphas.dark[4]}; border-top-color: currentColor;
  border-radius: ${borderRadius.full}px;
  animation: bootstrap-spin ${motion.slow * 3}ms linear infinite;
}
#web-bootstrap [hidden] { display: none; }
#bootstrap-retry {
  font: inherit; color: inherit; background: ${surfaces.dark[2]};
  padding: ${spacing[2]}px ${spacing[4]}px; border: none;
  border-radius: ${borderRadius.md}px; min-height: ${touchTargets.minimum}px; cursor: pointer;
}
#bootstrap-retry:focus-visible { outline: ${spacing[1] / 2}px solid currentColor; outline-offset: ${spacing[1] / 2}px; }
@keyframes bootstrap-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { #bootstrap-spinner { animation: none; } }
`;
html = html.replace("</head>", `<style>${css}</style></head>`);
html = html.replace(
  '<div id="root"></div>',
  `<div id="root"><div id="web-bootstrap">
    <div id="bootstrap-spinner" aria-hidden="true"></div>
    <div id="bootstrap-status" role="status" aria-live="polite">Loading Streamplace</div>
    <button id="bootstrap-retry" type="button" hidden>Try again</button>
  </div></div>`,
);
// Escape '<' so a filename cannot terminate the JSON script element.
const manifest = JSON.stringify(scripts).replace(/</g, "\\u003c");
// Match Expo's asset base URL rather than assuming deployment at the host root.
const baseUrl = scripts[0].slice(0, scripts[0].lastIndexOf("/") + 1);
html = html.replace(
  "</body>",
  `<script id="bootstrap-scripts" type="application/json">${manifest}</script>
<script src="${baseUrl}${filename}" defer></script></body>`,
);
await writeFile(htmlPath, html);
console.log(
  `Web bootstrap: ${code.length} initial JS bytes; ${scripts.length} Expo scripts deferred until after paint`,
);
