
import Resource from "sp-resource";

export default class Channel extends Resource {

  default(ctx) {
    return super.default().then((doc) => {
      return {
        ...doc,
        userId: ctx.user.id,
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

Channel.tableName = "channels";
