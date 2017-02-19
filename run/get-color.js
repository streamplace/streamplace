/**
 * Little helper script to get a nice bash color for all our apps in development
 */

/* eslint-disable no-console */

const crypto = require("crypto");

const BLACKLIST = [
  16, 17, 18, 232, 233, 234, 235, 236, 237, 238, 239
];

const memoized = {};

module.exports = function(input) {
  if (memoized[input]) {
    return memoized[input];
  }
  let color;

  while (!color || BLACKLIST.indexOf(color) !== -1) {
    input = crypto.createHash("sha1").update(input).digest("hex");
    color = parseInt(input.slice(8)) % 255;
  }
  memoized[input] = color;
  return color;
};

if (!module.parent) {
  console.log(module.exports(process.argv[2]));
}
