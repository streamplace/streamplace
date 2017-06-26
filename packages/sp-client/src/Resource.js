import _ from "underscore";
import { SocketCursor } from "./Cursor";

export default class Resource {
  constructor({ SK, swaggerResource }) {
    this.SK = SK;
    this.resource = swaggerResource;
    this.cursors = [];
    this.name = this.resource.label;
    this.onSuccess = ::this.onSuccess;
    this.onError = ::this.onError;
    // Alias some stuff on the resource to make our lives easier
    const capLabel =
      this.resource.label.charAt(0).toUpperCase() +
      this.resource.label.slice(1);
    ["find", "findOne", "create", "update", "delete"].forEach(action => {
      this.resource[action] = this.resource[action + capLabel];
    });
  }

  onSuccess(response) {
    // Set our new auth token if the server gave us one
    if (response.headers["sp-auth-token"]) {
      this.SK.newToken(response.headers["sp-auth-token"]);
    }
    return response.obj;
  }

  onError(err) {
    // Regular server error, we can dig it.
    if (err.obj && err.obj.errors) {
      throw new Error(err.obj.errors.map(e => e.message).join(", "));
    }
    if (err.status && err.obj && err.obj.message) {
      throw {
        status: err.status,
        message: err.obj.message,
        code: err.obj.code || "UNKNOWN_ERROR"
      };
    } else {
      // Hmm. Unexpected error. Possibly connectivity problems?
      let message = "Unknown Error";
      if (err instanceof Error) {
        message = err.message;
      } else if (typeof err === "string") {
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
      params = { filter: JSON.stringify(query) };
    }
    return this.resource.find(params).then(this.onSuccess).catch(this.onError);
  }

  findOne(id) {
    if (typeof id !== "string") {
      throw new Error(`findOne takes a string, ${typeof id} provided.`);
    }
    return this.resource
      .findOne({ id })
      .then(this.onSuccess)
      .catch(this.onError);
  }

  create(doc) {
    return this.resource
      .create({ body: doc })
      .then(this.onSuccess)
      .catch(this.onError);
  }

  update(id, fields) {
    return this.resource
      .update({ id: id, body: fields })
      .then(this.onSuccess)
      .catch(this.onError);
  }

  delete(id) {
    return this.resource
      .delete({ id })
      .then(this.onSuccess)
      .catch(this.onError);
  }

  watch(query = {}, fields) {
    if (typeof query !== "object" || query === null) {
      throw new Error(
        `Invalid query value for watch: ${query}. First argument to watch needs to be an object with {key: value}`
      );
    }
    Object.keys(query).forEach(key => {
      const value = query[key];
      if (value === undefined) {
        throw new Error(
          `Tried to watch for ${this
            .name} with ${key}=undefined. Stream Kitchen objects will never have undefined fields. Either you meant to watch for "null", or you're passing a value you expect to be defined.`
        );
      }
    });
    const cursor = new SocketCursor({
      query,
      fields,
      resource: this,
      SK: this.SK
    });
    this.cursors.push(cursor);
    cursor.on("stopped", () => {
      this.cursors.splice(this.cursors.indexOf(cursor), 1);
    });
    return cursor;
  }

  /**
   * Handle incoming data from a websocket
   */
  _data(id, doc) {
    this.cursors.forEach(c => c._data(id, doc));
  }
}
