
// Eventually objects will have what Kubernets calls "strategic merge patch" here... we're not
// that complicated yet so we're just using some rando npm package that got some nice real estate
// on the word "merge"
import merge from "merge";
import winston from "winston";

export default class Resource {
  constructor({dbDriver, ajv}) {
    if (!dbDriver) {
      throw new Error("no database provided");
    }
    if (!ajv) {
      throw new Error("no ajv provided");
    }

    this.db = new dbDriver({tableName: this.constructor.tableName});
    this.ajv = ajv;
    this.validator = this.ajv.getSchema(this.constructor.resourceName);
    if (!this.validator) {
      throw new Error(`Schema ${this.constructor.schema} not found!`);
    }
    if (this.validator.schema.additionalProperties !== false) {
      throw new Error(`Schema ${this.constructor.schema} lacks additionalProperties: false`);
    }
  }

  validate(ctx, doc) {
    return new Promise((resolve, reject) => {
      const valid = this.validator(doc);
      if (valid) {
        resolve(doc);
      }
      else {
        const err = new Resource.ValidationError(this.validator.errors);
        setTimeout(function() {
          reject(err);
        }, 0);
      }
    });
  }

  find(ctx, query = {}) {
    return this.authQuery(query, ctx)
    .then((restrictedQuery) => {
      merge(query, restrictedQuery);
      return this.db.find(ctx, query);
    })
    .then((docs) => {
      return Promise.all(docs.map(this.transform.bind(this, ctx)));
    });
  }

  findOne(ctx, id) {
    let doc;
    return this.db.findOne(ctx, id)
    .then((foundDoc) => {
      doc = foundDoc;
      if (!doc) {
        throw new Resource.NotFoundError();
      }
      return this.auth(ctx, doc);
    })
    .then(() => {
      return this.authFindOne(ctx, doc);
    })
    .then(() => {
      return this.transform(ctx, doc);
    });
  }

  create(ctx, doc) {
    if (doc.id !== undefined) {
      throw new Resource.ValidationError({
        message: "Cannot create with an extant ID"
      });
    }
    return this.validate(ctx, doc)
    .then(() => {
      return this.auth(ctx, doc);
    })
    .then(() => {
      return this.authCreate(ctx, doc);
    })
    .then((result) => {
      return this.db.upsert(ctx, doc);
    })
    .then((newDoc) => {
      return this.transform(ctx, newDoc);
    });
  }

  update(ctx, id, doc) {
    if (doc.id !== undefined && doc.id !== id) {
      throw new Resource.ValidationError({
        message: "Cannot modify ID of a document"
      });
    }
    doc.id = id;
    let oldDoc;
    let newDoc;
    return this.db.findOne(ctx, id)
    .then((old) => {
      if (!old) {
        throw new Resource.NotFoundError();
      }
      oldDoc = old;
      newDoc = {};
      merge.recursive(newDoc, oldDoc);
      merge.recursive(newDoc, doc);
      return this.validate(ctx, newDoc);
    })
    .then(() => {
      return this.auth(ctx, newDoc);
    })
    .then(() => {
      return this.authUpdate(ctx, oldDoc, newDoc);
    })
    .then((result) => {
      return this.db.upsert(ctx, newDoc);
    })
    .then((newDoc) => {
      return this.transform(ctx, newDoc);
    });
  }

  delete(ctx, id) {
    let doc;
    return this.db.findOne(ctx, id)
    .then((docToDelete) => {
      doc = docToDelete;
      if (!doc) {
        throw new Resource.NotFoundError();
      }
      return this.auth(ctx, doc);
    })
    .then(() => {
      return this.authDelete(ctx, doc);
    })
    .then(() => {
      return this.db.delete(ctx, id);
    });
  }

  transform(ctx, doc) {
    return Promise.resolve(doc);
  }

  watch(ctx, query) {
    return this.authQuery(ctx, query)
    .then((restrictedQuery) => {
      merge(query, restrictedQuery);
      return this.db.watch(ctx, query);
    })
    .then((feed) => {
      let resolved = false;
      const handle = {
        stop: () => {
          return feed.close();
        }
      };
      return new Promise((resolve, reject) => {
        feed.on("data", function({old_val, new_val, type, state}) {
          if (state) {
            if (!resolved && state === "ready") {
              resolved = true;
              resolve(handle);
            }
          }
          else {
            ctx.data({oldVal: old_val, newVal: new_val});
          }
        });
      });
    });
  }

  ////////////////////
  // Auth Functions //
  ////////////////////

  // These are all full blacklists by default. The minimum necessary for implementing classes is
  // auth and authQuery.

  /**
   * Auth takes a document and returns whether or not we have access to it. If you need to
   * implement something more complicated than that, use the authAction functions.
   */
  auth(ctx, doc) {
    winston.warn(`${this.constructor.name} does not implement auth, disallowing.`);
    throw new Resource.ForbiddenError();
  }

  authQuery(ctx, query) {
    winston.warn(`${this.constructor.name} does not implement authQuery, returning nothing.`);
    // This is kinda silly but it does keep any documents from being returned.
    return Promise.resolve({FIELD_DOES_NOT_EXIST: true});
  }

  authFindOne(ctx, doc) {

  }

  authCreate(ctx, doc) {

  }

  authUpdate(ctx, oldDoc, newDoc) {

  }

  authDelete(ctx, doc) {

  }
}

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
  constructor(errors) {
    if (!(errors instanceof Array)) {
      errors = [errors];
    }
    const message = errors.map((err) => {
      let msg = err.message;
      if (err.params) {
        msg += ` (${JSON.stringify(err.params)})`;
      }
      return msg;
    }).join(", ");
    super({
      message: message,
      status: 422,
      code: "VALIDATION_FAILED"
    });
  }
};

Resource.NotFoundError = class NotFoundError extends Resource.APIError {
  constructor() {
    super({
      message: "Resource not found",
      status: 404,
      code: "NOT_FOUND"
    });
  }
};

Resource.ForbiddenError = class ForbiddenError extends Resource.APIError {
  constructor() {
    super({
      message: "You may not access this resource",
      status: 403,
      code: "UNAUTHORIZED"
    });
  }
};
