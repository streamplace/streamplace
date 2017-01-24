#!/usr/bin/env node
"use strict";Object.defineProperty(exports, "__esModule", { value: true });

var _commander = require("commander");var _commander2 = _interopRequireDefault(_commander);
var _fs = require("mz/fs");var _fs2 = _interopRequireDefault(_fs);
var _path = require("path");
var _package = require("../package.json");var _package2 = _interopRequireDefault(_package);
var _jsYaml = require("js-yaml");
var _mkdirpPromise = require("mkdirp-promise");var _mkdirpPromise2 = _interopRequireDefault(_mkdirpPromise);
var _sync = require("./sync");var _sync2 = _interopRequireDefault(_sync);function _interopRequireDefault(obj) {return obj && obj.__esModule ? obj : { default: obj };}

var configDefault = (0, _path.resolve)(process.env.HOME, ".streamplace", "sp-config.yaml");

var die = function die() {var _console;
  (_console = console).error.apply(_console, arguments);
  process.exit(1);
};

var CONFIG_DEFAULT = {
  authServer: "https://stream.place" };


// const {version} = JSON.parse(pkg);
exports.default = _commander2.default.
version(_package2.default.version).
option("--sp-config <file>", "location of sp-config.yaml (default $HOME/.streamplace/sp-config.yaml)", configDefault);

var getConfig = function getConfig() {var
  spConfig = _commander2.default.spConfig;
  return _fs2.default.stat(spConfig).
  catch(function (err) {
    if (err.code !== "ENOENT") {
      throw new Error(err);
    }
    return (0, _mkdirpPromise2.default)((0, _path.dirname)(spConfig)).then(function () {
      return _fs2.default.writeFile(spConfig, (0, _jsYaml.safeDump)(CONFIG_DEFAULT));
    });
  }).
  then(function () {
    return _fs2.default.readFile(spConfig, "utf8");
  }).
  then(function (data) {
    return (0, _jsYaml.safeLoad)(data);
  });
};

_commander2.default.
command("sync").
description("Sync your plugin to the development server").
action(function (command, env) {
  getConfig().
  then(function (config) {
    (0, _sync2.default)(config);
  }).
  catch(die);
});

_commander2.default.parse(process.argv);

if (!process.argv.slice(2).length) {
  _commander2.default.outputHelp();
}