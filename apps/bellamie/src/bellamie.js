
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
import config from "sk-config";
import querystring from "querystring";
import url from "url";
import nJwt from "njwt";

import {ensureTableExists, ensureDatabaseExists} from "./util";

const RETHINK_HOST = config.require("RETHINK_HOST");
const RETHINK_PORT = config.require("RETHINK_PORT");
const RETHINK_DATABASE = config.require("RETHINK_DATABASE");
const BELLAMIE_PORT = config.require("BELLAMIE_PORT");
const PUBLIC_AUTH_MODE = config.require("PUBLIC_AUTH_MODE");
const PUBLIC_JWT_AUDIENCE = config.require("PUBLIC_JWT_AUDIENCE");

const useJwt = PUBLIC_AUTH_MODE === "jwt";
const JWT_SECRET = useJwt && config.require("JWT_SECRET");
const JWT_SECRET_DECODED = useJwt && Buffer.from(JWT_SECRET, "base64");

winston.level = process.env.DEBUG_LEVEL || "info";

const app = express();
const server = http.createServer(app);

const ERR_401_MISSING_HEADER = "401_MISSING_HEADER";
const ERR_403_BAD_TOKEN = "403_BAD_TOKEN";
const ERR_403_WRONG_AUDIENCE = "403_WRONG_AUDIENCE";

app.use(function(req, res, next) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, SK-Auth-Token");
  res.header("Access-Control-Allow-Methods", "OPTIONS, GET, HEAD, POST, PUT, PATCH, DELETE");
  if (req.method === "OPTIONS") {
    return res.end();
  }
  next();
});

// app.use(morgan("combined"));

/**
 * Parse the provided token, and return {code, message} if there's a problem.
 */
const jwtAuth = function(token) {
  if (PUBLIC_AUTH_MODE === "jwt") {
    if (!token) {
      return {
        code: 401,
        message: ERR_401_MISSING_HEADER
      };
    }
    let verifiedJwt;
    try {
      verifiedJwt = nJwt.verify(token, JWT_SECRET_DECODED);
    }
    catch (e) {
      winston.error("Provided JWT failed verification", e);
      return {
        code: 403,
        message: ERR_403_BAD_TOKEN
      };
    }
    if (verifiedJwt.body.aud !== PUBLIC_JWT_AUDIENCE) {
      winston.error(`Got a JWT, and the signature looks fine, but the audience is ${verifiedJwt.aud} instead of ${PUBLIC_JWT_AUDIENCE}. Weird, right?`);
      return {
        code: 403,
        message: ERR_403_WRONG_AUDIENCE
      };
    }
  }
};

app.use(function(req, res, next) {
  const err = jwtAuth(req.headers["sk-auth-token"]);
  if (err) {
    res.status(err.code);
    res.json(err);
    return res.end();
  }
  next();
});

// Make winston output pretty
winston.cli();

app.use(bodyParser.json());

const rethinkConfig = {
  host: RETHINK_HOST,
  port: RETHINK_PORT,
  db: RETHINK_DATABASE
};

const io = SocketIO(server);

io.use(function(socket, next){
  const {query} = url.parse(socket.request.url);
  const {token} = querystring.parse(query);
  const err = jwtAuth(token);
  if (err) {
    return next(new Error(JSON.stringify(err)));
  }
  next();
});

io.on("connection", function(socket) {
  socket.emit("hello");
  socket.setMaxListeners(100); // Otherwise it complains when we get to 11.

  const addr = socket.conn.remoteAddress;
  winston.info(`${addr} connect`);

  const dbPromise = r.connect(rethinkConfig).catch(function(err) {
    // TODO: error handling
    winston.error("Unable to connect to RethinkDB", err);
  });

  socket.on("sub", function({id, resource, query}) {
    winston.info(`${addr} sub`, {resource, query});
    dbPromise.then((conn) => {
      return resources[resource].watch({query, conn, socket, addr, subId: id});
    });
  });

  socket.on("disconnect", function() {
    winston.info(`${addr} disconnect`);
    dbPromise.then((conn) => {
      winston.debug("closing websocket database connection");
      conn.close();
    });
  });
});

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
        resources[resourceName] = new (require(`./resources/${resourceName}`).default);
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
  return ensureDatabaseExists(RETHINK_DATABASE, startupConn);
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
    server.listen(BELLAMIE_PORT);
    winston.info("Bellamie starting up on port " + BELLAMIE_PORT);
  }
})

.catch(function(err) {
  fatal(err);
});




