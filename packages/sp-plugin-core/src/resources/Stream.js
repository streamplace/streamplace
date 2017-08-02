import Resource from "sp-resource";
import { randomId } from "sp-utils";

export default class Stream extends Resource {
  default(ctx) {
    return super.default().then(doc => {
      return {
        ...doc
      };
    });
  }

  auth(ctx, doc) {
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }
}
