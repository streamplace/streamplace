// Written in Javascript instead of Bash because it builds on Windows too. Maybe some day the
// entire project should be this way.

/* eslint-disable no-console */

const fs = require("fs");
const { resolve } = require("path");
const { spawn } = require("child_process");
const { repoVersion, repoBranch } = require("../../run/repo-version");
const mkdirp = require("mkdirp");
const tmp = require("tmp-promise");
const os = require("os");
const axios = require("axios");

tmp.setGracefulCleanup();

// channel is latest if we're a tag, branch name otherwise
const channel = repoVersion.indexOf("-") === -1 ? "latest" : repoBranch;

let wd;
let buildDir;

// sanity check to see if we actually uploaded anything, electron-builder exits with 0 sometimes
const checkUploaded = () => {
  const pkg = require(resolve(wd, "package.json"));
  const checkFile = `${channel}-mac.json`;
  const hopefullyUploadedUrl = `https://s3-${
    pkg.build.publish.region
  }.amazonaws.com/${pkg.build.publish.bucket}/${
    pkg.build.publish.path
  }/${checkFile}`;
  return axios.get(hopefullyUploadedUrl).then(response => {
    const actual = JSON.stringify(response.data);
    const expectedStr = fs.readFileSync(resolve(wd, "dist", checkFile));
    const expected = JSON.stringify(JSON.parse(expectedStr));
    if (actual !== expected) {
      throw new Error(
        `upload seems to have failed, expected ${expected} got ${actual}`
      );
    }
  });
};

const run = function(command, ...args) {
  const proc = spawn(command, args, {
    cwd: wd,
    env: Object.assign({}, process.env, {
      ELECTRON_CHANNEL: channel
    })
  });

  proc.stdout.pipe(process.stdout);
  proc.stderr.pipe(process.stderr);

  return new Promise((resolve, reject) => {
    proc.on("close", code => {
      if (code === 0) {
        resolve();
      } else {
        reject();
      }
    });
  });
};

tmp
  .dir()
  .then(o => {
    buildDir = o.path;
    console.log(`Building app channel ${channel} in ${o.path}`);
    wd = o.path;
    return run("npm", "install", `sp-app@${repoBranch}`);
  })
  .then(() => {
    wd = resolve(wd, "node_modules", "sp-app");
    // Workaround for prepublish expecting a "src"
    mkdirp.sync(resolve(wd, "src"));
    // Now that we have it, do an actual dev-like npm install
    return run("npm", "install");
  })
  .then(() => {
    // code signing for mac only works on mac unfortunately
    if (os.platform() !== "darwin") {
      return run("npm", "run", "electron-publish-windows");
    } else {
      return run("npm", "run", "electron-publish-windows-mac");
    }
  })
  .then(checkUploaded)
  .catch(err => {
    console.error(err);
    process.exit(1);
  });
