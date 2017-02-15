/**
 * Little helper script to get a nice bash color for all our apps in development
 */

/* eslint-disable no-console */

const crypto = require("crypto");

const BLACKLIST = [
  16, 17, 18, 232, 233, 234, 235, 236, 237, 238, 239
];

let input = process.argv[2];
let color;

while (!color || BLACKLIST.indexOf(color) !== -1) {
  input = crypto.createHash("sha1").update(input).digest("hex");
  color = parseInt(input.slice(8)) % 255;
}

console.log(color);
