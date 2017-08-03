import Resource from "sp-resource";

export default class Output extends Resource {
  default(ctx) {
    return super.default().then(doc => {
      return {
        ...doc,
        broadcastId: null,
        userId: ctx.user.id
      };
    });
  }

  auth(ctx, doc) {
    if (ctx.isPrivileged()) {
      return Promise.resolve();
    }
    if (doc.userId !== ctx.user.id) {
      throw new Resource.APIError(403, "You may only access your own outputs");
    }
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    if (ctx.isPrivileged()) {
      return Promise.resolve({});
    }
    return Promise.resolve({ userId: ctx.user.id });
  }
}

Output.tableName = "outputs";
