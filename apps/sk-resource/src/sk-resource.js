
import {createNamespace} from "continuation-local-storage";

export default class Resource {
  constructor() {

  }

  find(ctx, selector) {

  }

  findOne(ctx, id) {
    return this._dbFindOne(ctx, id).then((doc) => {
      return doc;
    });
  }

  create(ctx, doc) {
    return this._dbUpsert(ctx, doc).then((newDoc) => {
      return newDoc;
    });
  }

  update(ctx, id, doc) {
    doc.id = id;
    return this._dbUpsert(ctx, doc).then((newDoc) => {
      return newDoc;
    });
  }

  delete(ctx, id) {
    return this._dbDelete(ctx, id);
  }

  _dbFind(ctx, selector) {

  }

  _dbFindOne(ctx, id) {

  }

  _dbUpsert(ctx, doc) {

  }

  _dbDelete(ctx, id) {

  }
}

Resource._contextSession = createNamespace("context");
