
import EE from "events";
import r from "rethinkdb";
import querystring from "querystring";
import url from "url";
import nJwt from "njwt";
import winston from "winston";
import APIError from "./api-error";

export default class SKContext extends EE {
  constructor() {
    super();
    this.resources = SKContext.resources;
  }

  data(tableName, oldVal, newVal) {
    if (!newVal) {
      this.emit("data", {
        tableName,
        id: oldVal.id,
        doc: null
      });
    }
    else {
      this.emit("data", {
        tableName,
        id: newVal.id,
        doc: newVal
      });
    }
  }

  cleanup() {
    if (this.conn) {
      this.conn.close();
    }
  }
}

SKContext.resources = {};
SKContext.addResource = function(resource) {
  if (SKContext.resources[resource.constructor.tableName]) {
    throw new Error(`Context got resource ${resource.constructor.name} twice!`);
  }
  SKContext.resources[resource.constructor.tableName] = resource;
};

SKContext.jwtSecret = null;
SKContext.jwtAudience = null;

SKContext.createContext = function({rethinkHost, rethinkPort, rethinkDatabase, token, remoteAddress}) {
  const ctx = new SKContext();
  ctx.remoteAddress = remoteAddress;
  return new Promise((resolve, reject) => {
    if (!token) {
      return reject(new APIError({
        status: 401,
        code: "MISSING_TOKEN",
        message: "Missing authentication token"
      }));
    }
    let verifiedJwt;
    try {
      verifiedJwt = nJwt.verify(token, SKContext.jwtSecret);
    }
    catch (e) {
      winston.error("Provided JWT failed verification", e);
      return reject(new APIError({
        status: 403,
        code: "ERR_TOKEN_INVALID",
        message: "Provided JWT is invalid"
      }));
    }
    if (verifiedJwt.body.aud !== SKContext.jwtAudience) {
      winston.error(`Got a JWT, and the signature looks fine, but the audience is ${verifiedJwt.aud} instead of ${SKContext.jwtAudience}. Weird, right?`);
      return reject(new APIError({
        status: 403,
        code: "ERR_TOKEN_INVALID",
        message: "Provided JWT is invalid"
      }));
    }
    ctx.jwt = verifiedJwt.body;
    resolve();
  })
  .then(() => {
    return r.connect({
      host: rethinkHost,
      port: rethinkPort,
      db: rethinkDatabase,
    });
  })
  .then((conn) => {
    ctx.rethink = r;
    ctx.conn = conn;
    return ctx.resources.users.findOrCreateFromContext(ctx);
  })
  .then((user) => {
    ctx.user = user;
    return ctx;
  });
};
