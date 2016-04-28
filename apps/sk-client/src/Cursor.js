
import EE from "wolfy87-eventemitter";
import _ from "underscore";

class Cursor {
  constructor(params) {
    // Handle arguments
    const {SK, resource, query, fields} = params;
    this.SK = SK;
    this.resource = resource;
    this.query = query;
    this.fields = fields;

    // Set up our promise, save the handlers
    this.promise = new Promise((resolve, reject) => {
      this._resolve = resolve;
      this._reject = reject;
    });

    // Instantiate some crud
    this.evt = new EE;
    this.knownDocs = {}; // Stored internally here as id --> doc

    // Set up event aliases. "changed" will run on our initial event and on every subsequent
    // change to the data
    ["created", "updated", "deleted"].forEach((action) => {
      this.on(action, (...args) => {
        this.evt.emit("data", ...args);
      });
    });
    this.promise.then((...args) => {
      this.evt.emit("data", ...args);
    });
  }

  then(...args) {
    this.promise = this.promise.then(...args);
    return this;
  }

  catch(...args) {
    this.promise = this.promise.catch(...args);
    return this;
  }

  on(eventName, cb) {
    this.evt.on(eventName, cb);
    return this;
  }
}

export class SocketCursor extends Cursor {
  constructor(params) {
    super(params);

    const handler = this._handleMessage.bind(this);
    this.handle = this.SK._subscribe(this.resource.name, this.query, handler);
  }

  _handleMessage(type, evt) {
    let ids;
    if (type === "suback") {
      const {docs} = evt;
      this.knownDocs = _(docs).indexBy("id");
      this._resolve(docs);
    }
    else if (type === "created") {
      const {doc} = evt;
      this.knownDocs[doc.id] = doc;
      ids = [doc.id];
    }
    else if (type === "updated") {
      const {doc} = evt;
      this.knownDocs[doc.id] = doc;
      ids = [doc.id];
    }
    else if (type === "deleted") {
      const {id} = evt;
      if (this.knownDocs[id]) {
        delete this.knownDocs[id];
      }
      ids = [id];
    }
    else {
      throw new Error("Got unknown message: " + type);
    }
    const knownDocsArr = _(this.knownDocs).values();
    this.then(() => {
      try {
        this.evt.emit(type, knownDocsArr, ids);
      }
      catch(e) {
        this.log("Error emitting event:" + e.stack);
      }
    });
  }

  stop() {
    this.handle.stop();
  }
}

export class PollCursor extends Cursor {
  constructor(params) {
    super(params);

    this.POLL_INTERVAL = 1000;

    this.promise = new Promise((resolve, reject) => {
      this._startPolling();
      this.resource.find(this.query).then(resolve).catch(reject);
    });
  }

  /**
   * Eventually this class will be awesome and use websockets. It is not yet awesome. It uses
   * polling. Also, this is designed to be a fallback in case the user can't use websockets for
   * whatever reason.
   */
  _startPolling() {
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


  stop() {
    this._stopPolling();
  }
}
