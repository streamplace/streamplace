class Config {
  require(key) {
    if (typeof this[key] === "undefined") {
      throw new Error(`Missing required configuration parameter: ${key}`);
    }
    return this[key];
  }

  optional(key) {
    return this[key];
  }

  add(obj) {
    Object.keys(obj).forEach(key => {
      if (typeof obj[key] === "function") {
        this[key] = obj[key].bind(this);
      }
      this[key] = obj[key];
    });
  }
}

let config = new Config();

if (typeof window === "object") {
  config.add(require("./sk-config-client"));
} else {
  // HACK so webpack doesn't webpack me. There's probably a better way.
  /*eslint-disable no-eval */
  config.add(eval("require('./sk-config-server')"));
}

module.exports = config;
