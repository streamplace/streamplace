
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
        resolve(_(this._db).values().filter(selector));
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

it("should create", () => {
  return testResource.create(ctx, {foo: "bar"})
  .then((doc) => {
    expect(db[doc.id].foo).toBe("bar");
  });
});

// it("shouldn't let you create with an id", () => {
//   return testResource.create(ctx, {foo: "bar", id: "nope"})
//   .then(() => {
//     throw new Error("Should have failed!");
//   })
//   .catch((err) => {
//     expect(err.status).toBe(422);
//   });
// });

it("should update", () => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  return testResource.update(ctx, testId, {foo: "baz"})
  .then(() => {
    expect(db[testId]).toEqual({id: testId, foo: "baz"});
  });
});

it("should delete", () => {
  const testId = v4();
  db[testId] = {"foo": "bar"};
  return testResource.delete(ctx, testId)
  .then(() => {
    expect(db).toEqual({});
  });
});

it("should findOne", () => {
  const testId = v4();
  db[testId] = {id: testId, "foo": "bar"};
  return testResource.findOne(ctx, testId)
  .then((doc) => {
    expect(doc).toEqual({id: testId, foo: "bar"});
  });
});
