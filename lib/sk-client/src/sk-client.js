
import url from "url";
import Swagger from "swagger-client";
import schema from "sk-schema";
import EE from "wolfy87-eventemitter";
import _ from "underscore";

let shouldLog = false;
let log = function(...args) {
  if (shouldLog) {
    /*eslint-disable no-console */
    console.error(...args);
  }
};

const METHOD_POLLING = Symbol("polling");
const METHOD_WEBSOCKET = Symbol("websocket");

class Cursor {
  constructor (resource, query) {
    this.POLL_INTERVAL = 1000;
    this.evt = new EE;
    this.resource = resource;
    this.query = query;

    this.knownDocs = {}; // Stored internally here as id --> doc

    this.promise = new Promise((resolve, reject) => {
      // TODO: right now we silently fail if there are errors
      this._startPolling();
      resolve();
    });
  }

  /**
   * Eventually this class will be awesome and use websockets. It is not yet awesome. It uses
   * polling. Also, this is designed to be a fallback in case the user can't use websockets for
   * whatever reason.
   */
  _startPolling() {
    this.method = METHOD_POLLING;
    this.intervalHandle = setInterval(this._poll.bind(this), this.POLL_INTERVAL);
  }

  _stopPolling() {
    clearInterval(this.intervalHandle);
  }

  _poll() {
    this.resource.find(this.query).then((docsArr) => {
      // If polling was turned off while our request was resolving, just stop.
      if (!this.intervalHandle) {
        return;
      }

      const newDocs = _(docsArr).indexBy("id");

      const knownIds = Object.keys(this.knownDocs);
      const newIds = Object.keys(newDocs);

      // If we see an id we don't have before, it's created.
      const createdIds = _(newIds).difference(knownIds);

      // If we don't see an id that we had seen before, it's removed.
      const deletedIds = _(knownIds).difference(newIds);

      // For all the other docs that weren't created or removed, check to see if they changed.
      const updatedIds = _(newIds).difference(createdIds, deletedIds).filter((id) => {
        return !_(this.knownDocs[id]).isEqual(newDocs[id]);
      });

      // Okay, update our local cache.
      _(createdIds).each((id) => {
        this.knownDocs[id] = newDocs[id];
      });

      _(updatedIds).each((id) => {
        this.knownDocs[id] = newDocs[id];
      });

      _(deletedIds).each((id) => {
        delete this.knownDocs[id];
      });

      const knownDocsArr = _(this.knownDocs).values();

      if (createdIds.length > 0) {
        this.evt.emit("created", knownDocsArr, createdIds);
      }

      if (updatedIds.length > 0) {
        this.evt.emit("updated", knownDocsArr, updatedIds);
      }

      if (deletedIds.length > 0) {
        this.evt.emit("deleted", knownDocsArr, deletedIds);
      }
    });
  }

  then (...args) {
    this.promise = this.promise.then(...args);
    return this;
  }

  catch (...args) {
    this.promise = this.promise.catch(...args);
    return this;
  }

  on (eventName, cb) {
    this.evt.on(eventName, cb);
    return this;
  }

  stop() {
    if (this.method === METHOD_POLLING) {
      this._stopPolling();
    }
    this.method = null;
  }
}

class Resource {
  constructor ({swaggerResource}) {
    this.resource = swaggerResource;
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
  find() {
    return this.resource.find()
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

const SKClient = function(params) {
  const {server} = params;
  if (params.log) {
    log = log;
  }
  // Override the schema with our provided endpoint
  const {protocol, host} = url.parse(server);
  schema.host = host;
  schema.schemes = [protocol.split(":")[0]];

  log(`SKClient initalizing for server ${schema.schemes[0]}://${schema.host}`);

  var client = new Swagger({
    spec: schema
  });

  client.buildFromSpec(schema);

  // Look at all the resources available in the freshly-parsed schema and build a Resource for
  // each one.
  client.apisArray.forEach((api) => {
    this[api.name] = new Resource({
      swaggerResource: client[api.name]
    });
  });

  client.usePromise = true;
};

SKClient.log = function() {
  shouldLog = true;
};

export default SKClient;
