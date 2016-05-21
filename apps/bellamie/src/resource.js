
// "Base class" from which the individual resources "inherit". I mean. That's not how it actually
// works, but think about it that way.

import r from "rethinkdb";
import winston from "winston";
import _ from "underscore";

export default class Resource {

  // TODO: dumbest filtering implementation ever, it loads every single row then filters with
  // underscore
  index(req, res, next) {
    let filter;
    if (req.query.filter) {
      try {
        filter = JSON.parse(req.query.filter);
      }
      catch (e) {
        res.status(400);
        res.json({
          code: "MALFORMED_REQUEST",
          message: "The 'filter' parameter must be in JSON format."
        });
        res.end();
        return;
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
      this.serverError(req, res, next, err);
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
        res.status(404);
        res.json({
          code: "NOT_FOUND",
          message: "The specified resource was not found."
        });
      }
      next();
    })
    .catch((err) => {
      this.serverError(req, res, next, err);
    });
  }

  post(req, res, next) {
    const newDoc = req.body;
    this.beforeCreate(newDoc).then(() => {
      return r.table(this.name).insert(req.body).run(req.conn);
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
      this.serverError(req, res, next, err);
    });
  }

  put(req, res, next) {
    r.table(this.name).get(req.params.id).update(req.body).run(req.conn)
    .then((stuff) => {
      if (stuff.errors > 0) {
        throw new Error("Error in r.update()");
      }
      if (stuff.skipped > 0) {
        res.status(404);
        return {
          code: "NOT_FOUND",
          message: "The specified resource was not found."
        };
      }
      res.status(200);
      return r.table(this.name).get(req.params.id).run(req.conn);
    })
    .then((doc) => {
      res.json(doc);
      next();
    })
    .catch((err) => {
      this.serverError(req, res, next, err);
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
        res.status(500);
        winston.error(stuff.errors);
        res.json({
          code: "DATABASE_ERROR",
          message: "Error when attempting to delete resource"
        });
        res.end();
        return next();
      }
      if (stuff.skipped > 0) {
        res.status(404);
        res.json({
          code: "NOT_FOUND",
          message: "The specified resource was not found."
        });
        res.end();
        return next();
      }
      res.status(204);
      res.end();
      next();
    })
    .catch((err) => {
      this.serverError(req, res, next, err);
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

  serverError(req, res, next, err) {
    res.status(500);
    winston.error(err);
    res.json({
      code: "DATABASE_ERROR",
      message: JSON.stringify(err)
    });
    res.end();
  }

  /**
   * Make sure they're allowed to do the thing that they're doing.
   * @param  {[type]}   req  [description]
   * @param  {[type]}   res  [description]
   * @param  {Function} next [description]
   * @return {[type]}        [description]
   */
  auth (req, res, next) {
    next();
  }

  validate (req, res, next) {
    next();
  }

  beforeCreate(newDoc) {
    return new Promise((resolve, reject) => {
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
