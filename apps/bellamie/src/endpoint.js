
import express from "express";
import winston from "winston";
import Resource, {SKContext} from "sk-resource";
import config from "sk-config";

const RETHINK_HOST = config.require("RETHINK_HOST");
const RETHINK_PORT = config.require("RETHINK_PORT");
const RETHINK_DATABASE = config.require("RETHINK_DATABASE");

export default function endpoint({resource}) {
  const app = express();

  const handleError = function(req, res, next, err) {
    if (!err) {
      err = {};
    }
    if (typeof err.status !== "number") {
      err.status = 500;
    }
    res.status(err.status);
    if (err.status >= 500) {
      winston.error(err);
    }
    // TODO: scrub message/stack if not dev
    res.json({
      message: err.message || "Unexpected Error",
      status: err.status,
      code: err.code || "UNEXPECTED_ERROR",
      stack: err.stack,
    });
    next && next();
  };

  app.use((req, res, next) => {
    SKContext.createContext({
      rethinkHost: RETHINK_HOST,
      rethinkPort: RETHINK_PORT,
      rethinkDatabase: RETHINK_DATABASE
    })
    .then((ctx) => {
      req.ctx = ctx;
      next();
    })
    .catch(handleError.bind(this, req, res, null));
  });

  app.get("/", (req, res, next) => {
    Promise.resolve()
    .then(() => {
      let filter = {};
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
      return resource.find(req.ctx, filter);
    })
    .then((docs) => {
      res.status(200);
      res.json(docs);
      res.end();
    })
    .then(next)
    .catch(handleError.bind(this, req, res, next));
  });

  app.get("/:id", (req, res, next) => {
    resource.findOne(req.ctx, req.params.id)
    .then((doc) => {
      if (doc) {
        res.status(200);
        res.json(doc);
        res.end();
        return next();
      }
      else {
        throw new Resource.NotFoundError();
      }
    })
    .catch(handleError.bind(this, req, res, next));
  });

  app.post("/", (req, res, next) => {
    resource.create(req.ctx, req.body)
    .then((newDoc) => {
      res.status(201);
      res.json(newDoc);
      res.end();
      return next();
    })
    .catch(handleError.bind(this, req, res, next));
  });

  app.put("/:id", (req, res, next) => {
    resource.update(req.ctx, req.params.id, req.body)
    .then((newDoc) => {
      res.status(200);
      res.json(newDoc);
      res.end();
      return next();
    })
    .catch(handleError.bind(this, req, res, next));
  });

  app.delete("/:id", (req, res, next) => {
    resource.delete(req.ctx, req.params.id)
    .then(() => {
      res.status(204);
      res.end();
      return next();
    })
    .catch(handleError.bind(this, req, res, next));
  });

  app.use((req, res, next) => {
    req.ctx.conn.close();
    next();
  });

  return app;
}
