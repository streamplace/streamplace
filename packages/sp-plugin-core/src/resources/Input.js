import Resource from "sp-resource";
import { randomId } from "sp-utils";

export default class Input extends Resource {
  auth(ctx, doc) {
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }

  create(ctx, newDoc) {
    newDoc.streamKey = randomId();
    newDoc.overlayKey = randomId();
    return super.create(ctx, newDoc);
  }
}

Input.tableName = "inputs";
