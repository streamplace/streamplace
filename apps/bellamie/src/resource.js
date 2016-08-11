
// "Base class" from which the individual resources "inherit". I mean. That's not how it actually
// works, but think about it that way.

import r from "rethinkdb";
import winston from "winston";
import _ from "underscore";
import schema from "sk-schema";
import Ajv from "ajv";

const ajv = new Ajv({
  allErrors: true
});

ajv.addSchema(schema, "swagger.json");

export default class Resource {

  constructor(name) {
    this.schema = schema.definitions[this.constructor.name];
    this.name = name;
  }

  /**
   * Use AJV to see if provided document is valid.
   * @param  {Object} doc
   * @return {Array|null} Returns an array of errors on failure.
   */
  _validator(doc) {
    const valid = ajv.validate({$ref: `swagger.json#/definitions/${this.constructor.name}`}, doc);
    if (valid) {
      return null;
    }
    else {
      return ajv.errors;
    }
  }

  // TODO: dumbest filtering implementation ever, it loads every single row then filters with
  // underscore
  index(req, res, next) {
    let filter;
    if (req.query.filter) {
      try {
        filter = JSON.parse(req.query.filter);
      }
      catch (e) {
        throw new Resource.APIError({
          code: "MALFORMED_REQUEST",
          status: 400,
          message: "The 'filter' parameter must be in JSON format."
        });
      }
    }
    r.table(this.name).run(req.conn)
    .then((cursor) => {
      return cursor.toArray();
    })
    .then((docs) => {
      if (filter) {
        docs = _(docs).where(filter);
      }
      res.status(200);
      res.json(docs);
      next();
    })
    .catch((err) => {
      this._handleError(req, res, next, err);
    });
  }

  get(req, res, next) {
    r.table(this.name).get(req.params.id).run(req.conn)
    .then((doc) => {
      if (doc) {
        res.status(200);
        res.json(doc);
      }
      else {
        throw new Resource.NotFoundError();
      }
      next();
    })
    .catch((err) => {
      this._handleError(req, res, next, err);
    });
  }

  post(req, res, next) {
    let newDoc = req.body;
    return this.beforeCreate(newDoc)
    .then((doc) => {
      newDoc = doc;
      return this.validate(newDoc);
    })
    .catch(({errs, doc}) => {
      throw new Resource.ValidationError(errs, doc);
    })
    .then(() => {
      return r.table(this.name).insert(newDoc).run(req.conn);
    })
    .then(({generated_keys}) => {
      // TODO: error handling
      return r.table(this.name).get(generated_keys[0]).run(req.conn);
    })
    .then((doc) => {
      res.status(201);
      res.json(doc);
      next();
    })
    .catch((err) => {
      this._handleError(req, res, next, err);
    });
  }

  put(req, res, next) {
    r.table(this.name).get(req.params.id).run(req.conn)
    .then((doc) => {
      if (!doc) {
        throw new Resource.NotFoundError();
      }
      if (!req.body) {
        throw new Resource.APIError({
          status: 411,
          code: "BODY_MISSING",
          message: "Missing request body"
        });
      }
      _.extend(doc, req.body);
      return this.validate(doc);
    })
    .catch(({errs, doc}) => {
      throw new Resource.ValidationError(errs, doc);
    })
    .then(() => {
      return r.table(this.name).get(req.params.id).update(req.body).run(req.conn);
    })
    .then((stuff) => {
      if (stuff.errors > 0) {
        throw new Resource.APIError({
          message: "Unexpected error updating resource."
        });
      }
      if (stuff.skipped > 0) {
        // Should be rare, but not impossible someone deleted it after validation.
        throw new Resource.NotFoundError();
      }
      res.status(200);
      return r.table(this.name).get(req.params.id).run(req.conn);
    })
    .then((doc) => {
      res.json(doc);
      next();
    })
    .catch((err) => {
      this._handleError(req, res, next, err);
    });
  }

  delete(req, res, next) {
    const id = req.params.id;
    const conn = req.conn;
    return this.beforeDelete(id, conn)
    .then(() => {
      return r.table(this.name).get(id).delete(req.body).run(conn);
    })
    .then((stuff) => {
      if (stuff.errors > 0) {
        throw new Resource.APIError({
          message: "Unexpected error deleting resource."
        });
      }
      if (stuff.skipped > 0) {
        throw new Resource.NotFoundError();
      }
      res.status(204);
      res.end();
      next();
    })
    .catch((err) => {
      this._handleError(req, res, next, err);
    });
  }

  /**
   * Given a query, a rethink connection, a socket with a client, and an IP address, notify the
   * client of changes.
   * @param  {[type]} options.query  [description]
   * @param  {[type]} options.conn   [description]
   * @param  {[type]} options.socket [description]
   * @param  {[type]} options.addr   [description]
   * @return {[type]}                [description]
   */
  watch({query, conn, socket, addr, subId}) {
    const logStr = JSON.stringify({addr, subId, resource: this.name, query});
    return r.table(this.name).filter(query).run(conn)
    .then((cursor) => {
      return cursor.toArray();
    })
    .then((docs) => {
      winston.info("suback", {addr, subId});
      socket.emit("suback", {subId, docs});
      return r.table(this.name).filter(query).changes().run(conn);
    })
    .then((feed) => {
      feed.on("data", function(change) {
        const newVal = change.new_val;
        const oldVal = change.old_val;
        if (oldVal === null) {
          winston.debug("created", {addr, subId});
          socket.emit("created", {subId, doc: newVal});
        }
        else if (newVal === null) {
          winston.debug("deleted", {addr, subId});
          socket.emit("deleted", {subId, id: oldVal.id});
        }
        else {
          winston.debug("updated", {addr, subId});
          socket.emit("updated", {subId, doc: newVal});
        }
      });
      feed.on("error", function(...args) {
        winston.error("error on resource watch", ...args);
      });
      socket.on("unsub", function(params) {
        if (params.subId === subId) {
          winston.debug("unsub", {subId, addr});
          feed.removeAllListeners("data");
          feed.removeAllListeners("error");
          // console.log("Closing cursor bc unsub");
          feed.close();
          // conn.close();
        }
      });
      socket.on("disconnect", function() {
        feed.removeAllListeners("data");
        feed.removeAllListeners("error");
        // console.log("Closing cursor bc disconnect");
        feed.close();
      });
    })
    .catch((err) => {
      winston.error("Error in watch function", err);
    });
  }

  _handleError(req, res, next, err) {
    let status;
    if (typeof err.status === "number") {
      status = err.status;
    }
    else {
      status = 500;
    }
    res.status(status);
    if (status >= 500) {
      winston.error(err);
    }
    else {
      winston.warn(err._descriptiveMessage || err.message);
    }
    res.json({
      errors: [{
        code: err.code,
        message: err._shortMessage || err.message
      }]
    });
    res.end();
  }

  validate(newDoc) {
    return new Promise((resolve, reject) => {
      const errs = this._validator(newDoc);
      if (errs === null) {
        resolve();
      }
      else {
        reject({errs, doc: newDoc});
      }
    });
  }

  beforeCreate(newDoc) {
    return new Promise((resolve, reject) => {
      newDoc.kind = this.constructor.name.toLowerCase();
      resolve(newDoc);
    });
  }

  beforeDelete(id) {
    return new Promise((resolve, reject) => {
      resolve(id);
    });
  }

  save (req, res, next) {
    next();
  }

  postSave (req, res, next) {
    next();
  }
}

Resource.ajv = ajv;

Resource.APIError = class APIError extends Error {
  constructor({message, code, status}) {
    if (!message || !code || !status) {
      throw new Error("Missing required parameters");
    }
    if (typeof message !== "string") {
      throw new Error("Invalid format for message");
    }
    if (typeof status !== "number" || status < 400 || status > 599) {
      throw new Error("Invalid HTTP status");
    }
    const shortMessage = message;
    const descriptiveMessage = `[${status}] ${code} - ${message}`;
    super(`[${status}] ${code} - ${message}`);
    this._shortMessage = shortMessage;
    this._descriptiveMessage = descriptiveMessage;
    this.code = code;
    this.status = status;
  }
};

Resource.ValidationError = class ValidationError extends Resource.APIError {
  constructor(errs, doc) {
    const message = errs.map((e) => {
      if (e.keyword === "additionalProperties") {
        let prop = e.params.additionalProperty;
        if (e.dataPath && e.dataPath.length > 0) {
          prop = `${e.dataPath}.${prop}`;
        }
        return `Unexpected property: ${prop}`;
      }
      return JSON.stringify(e);
    }).join(", ");
    super({
      message: message,
      status: 422,
      code: "VALIDATION_FAILED"
    });
    this.errs = errs;
  }
};

Resource.NotFoundError = class NotFoundError extends Resource.APIError {
  constructor() {
    super({
      message: "Resource not found.",
      status: 404,
      code: "NOT_FOUND"
    });
  }
};
