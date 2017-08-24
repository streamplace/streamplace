import Resource from "sp-resource";
import config from "sp-configuration";

const S3_HOST = config.require("S3_HOST");
const S3_BUCKET = config.require("S3_BUCKET");

export default class File extends Resource {
  default(ctx) {
    return super.default().then(doc => {
      return {
        ...doc,
        userId: ctx.user.id,
        host: S3_HOST,
        bucket: S3_BUCKET
      };
    });
  }

  auth(ctx, doc) {
    if (ctx.isPrivileged()) {
      return Promise.resolve();
    }
    if (doc.userId !== ctx.user.id) {
      throw new Resource.APIError(403, "You may only access your own files");
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
    return super.create(ctx, newDoc);
  }
}
