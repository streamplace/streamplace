
import {SocketCursor} from "./Cursor";

export default class Resource {
  constructor ({SK, swaggerResource}) {
    this.SK = SK;
    this.resource = swaggerResource;
    this.name = this.resource.label;
    // Alias some stuff on the resource to make our lives easier
    const capLabel = this.resource.label.charAt(0).toUpperCase() + this.resource.label.slice(1);
    ["find", "findOne", "create", "update", "delete"].forEach((action) => {
      this.resource[action] = this.resource[action + capLabel];
    });
  }

  onSuccess(response) {
    return response.obj;
  }

  onError(err) {
    // Regular server error, we can dig it.
    if (err.status && err.obj && err.obj.message) {
      throw {
        status: err.status,
        message: err.obj.message,
        code: err.obj.code || "UNKNOWN_ERROR"
      };
    }
    // Hmm. Unexpected error. Possibly connectivity problems?
    else {
      let message = "Unknown Error";
      if (err instanceof Error) {
        message = err.message;
      }
      else if (typeof err === "string") {
        message = err;
      }
      throw {
        status: -1,
        message: message,
        code: "UNKNOWN_ERROR"
      };
    }
  }

  find(query) {
    let params;
    if (query) {
      params = {filter: JSON.stringify(query)};
    }
    return this.resource.find(params)
      .then(this.onSuccess)
      .catch(this.onError);
  }

  findOne(id) {
    if (typeof id !== "string") {
      throw new Error(`findOne takes a string, ${typeof id} provided.`);
    }
    return this.resource.findOne({id})
      .then(this.onSuccess)
      .catch(this.onError);
  }

  create(doc) {
    return this.resource.create({body: doc})
      .then(this.onSuccess)
      .catch(this.onError);
  }

  update(id, fields) {
    return this.resource.update({id: id, body: fields})
      .then(this.onSuccess)
      .catch(this.onError);
  }

  delete(id) {
    return this.resource.delete({id})
      .then(this.onSuccess)
      .catch(this.onError);
  }

  watch(query = {}, fields) {
    Object.keys(query).forEach((key) => {
      const value = query[key];
      if (value === undefined) {
        throw new Error(`Tried to watch for ${this.name} with ${key}=undefined. Stream Kitchen objects will never have undefined fields. Either you meant to watch for "null", or you're passing a value you expect to be defined.`);
      }
    });
    const cursor = new SocketCursor({query, fields, resource: this, SK: this.SK});
    return cursor;
  }
}
