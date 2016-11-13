
import winston from "winston";
import Ajv from "ajv";
import {SKContext, RethinkDbDriver} from "sk-resource";
import express from "express";
import config from "sk-config";
import bodyParser from "body-parser";
import http from "http";
import schema from "sk-schema";
import httpHandler from "./http-handler";
import socketHandler from "./socket-handler";

const BELLAMIE_PORT = config.require("BELLAMIE_PORT");
const JWT_SECRET = config.require("JWT_SECRET");
const JWT_SECRET_DECODED = Buffer.from(JWT_SECRET, "base64");
const PUBLIC_JWT_AUDIENCE = config.require("PUBLIC_JWT_AUDIENCE");
const PLUGINS = config.require("PLUGINS").split(/\s+/).filter(p => p !== "");

SKContext.jwtSecret = JWT_SECRET_DECODED;
SKContext.jwtAudience = PUBLIC_JWT_AUDIENCE;

winston.cli();
winston.level = process.env.DEBUG_LEVEL || "info";

const app = express();

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

const schemaNames = {};
const resourceNames = {};

Object.keys(schema.definitions).forEach((schemaName) => {
  if (schemaNames[schemaName]) {
    throw new Error(`Duplicate declaration of ${schemaName}`);
  }
  schemaNames[schemaName] = true;
  const schemaObject = schema.definitions[schemaName];
  winston.debug(`Adding schema ${schemaName}`);
  ajv.addSchema(schemaObject, schemaName);
});

PLUGINS.forEach((pluginName) => {
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
    if (!Resource.tableName) {
      throw new Error(`Resource ${resourceName} lacks a tableName`);
    }
    const path = `/v0/${Resource.tableName}`;
    winston.debug(`[${pluginName}] Adding resource ${resourceName} at ${path}`);
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
