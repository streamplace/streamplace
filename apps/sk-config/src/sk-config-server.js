
import yaml from "js-yaml";
import fs from "fs";

const config = {};

if (process.env.SK_CONFIG) {
  try {
    let doc = yaml.safeLoad(fs.readFileSync(process.env.SK_CONFIG));
    Object.keys(doc).forEach((key) => {
      config[key] = doc[key];
    });
  }
  catch (e) {
    /*eslint-disable no-console */
    console.error(`Couldn't parse YAML file at ${process.env.SK_CONFIG}: `, e.stack);
    console.error("Proceeding without YAML values.");
  }
}

const PREFIX = "SK_";

Object.keys(process.env).forEach((key) => {
  if (key.indexOf(PREFIX) !== 0) {
    return;
  }
  const value = process.env[key];
  key = key.slice(PREFIX.length);
  config[key] = value;
});

module.exports = config;
