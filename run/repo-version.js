
// Written in Javascript instead of Bash because it builds on Windows too. Maybe some day the
// entire project should be this way.

const {spawnSync} = require("child_process");

const describeProc = spawnSync("git", ["describe", "--tags"]);
const repoVersion = describeProc.stdout.toString().trim().slice(1);

let repoBranch;
if (process.env.TRAVIS_BRANCH) {
  repoBranch = process.env.TRAVIS_BRANCH;
}
else {
  const revProc = spawnSync("git", ["rev-parse", "--abbrev-ref", "HEAD"]);
  repoBranch = revProc.stdout.toString().trim();
}

// Strip the leading "v" for when it's not needed
module.exports = {repoVersion, repoBranch};

if (!module.parent) {
  if (process.argv[2] === "--branch") {
    process.stdout.write(module.exports.repoBranch);
  }
  else {
    process.stdout.write(module.exports.repoVersion);
  }
}
