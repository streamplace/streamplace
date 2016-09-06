
export default class Resource {
  constructor() {

  }

  find(ctx, selector = {}) {
    return this._dbFind(ctx, selector);
  }

  findOne(ctx, id) {
    return this._dbFindOne(ctx, id).then((doc) => {
      return doc;
    });
  }

  create(ctx, doc) {
    if (doc.id !== undefined) {
      throw new Resource.ValidationError({
        message: "Cannot create with an extant ID"
      });
    }
    return this._dbUpsert(ctx, doc).then((newDoc) => {
      return newDoc;
    });
  }

  update(ctx, id, doc) {
    if (doc.id !== undefined && doc.id !== id) {
      throw new Resource.ValidationError({
        message: "Cannot modify ID of a document"
      });
    }
    doc.id = id;
    return this._dbUpsert(ctx, doc).then((newDoc) => {
      return newDoc;
    });
  }

  delete(ctx, id) {
    return this._dbDelete(ctx, id);
  }

  _dbFind(ctx, selector) {

  }

  _dbFindOne(ctx, id) {

  }

  _dbUpsert(ctx, doc) {

  }

  _dbDelete(ctx, id) {

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
