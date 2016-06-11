
// Nothing to see here. Just drop down into one of the implementations.

if (typeof window === "object") {
  module.exports = require("./sk-config-client");
}
else {
  // HACK so webpack doesn't webpack me. There's probably a better way.
  /*eslint-disable no-eval */
  module.exports = eval("require('./sk-config-server')");
}

module.exports.require = function(key) {
  if (typeof module.exports[key] === "undefined") {
    throw new Error(`Missing required configuration parameter: ${key}`);
  }
  return module.exports[key];
};
