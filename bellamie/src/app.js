
import express from "express";
import winston from "winston";
import schema from "skitchen-schema";
import SwaggerParser from "swagger-parser";
import r from "rethinkdb";
import morgan from "morgan";
import _ from "underscore";

import ENV from "./env";

const app = express();

// Make winston output pretty
winston.cli();

app.use(morgan("combined"));

const rethinkConfig = {
  host: ENV.RETHINK_HOST,
  port: ENV.RETHINK_PORT,
  db: ENV.RETHINK_DATABASE
};

///////////////////////////////////////////////////////////////////////////////
// Step 1: parse the swagger schema and use that to build our routing table. //
///////////////////////////////////////////////////////////////////////////////

const resources = {};

const fatal = function(msg) {
  winston.error(msg);
  throw new Error(msg);
};

const ensureTableExists = function(name, conn) {
  return new Promise(function(resolve, reject) {
    r.tableCreate(name).run(conn).then(function() {
      winston.info(`Created table ${name}`);
      resolve();
    })
    .catch(function(err) {
      // Already exists, that's fine.
      if (err.msg.indexOf("already exists") !== -1) {
        resolve();
      }
      else {
        reject(err);
      }
    });
  });
};

const ensureDatabaseExists = function(name, conn) {
  return new Promise(function(resolve, reject) {
    r.dbCreate(name).run(conn).then(function() {
      winston.info(`Created database ${name}`);
      resolve();
    })
    .catch(function(err) {
      // Already exists, that's fine.
      if (err.msg.indexOf("already exists") !== -1) {
        resolve();
      }
      else {
        reject(err);
      }
    });
  });
};

app.use(function(req, res, next) {
  r.connect(rethinkConfig).then(function(conn) {
    winston.info("Connected to Rethink");
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
  if (!module.parent) {
    app.listen(ENV.PORT);
    winston.info("Bellamie starting up on port " + ENV.PORT);
  }
})

.catch(function(err) {
  fatal(err);
});


app.use(function(req, res, next) {
  req.conn.close().then(function() {
    winston.info("Rethink connection closed");
    next();
  })
  .error(function(err) {
    winston.error("Error closing Rethink connection", err);
    // The user doesn't care about this, though. next() it anyway.
    return next();
  });
});

