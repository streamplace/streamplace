
import winston from "winston";
import Ajv from "ajv";
import {RethinkDbDriver} from "sk-resource";
import express from "express";
import morgan from "morgan";
import config from "sk-config";
import bodyParser from "body-parser";

import endpoint from "./endpoint";

const BELLAMIE_PORT = config.require("BELLAMIE_PORT");

winston.cli();
winston.level = process.env.DEBUG_LEVEL || "info";

const app = express();

app.use(morgan("dev"));
app.use(bodyParser.json());

app.use(function(req, res, next) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, SK-Auth-Token");
  res.header("Access-Control-Allow-Methods", "OPTIONS, GET, HEAD, POST, PUT, PATCH, DELETE");
  if (req.method === "OPTIONS") {
    return res.end();
  }
  next();
});

const ajv = new Ajv({
  allErrors: true
});

// Constant for now...
const SK_PLUGINS = ["sk-plugin-core"];

const schemaNames = {};
const resourceNames = {};
const resources = {};
SK_PLUGINS.forEach((pluginName) => {
  const plugin = require(pluginName);
  winston.info(`Loading ${pluginName}...`);

  // Add schemas
  Object.keys(plugin.schema || {}).forEach((schemaName) => {
    if (schemaNames[schemaName]) {
      throw new Error(`Duplicate declaration of ${schemaName}`);
    }
    schemaNames[schemaName] = true;
    const schema = plugin.schema[schemaName];
    winston.debug(`[${pluginName}] Adding schema ${schemaName}`);
    ajv.addSchema(schema, schemaName);
  });

  // Add resources
  Object.keys(plugin.resources || {}).forEach((resourceName) => {
    if (!schemaNames[resourceName]) {
      throw new Error(`Found resource ${resourceName} but not its schema!`);
    }
    if (resourceNames[resourceName]) {
      throw new Error(`Duplicate declaration of resource ${resourceName}!`);
    }
    resourceNames[resourceName] = true;
    const Resource = plugin.resources[resourceName];
    if (!Resource.tableName) {
      throw new Error(`Resource ${resourceName} lacks a tableName`);
    }
    const path = `/v0/${Resource.tableName}`;
    winston.debug(`[${pluginName}] Adding resource ${resourceName} at ${path}`);
    const resource = new Resource({
      dbDriver: RethinkDbDriver,
      ajv: ajv,
    });
    resources[resourceName] = resource;
    const resourceEndpoint = endpoint({resource});
    app.use(path, resourceEndpoint);
  });
});

app.listen(BELLAMIE_PORT);
