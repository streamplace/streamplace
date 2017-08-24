import Resource from "sp-resource";
import { randomId } from "sp-utils";

export default class Input extends Resource {
  default(ctx) {
    return super.default().then(doc => {
      return {
        ...doc,
        userId: ctx.user.id,
        title: "New Input",
        streamTime: null
      };
    });
  }

  auth(ctx, doc) {
    if (ctx.isPrivileged()) {
      return Promise.resolve();
    }
    if (doc.userId !== ctx.user.id) {
      throw new Resource.APIError(403, "You may only access your own inputs");
    }
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    if (ctx.isPrivileged()) {
      return Promise.resolve({});
    }
    return Promise.resolve({ userId: ctx.user.id });
  }

  create(ctx, newDoc) {
    newDoc.streamKey = randomId();
    return super.create(ctx, newDoc);
  }
}
