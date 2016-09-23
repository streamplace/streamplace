
import Resource from "../src/sk-resource";
import MockDbDriver from "../src/mock-db-driver";
import _ from "underscore";
import {v4} from "node-uuid";
import EventEmitter from "events";
import Ajv from "ajv";

let TestResource;
let testResource;
let ctx;
let db;
let ajv;

const testResourceSchema = {
  type: "object",
  additionalProperties: false,
  required: ["foo"],
  properties: {
    id: {
      type: "string"
    },
    userId: {
      type: "string"
    },
    foo: {
      type: "string"
    },
    transform: {
      type: "boolean"
    },
  },
};

const wait = function(ms) {
  return new Promise((resolve, reject) => {
    setTimeout(resolve, ms);
  });
};

const reversePromise = function(prom) {
  return new Promise((resolve, reject) => {
    prom.then(reject).catch(resolve);
  });
};

beforeEach(() => {
  ctx = {
    subscriptions: [],
  };
  db = new MockDbDriver();
  TestResource = class extends Resource {
    auth(ctx, doc) {
      return Promise.resolve();
    }

    authQuery(ctx, query) {
      return Promise.resolve({});
    }
  };
  ajv = new Ajv({
    allErrors: true
  });
  ajv.addSchema(testResourceSchema, "testResourceSchema");
  TestResource.schema = "testResourceSchema";
  testResource = new TestResource({db, ajv});
});

it("should initalize", () => {
  expect(testResource instanceof Resource).toBe(true);
});

it("should fail if no database is provided", () => {
  expect(() => {
    testResource = new TestResource({});
  }).toThrowError();
});

it("should findOne", () => {
  const testId = v4();
  return db.upsert(ctx, {id: testId, "foo": "bar"})
  .then(() => {
    return testResource.findOne(ctx, testId);
  })
  .then((doc) => {
    expect(doc).toEqual({id: testId, foo: "bar"});
  });
});

it("should find", () => {
  const randoDoc = () => {
    const doc = {};
    doc.id = v4();
    doc.foo = v4();
    return doc;
  };
  const testDoc = randoDoc();
  testDoc.foo = "bar";
  return Promise.all([
    db.upsert(ctx, randoDoc()),
    db.upsert(ctx, testDoc),
    db.upsert(ctx, randoDoc()),
  ])
  .then(() => {
    return testResource.find(ctx, {foo: "bar"});
  })
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
    return db.findOne(ctx, doc.id);
  })
  .then((doc) => {
    expect(doc.foo).toBe("bar");
  });
});

it("shouldn't auth creates with an id", () => {
  expect(() => {
    return testResource.create(ctx, {foo: "bar", id: "nope"});
  }).toThrowError(/VALIDATION_FAILED/);
});

it("should update", () => {
  const testId = v4();
  return db.upsert(ctx, {id: testId, "foo": "bar"})
  .then(() => {
    return testResource.update(ctx, testId, {foo: "baz"});
  })
  .then(() => {
    return db.findOne(ctx, testId);
  })
  .then((doc) => {
    expect(doc).toEqual({id: testId, foo: "baz"});
  });
});

it("should update with a provided id", () => {
  const testId = v4();
  return db.upsert(ctx, {id: testId, "foo": "bar"})
  .then(() => {
    return testResource.update(ctx, testId, {id: testId, foo: "baz"});
  })
  .then(() => {
    return db.findOne(ctx, testId);
  }).then((doc) => {
    expect(doc).toEqual({id: testId, foo: "baz"});
  });
});

it("shouldn't auth updates that change the id", () => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  expect(() => {
    return testResource.update(ctx, testId, {id: v4(), foo: "baz"});
  }).toThrowError(/VALIDATION_FAILED/);
});

it("should delete", () => {
  const testId = v4();
  return db.upsert(ctx, {id: testId, foo: "bar"})
  .then(() => {
    return testResource.delete(ctx, testId);
  })
  .then(() => {
    return db.find(ctx, {});
  })
  .then((docs) => {
    expect(docs).toEqual([]);
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

////////////////
// Validation //
////////////////

it("should fail if the schema is unknown", () => {
  TestResource.schema = "does-not-exist";
  expect(() => {
    testResource = new TestResource({
      db: new MockDbDriver(),
      ajv: ajv
    });
  }).toThrowError();
});

const shouldFail = function() {
  throw new Error("This should have failed.");
};

it("should disauth schema without additionalProperties: false", () => {
  const badSchema = JSON.parse(JSON.stringify(testResourceSchema));
  delete badSchema.additionalProperties;
  ajv.addSchema(badSchema, "bad-schema");
  TestResource.schema = "bad-schema";
  expect(() => {
    testResource = new TestResource({
      db: new MockDbDriver(),
      ajv: ajv
    });
  }).toThrowError();
});

it("should reject extra properties upon creation", (done) => {
  testResource.create(ctx, {foo: "bar", extra: "property"}).then(shouldFail)
  .catch((err) => {
    expect(err.message).toMatch(/VALIDATION_FAILED/);
    done();
  });
});

it("should reject extra properties upon update", (done) => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  testResource.update(ctx, testId, {extra: "property"}).then(shouldFail)
  .catch((err) => {
    done();
  });
});

it("should reject incorrect values upon creation", (done) => {
  testResource.create(ctx, {foo: false}).then(shouldFail)
  .catch((err) => {
    expect(err.message).toMatch(/VALIDATION_FAILED/);
    done();
  });
});

it("should reject incorrect values upon update", () => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  return db.upsert(ctx, {foo: "bar", id: testId})
  .then(() => {
    return reversePromise(testResource.update(ctx, testId, {foo: 123456}));
  })
  .then((err) => {
    expect(err.message).toMatch(/VALIDATION_FAILED/);
  });
});

////////////////
// Auth tests //
////////////////

const setUser = () => {
  ctx.userType = "user";
  ctx.user = {
    id: "test-id"
  };
};

const setService = () => {
  ctx.userType = "service";
  ctx.user = null;
};

it("should make auth checks on modification", () => {
  let createCalled = false;
  let updateCalled = false;
  let deleteCalled = false;
  let findOneCalled = false;
  TestResource.prototype.authCreate = function() {
    createCalled = true;
    return Promise.resolve(true);
  };
  TestResource.prototype.authUpdate = function() {
    updateCalled = true;
    return Promise.resolve(true);
  };
  TestResource.prototype.authDelete = function() {
    deleteCalled = true;
    return Promise.resolve(true);
  };
  TestResource.prototype.authFindOne = function() {
    findOneCalled = true;
    return Promise.resolve(true);
  };
  return testResource.create(ctx, {foo: "bar"})
  .then(({id}) => {
    expect(createCalled).toBe(true);
    return testResource.update(ctx, id, {foo: "baz"});
  })
  .then(({id}) => {
    expect(updateCalled).toBe(true);
    return testResource.findOne(ctx, id);
  })
  .then(({id}) => {
    return testResource.delete(ctx, id);
  })
  .then(() => {
    expect(deleteCalled).toBe(true);
  });
});

it("should disallow everything by default", () => {
  delete TestResource.prototype.auth;
  const testId = v4();
  return db.upsert(ctx, {id: testId, "foo": "bar"})
  .then(() => {
    return reversePromise(testResource.create(ctx, {foo: "bar"}));
  })
  .then((err) => {
    expect(err.status).toBe(403);
    return reversePromise(testResource.update(ctx, testId, {"foo": "baz"}));
  })
  .then((err) => {
    expect(err.status).toBe(403);
    return reversePromise(testResource.delete(ctx, testId));
  })
  .then((err) => {
    expect(err.status).toBe(403);
    return reversePromise(testResource.findOne(ctx, testId));
  })
  .then((err) => {
    expect(err.status).toBe(403);
  })
  .catch(() => {
    throw new Error("something succeeded! boo!");
  });
});

describe("queries", () => {
  let doc1;
  let doc2;
  let docAuthorized;
  let docUnauthorized;
  let selectorCalledCount;
  let watchCalledCount;
  beforeEach(() => {
    selectorCalledCount = 0;
    watchCalledCount = 0;
    TestResource.prototype.authQuery = () => {
      selectorCalledCount += 1;
      return Promise.resolve({userId: "yup"});
    };
    doc1 = {id: v4(), "foo": "bar"};
    doc2 = {id: v4(), "foo": "baz"};
    docAuthorized = {id: v4(), foo: "bar", userId: "yup"};
    docUnauthorized = {id: v4(), foo: "bar", userId: "nope"};
    return Promise.all([doc1, doc2, docAuthorized, docUnauthorized].map(db.upsert.bind(db, ctx)));
  });

  it("should authorize query on find", () => {
    return testResource.find(ctx, {foo: "bar"})
    .then((docs) => {
      expect(docs).toEqual([docAuthorized]);
      expect(selectorCalledCount).toBe(1);
    });
  });

  it("should authorize query on watch", () => {
    ctx.data = function({oldVal, newVal}) {
      expect(newVal.foo).toBe("bar");
      watchCalledCount += 1;
    };
    TestResource.prototype.authSelector = function() {
      selectorCalledCount += 1;
      return Promise.resolve({userId: "nope"});
    };
    let handle;
    return testResource.watch(ctx, {foo: "bar"})
    .then((h) => {
      handle = h;
      return testResource.update(ctx, doc1.id, {userId: "whatever"});
    })
    .then(() => {
      return wait();
    })
    .then(() => {
      handle.stop();
      expect(selectorCalledCount).toBe(1);
    });
  });
});
