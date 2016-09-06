
import Resource from "../src/sk-resource";
import _ from "underscore";
import {v4} from "node-uuid";

let testResource;
let ctx;
let db;

beforeEach(() => {
  ctx = {};
  db = {};
  const TestResource = class extends Resource {
    constructor(params) {
      super(params);
      this._db = db;
    }

    _dbFind(ctx, selector) {
      return new Promise((resolve, reject) => {
        const docs = _(db).chain()
          .values()
          .filter(selector)
          .value();
        resolve(docs);
      });
    }

    _dbFindOne(ctx, id) {
      return new Promise((resolve, reject) => {
        resolve(this._db[id]);
      });
    }

    _dbUpsert(ctx, doc) {
      return new Promise((resolve, reject) => {
        if (!doc.id) {
          doc.id = v4();
        }
        this._db[doc.id] = doc;
        resolve(doc);
      });
    }

    _dbDelete(ctx, id) {
      return new Promise((resolve, reject) => {
        delete this._db[id];
        resolve();
      });
    }
  };
  testResource = new TestResource();
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

