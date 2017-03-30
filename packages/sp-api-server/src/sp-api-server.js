
import winston from "winston";
import Ajv from "ajv";
import {SKContext, RethinkDbDriver} from "sp-resource";
import express from "express";
import config from "sp-configuration";
import bodyParser from "body-parser";
import http from "http";
import axios from "axios";
import httpHandler from "./http-handler";
import socketHandler from "./socket-handler";

const BELLAMIE_PORT = process.env.PORT || 80;
const SCHEMA_URL = config.require("SCHEMA_URL");
// Authorized upstream jwt issuer, e.g. auth.stream.place

winston.cli();
winston.level = process.env.DEBUG_LEVEL || "info";

const app = express();

app.get("/healthz", (req, res) => {
  res.sendStatus(200);
});

app.use(bodyParser.json());

app.use(function(req, res, next) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, SP-Auth-Token");
  res.header("Access-Control-Expose-Headers", "SP-Auth-Token");
  res.header("Access-Control-Allow-Methods", "OPTIONS, GET, HEAD, POST, PUT, PATCH, DELETE");
  if (req.method === "OPTIONS") {
    return res.end();
  }
  next();
});

const ajv = new Ajv({
  allErrors: true
});

const schemaNames = {};
const resourceNames = {};

axios.get(SCHEMA_URL).then((response) => {
  const schema = response.data;
  const plugins = new Set();
  Object.keys(schema.definitions).forEach((schemaName) => {
    if (schemaNames[schemaName]) {
      throw new Error(`Duplicate declaration of ${schemaName}`);
    }
    schemaNames[schemaName] = true;
    const schemaObject = schema.definitions[schemaName];
    plugins.add(schemaObject.plugin);
    winston.info(`Adding schema ${schemaName} (${schemaObject.plugin})`);
    ajv.addSchema(schemaObject, schemaName);
  });

  // Retrieve resources for each plugin
  ([...plugins]).forEach((pluginName) => {
    // Add resources
    const plugin = require(pluginName);
    winston.info(`Loading plugin ${pluginName}`);
    Object.keys(plugin.resources || {}).forEach((resourceName) => {
      if (!schemaNames[resourceName]) {
        throw new Error(`Found resource ${resourceName} but not its schema!`);
      }
      if (resourceNames[resourceName]) {
        throw new Error(`Duplicate declaration of resource ${resourceName}!`);
      }
      resourceNames[resourceName] = true;
      const Resource = plugin.resources[resourceName];
      Resource.tableName = schema.definitions[resourceName].tableName;
      if (!Resource.tableName) {
        throw new Error(`Resource ${resourceName} lacks a tableName`);
      }
      const path = `/${Resource.tableName}`;
      winston.info(`[${pluginName}] Adding resource ${resourceName} at ${path}`);
      const resource = new Resource({
        dbDriver: RethinkDbDriver,
        ajv: ajv,
      });
      SKContext.addResource(resource);
      const handler = httpHandler({resource});
      app.use(path, handler);
    });
  });

  const server = http.createServer(app);
  socketHandler(server);
  server.listen(BELLAMIE_PORT);
  winston.info(`Bellamie listening on port ${BELLAMIE_PORT}`);
})
.catch((err) => {
  winston.error(err);
  process.exit(1);
});
