#!/usr/bin/env node

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const pkg = require("../package.json");

function getGitInfo() {
  try {
    const hash = execSync("git rev-parse HEAD", { encoding: "utf-8" }).trim();
    const shortHash = execSync("git rev-parse --short HEAD", {
      encoding: "utf-8",
    }).trim();
    const branch = execSync("git rev-parse --abbrev-ref HEAD", {
      encoding: "utf-8",
    }).trim();

    let tag;
    try {
      tag = execSync("git describe --tags --always --dirty", {
        encoding: "utf-8",
      }).trim();
    } catch (error) {
      // git describe fails in shallow clones, use package.json version + short hash
      tag = `v${pkg.version}-${shortHash}`;
    }

    const isDirty = tag.endsWith("-dirty");

    return {
      hash,
      shortHash,
      branch,
      tag,
      isDirty,
    };
  } catch (error) {
    console.warn("Could not get git info:", error.message);
    return {
      hash: "unknown",
      shortHash: "unknown",
      branch: "unknown",
      tag: "unknown",
      isDirty: false,
    };
  }
}

const buildInfo = {
  ...getGitInfo(),
  buildTime: new Date().toISOString(),
};

const outputPath = path.join(__dirname, "..", "src", "build-info.json");
fs.writeFileSync(outputPath, JSON.stringify(buildInfo, null, 2));

console.log("Generated build-info.json:", buildInfo);
