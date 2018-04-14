#!/usr/bin/env node

/**
 * Hello I generate the Dockerfile for stream.place/streamplace I am so great look at me go
 */

const PACKAGE_BLACKLIST = ["sp-native"];

const fs = require("fs");
const path = require("path");
const output = [];
const child = require("child_process");
const add = str =>
  output.push(
    ...str
      .trim()
      .split("\n")
      .map(str => str.trim())
  );

add(`
  # ==============
  # DO NOT EDIT!!!
  # ==============
  #
  # This file is generated automatically by run/generate-dockerfile.js.
  # Edit that, not this.
`);

// build a minimal root package.json that doesn't change much for caching
const rootPkg = JSON.parse(
  fs.readFileSync(path.resolve(__dirname, "..", "package.json"), "utf8")
);

const buildPkg = {
  name: rootPkg.name,
  private: rootPkg.private,
  workspaces: rootPkg.workspaces
};

fs.writeFileSync(
  path.resolve(__dirname, "..", "package.build.json"),
  JSON.stringify(buildPkg, null, 2)
);

add(`
  FROM stream.place/sp-node AS base
  ADD package.build.json /app/package.json
`);

add("# add all package.json files");

const pkgs = {};

let packages = fs
  .readdirSync(path.resolve(__dirname, "..", "packages"))
  .filter(pkgName => {
    if (PACKAGE_BLACKLIST.includes(pkgName)) {
      // exclude eplicitly blacklisted packages
      return false;
    }
    try {
      pkgs[pkgName] = JSON.parse(
        fs.readFileSync(
          path.resolve(__dirname, "..", "packages", pkgName, "package.json")
        )
      );
    } catch (e) {
      // ENOTDIR happens b/c .DS_Store :(
      if (e.code !== "ENOENT" && e.code !== "ENOTDIR") {
        throw e;
      }
      return false;
    }
    return true;
  })
  .map(pkgName => {
    const pkg = pkgs[pkgName];
    pkg.containerName = pkg.name.replace(/-/g, "");
    return pkg;
  });

const rootDir = path.resolve(__dirname, "..");
// Sort all the packages by their last git modification time to try and speed things up
const times = {};
packages.forEach(pkg => {
  times[pkg.name] = child
    .execSync(`git log -5 --format="%at" -- packages/${pkg.name}`, {
      cwd: rootDir
    })
    .toString()
    .trim()
    .split("\n")
    .map(x => parseInt(x));
});

// But wait, also check if there's local modifications! They should deffo be first!
const modifiedFiles = child
  .execSync("git diff --name-only", {
    cwd: rootDir
  })
  .toString()
  .trim()
  .split("\n");

const untrackedFiles = child
  .execSync("git ls-files --others --exclude-standard", { cwd: rootDir })
  .toString()
  .trim()
  .split("\n");

const modifiedPackages = {};
modifiedFiles.concat(untrackedFiles).forEach(fileName => {
  if (fileName.startsWith("packages/")) {
    const pkgName = fileName.split("/")[1];
    modifiedPackages[pkgName] = true;
  }
});

const sortPackages = packages => {
  return packages.sort((a, b) => {
    for (let i = 0; i < times[a.name].length; i++) {
      const aTime = times[a.name][i];
      const bTime = times[b.name][i];
      if (aTime === bTime) {
        continue;
      }
      return aTime - bTime;
    }
  });
};

// first sort by git time for the builder build
packages = sortPackages(packages);

packages.forEach(pkg => {
  add(
    `ADD packages/${pkg.name}/package.json /app/packages/${
      pkg.name
    }/package.json`
  );
});

add("# build everyone's development dependencies into one big blob");
add(`
  FROM stream.place/sp-node AS builder
  ENV NODE_ENV development
  RUN apt-get update && apt-get install -y python build-essential # needed for building binaries on ARM
  COPY --from=base /app /app
  RUN yarn install
`);

add("# build each package separately");

let buildPackages = packages.filter(pkg => {
  if (!pkg.scripts || !pkg.scripts.prepare) {
    return false;
  }
  // exclude frontend apps, they're handled separately
  if (pkg.devDependencies && pkg.devDependencies["react-scripts"]) {
    return false;
  }
  return true;
});
buildPackages.forEach(pkg => {
  add(`
    FROM builder AS ${pkg.containerName}
    ADD packages/${pkg.name}/src /app/packages/${pkg.name}/src
    RUN cd /app/packages/${pkg.name} && npm run prepare
  `);
});

add("# build everyone's production dependencies into one big blob");
add("FROM base");
add(
  "RUN " +
    [
      // clear out dev dependencies, we don't want em
      "find . -maxdepth 3 -type f -name package.json -exec sed -i s/devDependencies/devDependenciesRemoved/ {} \\;",
      "yarn install --prod",
      "yarn cache clean"
    ].join(" && ")
);

packages.forEach(pkg => {
  if (modifiedPackages[pkg.name]) {
    // replace with actual modification time someday
    times[pkg.name] = [Number.MAX_SAFE_INTEGER, ...times[pkg.name]];
  } else {
    times[pkg.name] = [0, ...times[pkg.name]];
  }
});

buildPackages = sortPackages(buildPackages);

buildPackages.forEach(pkg => {
  const distDir = pkg.main.split("/")[0];
  add(
    `COPY --from=${pkg.containerName} /app/packages/${
      pkg.name
    }/${distDir} /app/packages/${pkg.name}/${distDir}`
  );
});

fs.writeFileSync(
  path.resolve(__dirname, "..", "Dockerfile"),
  output.join("\n") + "\n",
  "utf8"
);

module.exports.buildRootContainer = () => {
  child.execSync("docker build -t stream.place/streamplace .", {
    cwd: rootDir,
    stdio: "inherit"
  });
};

module.exports.triggerDeploy = deploymentName => {
  const patch = JSON.stringify({
    spec: {
      template: { metadata: { labels: { deployTime: `${Date.now()}` } } }
    }
  });
  child.execSync(`kubectl patch deployment ${deploymentName} -p '${patch}'`, {
    stdio: "inherit"
  });
};

module.exports.buildDev = () => {
  if (modifiedPackages["sp-node"]) {
    child.execSync("docker build -t stream.place/sp-node .", {
      cwd: process.resolve(rootDir, "packages", "sp-node"),
      stdio: "inherit"
    });
  }

  module.exports.buildRootContainer();

  const lernaString = `npx lerna exec ${Object.keys(modifiedPackages)
    .map(pkgName => `--scope ${pkgName}`)
    .join(
      " "
    )} 'if [ -f Dockerfile ]; then ../../run/package-log.sh docker build -t stream.place/$(basename $PWD) .; fi'`;
  if (Object.keys(modifiedPackages).length > 0) {
    child.execSync(lernaString, {
      cwd: rootDir,
      stdio: "inherit"
    });
  }

  const deployments = JSON.parse(
    child.execSync("kubectl get deployments -o json")
  );
  // .items[].spec.template.spec.containers[].image
  deployments.items.forEach(item => {
    for (const container of item.spec.template.spec.containers) {
      for (const modifiedPackage of Object.keys(modifiedPackages)) {
        if (container.image.startsWith(`stream.place/${modifiedPackage}`)) {
          module.exports.triggerDeploy(item.metadata.name);
          return;
        }
      }
    }
  });
};

if (process.argv[2] === "--dev") {
  module.exports.buildDev();
} else {
  module.exports.buildRootContainer();
}
