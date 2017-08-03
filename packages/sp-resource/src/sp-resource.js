// Eventually objects will have what Kubernets calls "strategic merge patch" here... we're not
// that complicated yet so we're just using some rando npm package that got some nice real estate
// on the word "merge"
import merge from "merge";
import winston from "winston";

import APIError from "./api-error";
import RethinkDbDriver from "./rethink-db-driver";
import SKContext from "./sk-context";
export { RethinkDbDriver, SKContext, APIError };

// Magic key that will never exist for queries we can't access
const DISALLOWED_QUERY_SELECTOR = {
  QUERY_NOT_ALLOWED: true
};

export default class Resource {
  constructor({ dbDriver, ajv }) {
    if (!dbDriver) {
      throw new Error("no database provided");
    }
    if (!ajv) {
      throw new Error("no ajv provided");
    }
    this.db = new dbDriver({
      tableName: this.constructor.tableName,
      primaryKey: this.constructor.primaryKey || "id",
      indices: this.constructor.indices || []
    });
    this.ajv = ajv;
    this.validator = this.ajv.getSchema(this.constructor.name);
    if (!this.validator) {
      throw new Error(`Schema ${this.constructor.schema} not found!`);
    }
    if (this.validator.schema.additionalProperties !== false) {
      throw new Error(
        `Schema ${this.constructor.schema} lacks additionalProperties: false`
      );
    }
  }

  default(ctx) {
    return Promise.resolve({ kind: this.constructor.name });
  }

  validate(ctx, doc) {
    return new Promise((resolve, reject) => {
      const valid = this.validator(doc);
      if (valid) {
        resolve(doc);
      } else {
        const err = new Resource.ValidationError(this.validator.errors);
        setTimeout(function() {
          reject(err);
        }, 0);
      }
    });
  }

  find(ctx, query = {}) {
    return this.authQuery(ctx, query)
      .then(restrictedQuery => {
        merge(query, restrictedQuery);
        return this.db.find(ctx, query);
      })
      .then(docs => {
        return Promise.all(docs.map(this.transform.bind(this, ctx)));
      });
  }

  findOne(ctx, id) {
    let doc;
    return this.db
      .findOne(ctx, id)
      .then(foundDoc => {
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

  create(ctx, newDoc) {
    let doc;
    if (newDoc.id !== undefined) {
      throw new Resource.ValidationError({
        message: "Cannot create with an extant ID"
      });
    }
    return this.default(ctx)
      .then(defaultDoc => {
        doc = defaultDoc;
        merge(doc, newDoc);
        return this.validate(ctx, doc);
      })
      .then(() => {
        return this.auth(ctx, doc);
      })
      .then(() => {
        return this.authCreate(ctx, doc);
      })
      .then(result => {
        return this.db.insert(ctx, doc);
      })
      .then(newDoc => {
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
    return this.db
      .findOne(ctx, id)
      .then(old => {
        if (!old) {
          throw new Resource.NotFoundError();
        }
        oldDoc = old;
        newDoc = {};
        return this.default(ctx);
      })
      .then(defaultDoc => {
        merge.recursive(newDoc, defaultDoc);
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
      .then(result => {
        return this.db.replace(ctx, id, newDoc);
      })
      .then(newDoc => {
        return this.transform(ctx, newDoc);
      });
  }

  delete(ctx, id) {
    let doc;
    return this.db
      .findOne(ctx, id)
      .then(docToDelete => {
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

  /**
   * Transform the document on the way out to the user. One of the things we do here in all cases
   * is ensure that the outgoing document has a value for every field present in the "defaults"
   * block. This makes a common data migration case -- adding a new value to an object -- really
   * really easy.
   */
  transform(ctx, doc) {
    // return this.default(ctx).then((defaultDoc) => {
    //   Object.keys(defaultDoc).forEach((key) => {
    //     if (doc[key] === undefined) {
    //       doc[key] = defaultDoc[key];
    //     }
    //   });
    //   return doc;
    // });
    return Promise.resolve(doc);
  }

  watch(ctx, query) {
    return this.authQuery(ctx, query)
      .catch(err => {
        // If authQuery produced an error we just allow the sub but it's empty
        return DISALLOWED_QUERY_SELECTOR;
      })
      .then(restrictedQuery => {
        merge(query, restrictedQuery);
        return this.db.watch(ctx, query);
      })
      .then(feed => {
        let resolved = false;
        const handle = {
          close: () => {
            return feed.close();
          }
        };
        return new Promise((resolve, reject) => {
          feed.on("data", ({ old_val, new_val, type, state }) => {
            if (state) {
              if (!resolved && state === "ready") {
                resolved = true;
                resolve(handle);
              }
            } else {
              this.transform(ctx, new_val).then(transformedVal => {
                ctx.data(this.constructor.tableName, old_val, transformedVal);
              });
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
    winston.warn(
      `${this.constructor.name} does not implement auth, disallowing.`
    );
    throw new Resource.ForbiddenError();
  }

  authQuery(ctx, query) {
    winston.warn(
      `${this.constructor
        .name} does not implement authQuery, returning nothing.`
    );
    // This is kinda silly but it does keep any documents from being returned.
    return Promise.resolve({ FIELD_DOES_NOT_EXIST: true });
  }

  authFindOne(ctx, doc) {}

  authCreate(ctx, doc) {}

  authUpdate(ctx, oldDoc, newDoc) {}

  authDelete(ctx, doc) {}
}

Resource.APIError = APIError;

Resource.ValidationError = class ValidationError extends Resource.APIError {
  constructor(errors) {
    if (!(errors instanceof Array)) {
      errors = [errors];
    }
    const message = errors
      .map(err => {
        let msg = err.message;
        if (err.dataPath) {
          msg = `'${err.dataPath}' ${msg}`;
        }
        if (err.params) {
          msg += ` (${JSON.stringify(err.params)})`;
        }
        return msg;
      })
      .join(", ");
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
