
const fs = require("fs");
const path = require("path");
const yaml = require("js-yaml");

const yamlData = fs.readFileSync(path.resolve(__dirname, "api", "swagger", "swagger.yaml"));
const parsed = yaml.safeLoad(yamlData);
module.exports = parsed;
