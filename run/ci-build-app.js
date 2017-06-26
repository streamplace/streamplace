// Written in Javascript instead of Bash because it builds on Windows too. Maybe some day the
// entire project should be this way.

/* eslint-disable no-console */

const fs = require("fs");
const { resolve } = require("path");
const { spawn } = require("child_process");
const { repoVersion, repoBranch } = require("./repo-version");
const mkdirp = require("mkdirp");
const tmp = require("tmp-promise");

tmp.setGracefulCleanup();

// channel is latest if we're a tag, branch name otherwise
const channel = repoVersion.indexOf("-") === -1 ? "latest" : repoBranch;

let wd;
let buildDir;

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
    return run("npm", "install", `sp-app@${repoVersion}`);
  })
  .then(() => {
    wd = resolve(wd, "node_modules", "sp-app");
    // Workaround for prepublish expecting a "src"
    mkdirp.sync(resolve(wd, "src"));
    // Now that we have it, do an actual dev-like npm install
    return run("npm", "install");
  })
  .then(() => {
    return run("npm", "run", "electron-publish");
  })
  .catch(err => {
    console.error(err);
    process.exit(1);
  });
