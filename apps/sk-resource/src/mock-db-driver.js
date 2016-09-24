
/**
 * Dummy, in-memory driver used instead of Rethink for testing.
 */

import _ from "underscore";
import {v4} from "node-uuid";

const req = function() {
  throw new Error("Missing required parameter");
};

export default class MockDbDriver {
  constructor({tableName}) {
    this.watching = {};
    this.data = {};
  }

  find(ctx = req(), query = req()) {
    return new Promise((resolve, reject) => {
      const docs = _(this.data).chain()
        .values()
        .filter(query)
        .value();
      resolve(docs);
    });
  }

  findOne(ctx = req(), id = req()) {
    return new Promise((resolve, reject) => {
      resolve(this.data[id]);
    });
  }

  upsert(ctx = req(), doc = req()) {
    return new Promise((resolve, reject) => {
      let newDoc = false;
      if (!doc.id) {
        doc.id = v4();
        this._change(null, doc);
      }
      else {
        this._change(this.data[doc.id], doc);
      }
      this.data[doc.id] = doc;
      resolve(doc);
    });
  }

  delete(ctx = req(), id = req()) {
    return new Promise((resolve, reject) => {
      if (!this.data[id]) {
        throw new Error("I do not have that ID");
      }
      this._change(this.data[id], null);
      delete this.data[id];
      resolve();
    });
  }

  _change(oldVal, newVal) {
    process.nextTick(() => {
      _(this.watching).values().forEach(cb => cb({oldVal, newVal}));
    });
  }

  watch(ctx = req(), query = req()) {
    const me = v4();
    const matchesQuery = function(doc) {
      if (!doc) {
        return false;
      }
      return _([doc]).where(query).length === 1;
    };
    const processor = (cb) => {
      return ({oldVal, newVal}) => {
        if (!matchesQuery(oldVal)) {
          oldVal = null;
        }
        if (!matchesQuery(newVal)) {
          newVal = null;
        }
        if (oldVal === null && newVal === null) {
          return;
        }
        cb({old_val: oldVal, new_val: newVal});
      };
    };
    return Promise.resolve({
      on: (keyword, cb) => {
        this.watching[me] = processor(cb);
      },
      close: () => {
        delete this.watching[me];
      },
    });
  }
}
