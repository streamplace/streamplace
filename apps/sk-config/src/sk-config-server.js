
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

export default config;
