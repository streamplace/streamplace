// Copies the built web app into the worker's asset directory, minus what a
// node serves (native OTA bundles) and source maps (the web map alone is over
// Cloudflare's per-file limit).

import { cpSync, existsSync, readFileSync, rmSync } from "node:fs";
import path from "node:path";

const source = path.resolve(import.meta.dirname, "../../app/dist");
const target = path.resolve(import.meta.dirname, "../assets");

const index = path.join(source, "index.html");
if (!existsSync(index)) {
  throw new Error(`${index} missing — run make app`);
}
const html = readFileSync(index, "utf8");
if (!html.includes("<!-- sp-head -->")) {
  throw new Error(`${index} has no <!-- sp-head --> markers — run make app`);
}

const skip = (src) => {
  if (src.endsWith(".map")) return false;
  const rel = path.relative(source, src).split(path.sep).join("/");
  return (
    rel !== "_expo/static/js/ios" &&
    !rel.startsWith("_expo/static/js/ios/") &&
    rel !== "_expo/static/js/android" &&
    !rel.startsWith("_expo/static/js/android/")
  );
};

rmSync(target, { recursive: true, force: true });
cpSync(source, target, { recursive: true, filter: skip });
console.log(`copied ${source} -> ${target}`);
