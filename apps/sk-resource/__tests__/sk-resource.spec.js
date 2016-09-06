
import Resource from "../src/sk-resource";
import _ from "underscore";
import {v4} from "node-uuid";
import EventEmitter from "events";

let testResource;
let ctx;
let db;

const wait = function(ms) {
  return new Promise((resolve, reject) => {
    setTimeout(resolve, ms);
  });
};

class MockDbDriver {
  constructor() {
    this.watching = {};
  }

  find(ctx, selector) {
    return new Promise((resolve, reject) => {
      const docs = _(db).chain()
        .values()
        .filter(selector)
        .value();
      resolve(docs);
    });
  }

  findOne(ctx, id) {
    return new Promise((resolve, reject) => {
      resolve(db[id]);
    });
  }

  upsert(ctx, doc) {
    return new Promise((resolve, reject) => {
      let newDoc = false;
      if (!doc.id) {
        doc.id = v4();
        this._change(null, doc);
      }
      else {
        this._change(db[doc.id], doc);
      }
      db[doc.id] = doc;
      resolve(doc);
    });
  }

  delete(ctx, id) {
    return new Promise((resolve, reject) => {
      if (!db[id]) {
        throw new Error("I do not have that ID");
      }
      this._change(db[id], null);
      delete db[id];
      resolve();
    });
  }

  _change(oldVal, newVal) {
    process.nextTick(() => {
      _(this.watching).values().forEach(cb => cb({oldVal, newVal}));
    });
  }

  watch(ctx, query) {
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
        cb({oldVal, newVal});
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

beforeEach(() => {
  ctx = {
    subscriptions: [],
  };
  db = {};
  const TestResource = class extends Resource {};
  testResource = new TestResource({
    db: new MockDbDriver()
  });
});

it("should initalize", () => {
  expect(testResource instanceof Resource).toBe(true);
});


it("should findOne", () => {
  const testId = v4();
  db[testId] = {id: testId, "foo": "bar"};
  return testResource.findOne(ctx, testId)
  .then((doc) => {
    expect(doc).toEqual({id: testId, foo: "bar"});
  });
});

it("should find", () => {
  const pushDoc = () => {
    const doc = {};
    doc.id = v4();
    doc.foo = v4();
    db[doc.id] = doc;
    return doc;
  };
  pushDoc();
  pushDoc();
  const testDoc = pushDoc();
  testDoc.foo = "bar";
  return testResource.find(ctx, {foo: "bar"})
  .then((docs) => {
    expect(docs).toEqual([testDoc]);
    return testResource.find(ctx);
  })
  .then((docs) => {
    expect(docs.length).toBe(3);
  });
});

it("should create", () => {
  return testResource.create(ctx, {foo: "bar"})
  .then((doc) => {
    expect(db[doc.id].foo).toBe("bar");
  });
});

it("shouldn't allow creates with an id", () => {
  expect(() => {
    return testResource.create(ctx, {foo: "bar", id: "nope"});
  }).toThrowError(/VALIDATION_FAILED/);
});

it("should update", () => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  return testResource.update(ctx, testId, {foo: "baz"})
  .then(() => {
    expect(db[testId]).toEqual({id: testId, foo: "baz"});
  });
});

it("should update with a provided id", () => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  return testResource.update(ctx, testId, {id: testId, foo: "baz"})
  .then(() => {
    expect(db[testId]).toEqual({id: testId, foo: "baz"});
  });
});

it("shouldn't allow updates that change the id", () => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  expect(() => {
    return testResource.update(ctx, testId, {id: v4(), foo: "baz"});
  }).toThrowError(/VALIDATION_FAILED/);
});

it("should delete", () => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  return testResource.delete(ctx, testId)
  .then(() => {
    expect(db).toEqual({});
  });
});

it("should transform for all CRUD operations", () => {
  testResource.transform = function(ctx, doc) {
    doc.transform = true;
    return Promise.resolve(doc);
  };
  let id;
  return testResource.create(ctx, {foo: "bar"})
  .then((doc) => {
    id = doc.id;
    expect(doc.transform).toBe(true);
    return testResource.update(ctx, id, {foo: "baz"});
  })
  .then((doc) => {
    expect(doc.transform).toBe(true);
    return testResource.findOne(ctx, id);
  })
  .then((doc) => {
    expect(doc.transform).toBe(true);
    return testResource.find(ctx, {id});
  })
  .then(([doc]) => {
    expect(doc.transform).toBe(true);
  });
});

///////////////////////
// tests for watch() //
///////////////////////

let watchCalledCount;
let oldVal;
let newVal;
beforeEach(() => {
  watchCalledCount = 0;

  ctx.data = function(vals) {
    watchCalledCount += 1;
    oldVal = vals.oldVal;
    newVal = vals.newVal;
  };
});

it("should watch on CRUD operations", () => {
  return testResource.watch(ctx, {}, v4())
  .then(() => {
    return testResource.create(ctx, {foo: "bar"});
  })
  .then(() => {
    return wait(0);
  })
  .then(() => {
    expect(watchCalledCount).toBe(1);
    expect(oldVal).toBe(null);
    expect(newVal.foo).toBe("bar");
    return testResource.update(ctx, newVal.id, {foo: "baz"});
  })
  .then(() => {
    return wait(0);
  })
  .then(() => {
    expect(watchCalledCount).toBe(2);
    expect(oldVal.foo).toBe("bar");
    expect(newVal.foo).toBe("baz");
    return testResource.delete(ctx, oldVal.id);
  })
  .then(() => {
    return wait(0);
  })
  .then(() => {
    expect(watchCalledCount).toBe(3);
    expect(oldVal.foo).toBe("baz");
    expect(newVal).toBe(null);
  });
});

it("should stop watching", () => {
  const subId = v4();
  let handle;
  return testResource.watch(ctx, {}, subId)
  .then((newHandle) => {
    handle = newHandle;
    return wait(0);
  })
  .then(() => {
    const p = testResource.create(ctx, {foo: "bar"});
    handle.stop();
    return p;
  })
  .then(() => {
    return wait(0);
  })
  .then(() => {
    expect(watchCalledCount).toBe(0);
  });

});
