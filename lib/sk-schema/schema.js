
var fs = require("fs");
var path = require("path");
var yaml = require("js-yaml");

var yamlData = fs.readFileSync(path.resolve(__dirname, "api", "swagger", "swagger.yaml"));
var parsed = yaml.safeLoad(yamlData);
module.exports = parsed;
