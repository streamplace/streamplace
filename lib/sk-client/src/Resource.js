
import Cursor from "./Cursor";

export default class Resource {
  constructor ({swaggerResource}) {
    this.resource = swaggerResource;
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

  /**
   * This will take a selector someday.
   * @return {[type]} [description]
   */
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

  watch(query = {}) {
    const cursor = new Cursor(this, query);
    return cursor;
  }
}
