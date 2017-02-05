#!/usr/bin/env node

import fs from "mz/fs";
import {resolve} from "path";
import {createStore, combineReducers, applyMiddleware} from "redux";
import thunk from "redux-thunk";
import program from "commander";
import pkg from "../package.json";
import terminalComponent from "./terminal/terminalComponent";
import rootReducer from "./rootReducer";
import getConfig from "./config";
import {commandSync, commandServe, commandBuild} from "./command/commandActions";
import * as actionNames from "./constants/actionNames";

const configDefault = resolve(process.env.HOME, ".streamplace", "sp-config.yaml");
// const {version} = JSON.parse(pkg);
export default program
  .version(pkg.version)
  .option("--sp-config <file>", "location of sp-config.yaml (default $HOME/.streamplace/sp-config.yaml)", configDefault);

const store = createStore(rootReducer, applyMiddleware(thunk));
terminalComponent(store);

program
  .command(actionNames.COMMAND_SYNC.toLowerCase())
  .description("sync your plugin to a development server")
  .option("--dev-server <url>", "url of your development server")
  .action(function(command) {
    store.dispatch(commandSync({
      devServer: command.devServer,
    }));
  });

program
  .command(actionNames.COMMAND_SERVE.toLowerCase())
  .description("[in-cluster only] run a development server")
  .option("--port <number>", "port to listen for incoming websocket connections", "80")
  .action(function(command) {
    store.dispatch(commandServe({
      port: command.port
    }));
  });

program
  .command(actionNames.COMMAND_BUILD.toLowerCase())
  .description("build a streamplace plugin")
  .option("--docker-prefix <name>", "prefix of built Docker images (default streamplace)", "streamplace")
  .option("--docker-tag <name>", "tag of built Docker images (default latest)", "latest")
  .option("--dev-server <url>", "url of your development server")
  .action(function(command) {
    store.dispatch(commandBuild({
      dockerPrefix: command.dockerPrefix,
      dockerTag: command.dockerTag,
      directory: process.cwd(),
    }));
  });

program.parse(process.argv);

if (!process.argv.slice(2).length) {
  program.outputHelp();
}
