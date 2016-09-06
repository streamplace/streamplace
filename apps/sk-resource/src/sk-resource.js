
export default class Resource {
  constructor({db}) {
    this._db = db;
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
    return this._db.upsert(ctx, doc).then((newDoc) => {
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
    return this._db.upsert(ctx, doc).then((newDoc) => {
      return this.transform(ctx, newDoc);
    });
  }

  delete(ctx, id) {
    return this._db.delete(ctx, id);
  }

  transform(ctx, doc) {
    return Promise.resolve(doc);
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
  constructor({message = "Validation Error"} = {}) {
    super({
      message: message,
      status: 422,
      code: "VALIDATION_FAILED"
    });
  }
};
