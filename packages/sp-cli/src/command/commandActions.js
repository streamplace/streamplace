
import {COMMAND_SYNC, COMMAND_SERVE} from "../constants/actionNames";
import {watcherWatch} from "../watcher/watcherActions";
import {socketConnect} from "../socket/socketActions";
import {serverListen} from "../server/serverActions";

export const commandSync = (config) => dispatch => {
  dispatch({
    type: COMMAND_SYNC,
    config: config,
  });
  dispatch(watcherWatch());
  dispatch(socketConnect(config.devServer));
};

export const commandServe = (config) => dispatch => {
  dispatch({
    type: COMMAND_SERVE,
    config: config,
  });
  dispatch(serverListen());
};

/**
 * HACK SECTION
 * This isn't reduxy yet but I really need building to work so here we are.
 */
/* eslint-disable no-console */

import {spawn as spawnChild} from "mz/child_process";
import {resolve} from "path";
import fs from "mz/fs";

const spawnLog = function(command, args, options) {
  console.log(`Running ${command} ${args.join(" ")}`);
  const proc = spawnChild(command, args, options);
  proc.stdout.on("data", (data) => {
    console.log(`stdout: ${data.toString().trim()}`);
  });
  proc.stderr.on("data", (data) => {
    // Hack for https://github.com/yarnpkg/yarn/issues/2538
    const str = data.toString().trim();
    console.log(`stderr: ${str}`);
    if (str.indexOf("Your lockfile needs to be updated")) {
      throw new Error("needs lockfile update");
    }
  });
  return new Promise((resolve, reject) => {
    proc.on("close", (code) => {
      if (code !== 0) {
        const err = new Error(`${command} ${args.join(" ")} exited with code ${code}`);
        err.exitCode = code;
        return reject(err);
      }
      resolve();
    });
  });
};

export const commandBuild = (config) => dispatch => {
  const {dockerPrefix, dockerTag, directory} = config;
  console.log(`Building ${directory}`);
  let needsDocker = true;
  let dockerName;
  const spawn = (file, ...args) => {
    return spawnLog(file, args, {
      cwd: directory,
    });
  };
  try {
    fs.statSync(resolve(directory, "Dockerfile"));
  }
  catch(e) {
    if (e.code !== "ENOENT") {
      throw e;
    }
    needsDocker = false;
  }
  let pkg;
  return Promise.resolve()

  .then(() => {
    return fs.readFile(resolve(directory, "package.json"), "utf8");
  })

  .then((pkgStr) => {
    pkg = JSON.parse(pkgStr);
    dockerName = `${dockerPrefix}/${pkg.name}:${dockerTag}`;
    return spawn("yarn", "install", "--frozen-lockfile");
  })

  .then(function() {
    if (needsDocker) {
      return spawn("docker", "build", "-t", dockerName, ".");
    }
  })

  .then(function() {
    if (needsDocker) {
      return spawn("docker", "push", dockerName);
    }
  })

  .then(function() {
    return fs.readdir(resolve(directory, "packages")).catch(function(err) {
      if (err.code !== "ENOENT") {
        throw err;
      }
      return [];
    });
  })

  .then(function(packages) {
    const doOne = function() {
      if (packages.length === 0) {
        return;
      }
      const newPkg = packages.pop();
      return dispatch(commandBuild({
        dockerPrefix: dockerPrefix,
        dockerTag: dockerTag,
        directory: resolve(directory, "packages", newPkg),
      }))
      .then(doOne);
    };
    return doOne();
  })

  .catch(function(err) {
    console.error(err);
    process.exit(err.exitCode || 1);
  });
};
