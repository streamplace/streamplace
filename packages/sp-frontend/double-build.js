#!/usr/bin/env node

/**
 * So we need two builds of sp-frontend with marginal changes for electron. The one for deployment at https://stream.place/ uses a root of "/" and the one for Electon uses a root of ".". This does
 * that.
 */

/* eslint-disable no-console */

const child = require("child_process");
const fs = require("fs-extra");
const path = require("path");

const oldPkgStr = fs.readFileSync(
  path.resolve(__dirname, "package.json"),
  "utf8"
);

fs.removeSync(path.resolve(__dirname, "build"));
fs.removeSync((__dirname, "build-electron"));
const newPkg = { ...JSON.parse(oldPkgStr), homepage: "." };
fs.writeFileSync(
  path.resolve(__dirname, "package.json"),
  JSON.stringify(newPkg, null, 2)
);
console.log("++++ building sp-frontend for electron ++++");
child.execSync("npm run prepare:build", { cwd: __dirname, stdio: "inherit" });
fs.renameSync(
  path.resolve(__dirname, "build"),
  path.resolve(__dirname, "build-electron")
);
fs.writeFileSync(path.resolve(__dirname, "package.json"), oldPkgStr);
console.log("++++ building sp-frontend for browsers ++++");
child.execSync("npm run prepare:build", { cwd: __dirname, stdio: "inherit" });
