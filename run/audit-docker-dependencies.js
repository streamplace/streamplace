#!/usr/bin/env node

/* eslint-disable no-console */

const fs = require("fs");
const path = require("path");
const packages = fs.readdirSync(path.resolve(__dirname, "..", "packages"));
const prefix = "quay.io/streamplace";
const depcheck = require("depcheck");

let failed = false;
const graph = {};
const dirs = {};
for (const pkgName of packages) {
  graph[pkgName] = [];
  const pkgDir = path.resolve(__dirname, "..", "packages", pkgName);
  dirs[pkgName] = pkgDir;
  const pkg = JSON.parse(fs.readFileSync(path.resolve(pkgDir, "package.json")));
  const pkgDeps = new Set(
    [
      ...Object.keys(pkg.dependencies || []),
      ...Object.keys(pkg.devDependencies || [])
    ].filter(dep => packages.includes(dep))
  );
  graph[pkgName] = [...pkgDeps];
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

const dotFile = `
  digraph streamplace {
    ${Object.entries(graph)
      .map(([pkgName, deps]) =>
        deps.map(dep => `    "${dep}" -> "${pkgName}";`).join("\n")
      )
      .join("\n")}
  }
`;

fs.writeFileSync(path.resolve(__dirname, "..", "dependencies.dot"), dotFile);

(async () => {
  for (const pkgName of packages) {
    const unused = await new Promise((resolve, reject) =>
      depcheck(dirs[pkgName], {}, resolve)
    );
    Object.keys(unused.missing).forEach(missingDep => {
      failed = true;
      console.log(`⛔️ ${pkgName} is missing ${missingDep}`);
    });
    // most unused dependencies are for docker deps and stuff but we do care about sp-configuration
    if (
      unused.dependencies.includes("sp-configuration") ||
      unused.devDependencies.includes("sp-configuration")
    ) {
      failed = true;
      console.log(`⛔️ ${pkgName} has unnecessary sp-configuration`);
      console.log(path.resolve(dirs[pkgName], "package.json"));
    }
  }
})();

// process.exit(failed ? 1 : 0);
