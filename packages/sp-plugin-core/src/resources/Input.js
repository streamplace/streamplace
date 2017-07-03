import Resource from "sp-resource";
import { randomId } from "sp-utils";

export default class Input extends Resource {
  default(ctx) {
    return super.default().then(doc => {
      return {
        ...doc,
        userId: ctx.user.id
      };
    });
  }

  auth(ctx, doc) {
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }

  create(ctx, newDoc) {
    newDoc.streamKey = randomId();
    return super.create(ctx, newDoc);
  }
}

Input.tableName = "inputs";
