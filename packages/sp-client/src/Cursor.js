
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
    this.ready = false;
    this.promise = new Promise((resolve, reject) => {
      this._resolve = resolve;
    });
    this.promise.then(() => {
      this.ready = true;
    });

    // Instantiate some crud
    this.evt = new EE;
    this.knownDocs = {}; // Stored internally here as id --> doc

    // Set up event aliases.
    //
    // "data" will run on our initial event and on every subsequent change to the data
    ["created", "updated", "deleted"].forEach((action) => {
      this.on(action, (...args) => {
        this._emit("data", ...args);
      });
    });
    this.promise.then((...args) => {
      this._emit("data", ...args);
    });

    // "newDoc" will run once for every document in the initial pull or subsequently created
    this.promise.then((docs) => {
      docs.forEach((doc) => {
        this._emit("newDoc", doc);
      });
    });
    this.on("created", (docs, newIds) => {
      docs.forEach((doc) => {
        if (newIds.indexOf(doc.id) !== -1) {
          this._emit("newDoc", doc);
        }
      });
    });

    // "deletedDoc" is when documents are deleted, you rube.
    this.on("deleted", (docs, deletedIds) => {
      deletedIds.forEach((id) => {
        this._emit("deletedDoc", id);
      });
    });
  }

  /**
   * Returns true if the provided doc matches our query. Don't be doin' queries with weird
   * prototypes, now, or I'll be very cross.
   */
  _matches(doc, query = this.query) {
    if (!doc) {
      return false;
    }
    for (let key in query) {
      if (typeof query[key] === "object" && query[key] !== null) {
        if (!this._matches(doc[key], query[key])) {
          return false;
        }
      }
      else if (doc[key] !== query[key]) {
        return false;
      }
    }
    return true;
  }

  _emit(type, ...args) {
    try {
      this.evt.emit(type, ...args);
    }
    catch (e) {
      this.SK.log(`Error emitting ${type} event`);
      this.SK.log(e.stack);
    }
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

  off(eventName, cb) {
    this.evt.off(eventName, cb);
    return this;
  }
}

export class SocketCursor extends Cursor {
  constructor(params) {
    super(params);

    this.handle = this.SK._subscribe(this.resource.name, this.query, ::this._suback);
  }

  _suback() {
    try {
      this._resolve(_(this.knownDocs).values());
    }
    catch (e) {
      this.SK.log("Error resolving cursor");
      this.SK.log(e.stack);
    }
  }

  _data(id, doc) {
    const matches = this._matches(doc);
    const exists = !!this.knownDocs[id];
    let type;
    if (!matches && !exists) {
      // We have nothing to do with this document. Return.
      return;
    }
    else if (matches && !exists) {
      // New document, that's cool.
      this.knownDocs[id] = doc;
      type = "created";
    }
    else if (!matches && exists) {
      // We know that ID, but now it doesn't match. Deleted.
      delete this.knownDocs[id];
      type = "deleted";
    }
    else if (matches && exists) {
      // Doc matches, and we've seen it before. Updated.
      this.knownDocs[id] = doc;
      type = "updated";
    }
    // Okay, now notify the user if we're ready.
    if (this.ready) {
      const knownDocsArr = _(this.knownDocs).values();
      this._emit(type, knownDocsArr, [id]);
    }
  }

  stop() {
    this.handle.stop();
    this._emit("stopped");
  }
}
