
let config;

if (typeof window === "object") {
  config = require("./sk-config-client");
}
else {
  // HACK so webpack doesn't webpack me. There's probably a better way.
  /*eslint-disable no-eval */
  config = eval("require('./sk-config-server')");
}

config.require = function(key) {
  if (typeof config[key] === "undefined") {
    throw new Error(`Missing required configuration parameter: ${key}`);
  }
  return config[key];
};

config.optional = function(key) {
  return config[key];
};

module.exports = config;

