
import yaml from "js-yaml";
import fs from "fs";
import path from "path";

const config = {};

if (process.env.SP_CONFIG) {
  try {
    let doc = yaml.safeLoad(fs.readFileSync(process.env.SP_CONFIG));
    Object.keys(doc).forEach((key) => {
      config[key] = doc[key];
    });
  }
  catch (e) {
    /*eslint-disable no-console */
    console.error(`Couldn't parse YAML file at ${process.env.SP_CONFIG}: `, e.stack);
    console.error("Proceeding without YAML values.");
  }
}

const PREFIX = "SP_";

Object.keys(process.env).forEach((key) => {
  if (key.indexOf(PREFIX) !== 0) {
    return;
  }
  const value = process.env[key];
  key = key.slice(PREFIX.length);
  config[key] = value;
});

// Automatically add my name, based on filename
if (process.argv[1]) {
  config.APP_NAME = path.basename(process.argv[1], ".js");
}
else {
  config.APP_NAME = "repl";
}

module.exports = config;
