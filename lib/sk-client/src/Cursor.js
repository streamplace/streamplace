
import EE from "wolfy87-eventemitter";
import _ from "underscore";

class Cursor {
  constructor(params) {
    // Handle arguments
    const {SK, resource, query} = params;
    this.SK = SK;
    this.resource = resource;
    this.query = query;

    // Instantiate some crud
    this.evt = new EE;
    this.knownDocs = {}; // Stored internally here as id --> doc
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
    this.promise = new Promise((resolve, reject) => {
      const handler = this._handleMessage.bind(this);
      this.socket = this.SK._subscribe(this.resource.name, this.query, handler);
    });
  }

  _handleMessage(message) {
    this.SK.log("Got message: " + message);
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
