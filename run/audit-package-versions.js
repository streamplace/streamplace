/**
 * Make sure that all versions of all required packages are what they oughta be.
 */

/* eslint-disable no-console */

const { resolve } = require("path");
const fs = require("fs");

function auditPackageVersions(dir) {
  if (!dir) {
    throw new Error("Missing directory!");
  }
  const problems = [];
  const correctVersion = "^" + require(resolve(dir, "lerna.json")).version;
  const packages = fs.readdirSync(resolve(dir, "packages"));
  packages.forEach(pkgName => {
    const pkgJson = resolve(dir, "packages", pkgName, "package.json");
    try {
      fs.statSync(pkgJson);
    } catch (e) {
      // No package.json, whatever, it's chill, let's move on
      return;
    }
    const pkg = require(pkgJson);
    ["dependencies", "peerDependencies", "devDependencies"].forEach(
      category => {
        if (!pkg[category]) {
          return;
        }
        const localDeps = Object.keys(pkg[category]).filter(depName => {
          return packages.indexOf(depName) !== -1;
        });
        localDeps.forEach(depName => {
          const depVersion = pkg[category][depName];
          if (depVersion !== correctVersion) {
            problems.push(
              `${pkgName} has ${depName} of ${depVersion} instead of ${
                correctVersion
              }`
            );
          }
        });
      }
    );
  });

  if (problems.length > 0) {
    problems.forEach(p => console.error(p));
    throw new Error("Package version audit failed");
  }
}

module.exports = auditPackageVersions;

if (!module.parent) {
  try {
    auditPackageVersions(process.argv[2]);
  } catch (e) {
    console.error("Correct package versions before proceeding.");
    process.exit(1);
  }
}
