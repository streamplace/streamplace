
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

const fatal = function(msg) {
  winston.error(msg);
  throw new Error(msg);
};

SwaggerParser.parse(schema)

.then(function(parsed) {
  const paths = Object.keys(parsed.paths);
  paths.forEach(function(path) {
    // Get the resource. It's the thing right after the slash.
    const actions = parsed.paths[path];
    const resourceName = path.split("/")[1];
    if (!resources[resourceName]) {
      winston.info(`Resource: ${resourceName}`);
      resources[resourceName] = require(`./resources/${resourceName}`).default;
    }
    const resource = resources[resourceName];

    // Paths can be either of the form /resource or /resource/{id}. For now.
    if (path === `/${resourceName}/{id}`) {
      const expressPath = `/${resourceName}/:id`;
      winston.info(`  Path ${expressPath}`);
      Object.keys(actions).forEach(function(verb) {
        if (verb === "parameters") {
          // We don't care! Return.
          return;
        }
        else if (verb === "get") {
          app.get(expressPath, resource.get.bind(resource));
        }
        else if (verb === "put") {
          app.put(expressPath, resource.put.bind(resource));
        }
        else if (verb === "delete") {
          app.delete(expressPath, resource.delete.bind(resource));
        }
        else {
          fatal(`Not sure how to handle verb "${verb} on this path`);
        }
      });
    }
    else if (path === `/${resourceName}`) {
      const expressPath = path;
      winston.info(`  Path ${expressPath}`);
      Object.keys(actions).forEach(function(verb) {
        if (verb === "parameters") {
          // We don't care! Return.
          return;
        }
        else if (verb === "get") {
          app.get(expressPath, resource.index.bind(resource));
        }
        else if (verb === "post") {
          app.post(expressPath, resource.post.bind(resource));
        }
        else {
          fatal(`Not sure how to handle verb "${verb} on this path`);
        }
      });
    }
    else {
      fatal(`Unrecognized path format in schema: ${path}`);
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
