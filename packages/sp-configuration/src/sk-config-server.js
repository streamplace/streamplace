import yaml from "js-yaml";
import fs from "fs";
import path from "path";
import dot from "dot-object";

const config = {};

const PREFIX = "SP_";

Object.keys(process.env).forEach(key => {
  if (key.indexOf(PREFIX) !== 0) {
    return;
  }
  const value = process.env[key];
  key = key.slice(PREFIX.length);
  config[key] = value;
});

config.loadValuesFile = function(valuesPath) {
  const doc = yaml.safeLoad(fs.readFileSync(valuesPath, "utf8"));
  const flat = dot.dot(doc);
  Object.keys(flat).forEach(key => {
    const capitalKey = key
      .replace(/([A-Z])/g, "_$1")
      .toUpperCase()
      .replace(/\./g, "_")
      .replace(/^GLOBAL_/, "");
    this[capitalKey] = flat[key];
  });
};

// Special case: load values from a Helm values file if it exists
if (process.env.SP_VALUES_FILE) {
  config.loadValuesFile(process.env.SP_VALUES_FILE);
}

// Automatically add my name, based on filename
if (process.argv[1]) {
  config.APP_NAME = path.basename(process.argv[1], ".js");
} else {
  config.APP_NAME = "repl";
}

module.exports = config;
