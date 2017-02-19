
// If we're in a production setting, rebuild our schema once right quick
if (typeof window === "undefined" && process.env.NODE_ENV === "production") {
  /*eslint-disable no-eval */
  eval("require('./build-schema.js')");
}

module.exports = require("./schema.json");
