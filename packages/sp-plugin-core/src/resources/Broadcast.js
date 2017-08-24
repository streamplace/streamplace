import Resource from "sp-resource";

export default class Broadcast extends Resource {
  default(ctx) {
    return super.default().then(doc => {
      return {
        ...doc,
        userId: ctx.user.id,
        title: "New Broadcast",
        sources: [],
        active: false
      };
    });
  }

  auth(ctx, doc) {
    if (ctx.isPrivileged()) {
      return Promise.resolve();
    }
    if (doc.userId !== ctx.user.id) {
      throw new Resource.APIError(
        403,
        "You may only access your own broadcasts"
      );
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
