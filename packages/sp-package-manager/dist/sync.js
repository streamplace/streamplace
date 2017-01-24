"use strict";Object.defineProperty(exports, "__esModule", { value: true });exports.default =
















function (config) {
  _chokidar2.default.watch('.', { ignored: ignored }).on('all', function (event, path) {
    console.log(event, path);
  });
};var _chokidar = require("chokidar");var _chokidar2 = _interopRequireDefault(_chokidar);var _fs = require("fs");var _fs2 = _interopRequireDefault(_fs);var _log = require("./log");function _interopRequireDefault(obj) {return obj && obj.__esModule ? obj : { default: obj };}var ignored = [];try {var gitignore = _fs2.default.readFileSync(".gitignore", "utf8");ignored = ignored.concat(gitignore.split("\n"));} catch (e) {if (e.code !== "ENOENT") {throw e;}(0, _log.warn)("You don't have a .gitignore file, that's weird.");}