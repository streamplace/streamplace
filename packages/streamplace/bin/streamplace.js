#!/usr/bin/env node
require("regenerator-runtime/runtime");
// early logic here... we want to support starting debug with the "--verbose" flag
if (process.argv.indexOf("--verbose") !== -1) {
  if (process.env.DEBUG) {
    process.env.DEBUG = process.env.DEBUG + ",sp:*";
  } else {
    process.env.DEBUG = "sp:*";
  }
}
/* eslint-disable no-var */
var cli = require("../dist/sp-cli").default;

if (!module.parent) {
  cli();
}
