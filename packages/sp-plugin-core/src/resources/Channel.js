
import Resource from "sp-resource";
import _ from "underscore";

export default class Channel extends Resource {

  default(ctx) {
    return super.default().then((doc) => {
      return {
        ...doc,
        userId: ctx.user.id,
        users: [],
      };
    });
  }

  validate(ctx, doc) {
    return super.validate(ctx, doc).then((doc) => {
      const userIds = doc.users.map(u => u.userId);
      if (userIds.length !== _.uniq(userIds).length) {
        throw new Resource.APIError(422, "Each user may only be in a channel once.");
      }
      return doc;
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
