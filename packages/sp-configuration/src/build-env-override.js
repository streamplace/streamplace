#!/usr/bin/env node

import config from "./sp-configuration";
import fs from "fs";
import { resolve } from "path";
for (const file of process.argv.slice(2)) {
  config.loadValuesFile(process.argv[2]);
}
const str = `module.exports = ${JSON.stringify(config, null, 2)};`;
const filePath = resolve(__dirname, "env-override.js");
let currentFile;
try {
  currentFile = fs.readFileSync(filePath, "utf8");
} catch (e) {
  // nbd, we'll just write it anyway
}
if (currentFile !== str) {
  fs.writeFileSync(resolve(__dirname, "env-override.js"), str, "utf8");
}
