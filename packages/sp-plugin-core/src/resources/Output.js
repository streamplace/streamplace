import Resource from "sp-resource";

export default class Output extends Resource {
  default(ctx) {
    return super.default().then(doc => {
      return {
        ...doc,
        broadcastId: null
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

Output.tableName = "outputs";
