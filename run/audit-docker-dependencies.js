#!/usr/bin/env node

/* eslint-disable no-console */

const fs = require("fs");
const path = require("path");
const packages = fs.readdirSync(path.resolve(__dirname, "..", "packages"));
const prefix = "quay.io/streamplace";

let failed = false;
for (const pkgName of packages) {
  const pkgDir = path.resolve(__dirname, "..", "packages", pkgName);
  const pkg = JSON.parse(fs.readFileSync(path.resolve(pkgDir, "package.json")));
  const pkgDeps = new Set(
    [
      ...Object.keys(pkg.dependencies || []),
      ...Object.keys(pkg.devDependencies || [])
    ].filter(dep => packages.includes(dep))
  );
  let dockerfile;
  try {
    dockerfile = fs.readFileSync(path.resolve(pkgDir, "Dockerfile"), "utf8");
  } catch (e) {
    if (e.code !== "ENOENT") {
      throw e;
    }
    continue;
  }
  console.log();
  console.log(pkgName);
  console.log(
    [...pkgDeps]
      .map(dep => {
        if (dockerfile.includes(`${prefix}/${dep}`)) {
          return `  ✅  ${dep}`;
        } else {
          failed = true;
          return `  ⛔️  ${dep}`;
        }
      })
      .join("\n")
  );
}

process.exit(failed ? 1 : 0);
