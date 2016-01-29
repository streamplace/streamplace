
import express from "express";
import winston from "winston";
import schema from "skitchen-schema";
import SwaggerParser from "swagger-parser";
import ENV from "./env";

const app = express();

// Make winston output pretty
winston.cli();

///////////////////////////////////////////////////////////////////////////////
// Step 1: parse the swagger schema and use that to build our routing table. //
///////////////////////////////////////////////////////////////////////////////

const resources = {};

SwaggerParser.parse(schema)

.then(function(parsed) {
  const paths = Object.keys(parsed.paths);
  paths.forEach(function(path) {
    // Get the resource. It's the thing right after the slash.
    const resourceName = path.split("/")[1];
    if (!resources[resourceName]) {
      winston.info(`Registering resource: ${resourceName}`);
      resources[resourceName] = require(`./resources/${resourceName}`);
    }
  });
})

.catch(function(err) {
  winston.error("Error parsing schema", err);
  throw err;
});

if (!module.parent) {
  app.listen(ENV.PORT);
  winston.info("Bellamie starting up on port " + ENV.PORT);
}
