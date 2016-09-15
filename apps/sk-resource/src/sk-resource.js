
// Eventually objects will have what Kubernets calls "strategic merge patch" here... we're not
// that complicated yet so we're just using some rando npm package that got some nice real estate
// on the word "merge"
import merge from "merge";

export default class Resource {
  constructor({db, ajv}) {
    if (!db) {
      throw new Error("no database provided");
    }
    if (!ajv) {
      throw new Error("no ajv provided");
    }
    this._db = db;
    this.ajv = ajv;
    this.validator = this.ajv.getSchema(this.constructor.schema);
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

  find(ctx, selector = {}) {
    return this._db.find(ctx, selector).then((docs) => {
      return Promise.all(docs.map(this.transform.bind(this, ctx)));
    });
  }

  findOne(ctx, id) {
    return this._db.findOne(ctx, id).then((doc) => {
      return this.transform(ctx, doc);
    });
  }

  create(ctx, doc) {
    if (doc.id !== undefined) {
      throw new Resource.ValidationError({
        message: "Cannot create with an extant ID"
      });
    }
    return this.validate(ctx, doc).then(() => {
      return this._db.upsert(ctx, doc);
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
    return this._db.findOne(ctx, id)
    .then((oldDoc) => {
      if (!oldDoc) {
        throw new Resource.NotFoundError();
      }
      const newDoc = {};
      merge.recursive(newDoc, oldDoc);
      merge.recursive(newDoc, doc);
      return this.validate(ctx, newDoc);
    })
    .then((mergedDoc) => {
      return this._db.upsert(ctx, mergedDoc);
    })
    .then((newDoc) => {
      return this.transform(ctx, newDoc);
    });
  }

  delete(ctx, id) {
    return this._db.delete(ctx, id);
  }

  transform(ctx, doc) {
    return Promise.resolve(doc);
  }

  watch(ctx, query) {
    return this._db.watch(ctx, query).then((feed) => {
      feed.on("data", function({oldVal, newVal}) {
        ctx.data({oldVal, newVal});
      });
      return {
        stop: () => {
          feed.close();
        }
      };
    });
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
  constructor({message = "Validation Error"} = {}) {
    super({
      message: "Resource not found",
      status: 404,
      code: "NOT_FOUND"
    });
  }
};
