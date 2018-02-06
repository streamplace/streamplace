#!/usr/bin/env node
require("regenerator-runtime/runtime");
/* eslint-disable no-var */
var cli = require("../dist/sp-cli").default;

if (!module.parent) {
  cli();
}
