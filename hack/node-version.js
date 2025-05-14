const pkg = require("../package.json");
const engines = pkg.engines;

let version = engines.node;
if (!version.startsWith(">=")) {
  console.log(
    `fix package.json! idk what engines.node=${version} means, i'm just a little script :(`,
  );
  process.exit(1);
}
version = version.slice(2);
const neededMajor = parseInt(version.split(".")[0]);
const runningMajor = parseInt(process.version.slice(1).split(".")[0]);

if (runningMajor < neededMajor) {
  console.log(
    `Required node version ${version} not satisfied with current version ${process.version}.`,
  );
  process.exit(1);
}
