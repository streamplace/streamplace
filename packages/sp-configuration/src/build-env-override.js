#!/usr/bin/env node

import config from "./sp-configuration";
import fs from "fs";
import { resolve } from "path";
config.loadValuesFile(process.argv[2]);
const str = `module.exports = ${JSON.stringify(config, null, 2)};`;
fs.writeFileSync(resolve(__dirname, "env-override.js"), str, "utf8");
