
import Resource from "sk-resource";

export default class Broadcast extends Resource {

  default() {
    return super.default().then((doc) => {
      return {
        ...doc,
        outputIds: [],
        enabled: false,
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

Broadcast.tableName = "broadcasts";
