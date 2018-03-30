#!/usr/bin/env node

/* eslint-disable no-console */

const fs = require("fs");
const path = require("path");
const packages = fs.readdirSync(path.resolve(__dirname, "..", "packages"));
const prefix = "quay.io/streamplace";

let failed = false;
const graph = {};
for (const pkgName of packages) {
  graph[pkgName] = [];
  const pkgDir = path.resolve(__dirname, "..", "packages", pkgName);
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

process.exit(failed ? 1 : 0);
