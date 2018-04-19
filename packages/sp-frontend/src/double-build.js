#!/usr/bin/env node

/**
 * So we need two builds of sp-frontend with marginal changes for electron. The one for deployment at https://stream.place/ uses a root of "/" and the one for Electon uses a root of ".". This does
 * that.
 */

/* eslint-disable no-console */

const child = require("child_process");
const fs = require("fs-extra");
const path = require("path");

const pkgRoot = path.resolve(__dirname, "..");
const oldPkgStr = fs.readFileSync(
  path.resolve(pkgRoot, "package.json"),
  "utf8"
);

fs.removeSync(path.resolve(pkgRoot, "build"));
fs.removeSync((pkgRoot, "build-electron"));
const newPkg = { ...JSON.parse(oldPkgStr), homepage: "." };
fs.writeFileSync(
  path.resolve(pkgRoot, "package.json"),
  JSON.stringify(newPkg, null, 2)
);
console.log("++++ building sp-frontend for electron ++++");
child.execSync("npm run prepare:build", { cwd: pkgRoot, stdio: "inherit" });
fs.renameSync(
  path.resolve(pkgRoot, "build"),
  path.resolve(pkgRoot, "build-electron")
);
fs.writeFileSync(path.resolve(pkgRoot, "package.json"), oldPkgStr);
console.log("++++ building sp-frontend for browsers ++++");
child.execSync("npm run prepare:build", { cwd: pkgRoot, stdio: "inherit" });
