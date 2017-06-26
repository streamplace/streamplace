#!/usr/bin/env node
/**
 * The sk-config CLI is designed to be run in our microservices that serve static HTML pages to
 * clients. It runs once on any *.mustache files in the /app/dist directory, populating their
 * templates with the proper environment variables.
 */

/*eslint-disable no-console */

import mustache from "mustache";
import fs from "fs";

import config from "./sk-config";

const fileName = process.argv[2];
if (!fileName) {
  console.error(`Usage: ${process.argv.join(" ")} [file.mustache]`);
}

let fileData;
try {
  fileData = fs.readFileSync(fileName, "utf8");
} catch (e) {
  console.error(e.stack);
  process.exit(1);
}

console.log(mustache.render(fileData, config));
