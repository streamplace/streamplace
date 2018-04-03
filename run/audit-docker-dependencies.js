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
const allDockerDeps = {};
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
  const re = new RegExp(`${prefix}/([a-z-]+)`);
  allDockerDeps[pkgName] = new Set();
  dockerfile.split("\n").forEach(line => {
    const results = re.exec(line);
    if (results) {
      allDockerDeps[pkgName].add(results[1]);
    }
  });
}

for (const pkgName of packages) {
  const pkgDeps = graph[pkgName];
  const dockerDeps = allDockerDeps[pkgName];
  if (!dockerDeps) {
    continue;
  }
  console.log();
  console.log(pkgName);
  console.log(
    [...pkgDeps]
      .map(dep => {
        if (dockerDeps.has(dep)) {
          return `  ✅  package.json: ${dep}`;
        } else {
          failed = true;
          return `  ⛔️  package.json: ${dep}`;
        }
      })
      .join("\n")
  );
  console.log(
    [...dockerDeps]
      .map(dep => {
        if (pkgDeps.includes(dep)) {
          return `  ✅  Dockerfile: ${prefix}/${dep}`;
        } else {
          failed = true;
          return `  ⛔️  Dockerfile: ${prefix}/${dep}`;
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
      depcheck(
        dirs[pkgName],
        {
          ignoreDirs: ["dist", "build", "node_modules"]
        },
        resolve
      )
    );
    Object.keys(unused.missing).forEach(missingDep => {
      failed = true;
      console.log(`⛔️ ${pkgName} is missing ${missingDep}`);
    });
    // most unused dependencies are for docker deps and stuff but we do care about some
    const IMPORTANT_UNUSED = ["sp-configuration", "sp-streams"];
    IMPORTANT_UNUSED.forEach(dep => {
      if (
        unused.dependencies.includes(dep) ||
        unused.devDependencies.includes(dep)
      ) {
        failed = true;
        console.log(`⛔️ ${pkgName} has unnecessary ${dep}`);
        console.log(path.resolve(dirs[pkgName], "package.json"));
      }
    });
  }
  process.exit(failed ? 1 : 0);
})();
