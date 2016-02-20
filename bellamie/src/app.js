
import express from "express";
import winston from "winston";
import schema from "sk-schema";
import SwaggerParser from "swagger-parser";
import r from "rethinkdb";
import morgan from "morgan";
import _ from "underscore";
import http from "http";
import bodyParser from "body-parser";
import SocketIO from "socket.io";

import {ensureTableExists, ensureDatabaseExists} from "./util";
import ENV from "./env";

winston.level = process.env.DEBUG_LEVEL || "info";

const app = express();
const server = http.createServer(app);

// Make winston output pretty
winston.cli();

app.use(morgan("combined"));

app.use(bodyParser.json());

const rethinkConfig = {
  host: ENV.RETHINK_HOST,
  port: ENV.RETHINK_PORT,
  db: ENV.RETHINK_DATABASE
};

const io = SocketIO(server);

///////////////////////////////////////////////////////////////////////////////
// Step 1: parse the swagger schema and use that to build our routing table. //
///////////////////////////////////////////////////////////////////////////////

const resources = {};

const fatal = function(msg) {
  winston.error(msg);
  throw new Error(msg);
};

app.use(function(req, res, next) {
  r.connect(rethinkConfig).then(function(conn) {
    winston.debug("Connected to Rethink");
    req.conn = conn;
    next();
  })
  .catch(function(err) {
    winston.error("Unable to connect to RethinkDB", err);
    res.status(500);
    res.end();
  });
});

let startupConn;

SwaggerParser.parse(schema)

.then(function(parsed) {
  const paths = Object.keys(parsed.paths);
  paths.forEach(function(path) {
    // Get the resource. It's the thing right after the slash.
    const actions = parsed.paths[path];
    const resourceName = path.split("/")[1];
    if (!resources[resourceName]) {
      winston.debug(`Resource: ${resourceName}`);
      try {
        resources[resourceName] = require(`./resources/${resourceName}`).default;
      }
      catch (err) {
        winston.error("Caught error attempting to require resource. This probably means you " +
          "added something in the schema, but didn't implement it in the ./resources directory.");
        throw err;
      }
    }
    const resource = resources[resourceName];

    // Paths can be either of the form /resource or /resource/{id}. For now.
    if (path === `/${resourceName}/{id}`) {
      const expressPath = `/v0/${resourceName}/:id`;
      winston.debug(`  Path ${expressPath}`);
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
      const expressPath = `/v0${path}`;
      winston.debug(`  Path ${expressPath}`);
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

/////////////////////////////////////////////////////////////
// Create all necessary tables if they don't already exist //
/////////////////////////////////////////////////////////////

.then(function() {
  return r.connect(rethinkConfig);
})

.then(function(conn) {
  startupConn = conn;
  return ensureDatabaseExists(ENV.RETHINK_DATABASE, startupConn);
})

.then(function() {
  const resourceNames = Object.keys(resources);
  return Promise.all(resourceNames.map(function(resourceName) {
    return ensureTableExists(resourceName, startupConn);
  }));
})

////////////////////////////
// Cool, start listening. //
////////////////////////////
.then(function() {
  app.use(function(req, res, next) {
    req.conn.close().then(function() {
      winston.debug("Rethink connection closed");
      next();
    })
    .error(function(err) {
      winston.error("Error closing Rethink connection", err);
      // The user doesn't care about this, though. next() it anyway.
      return next();
    });
  });
  if (!module.parent) {
    server.listen(ENV.PORT);
    winston.info("Bellamie starting up on port " + ENV.PORT);
  }
})

.catch(function(err) {
  fatal(err);
});




