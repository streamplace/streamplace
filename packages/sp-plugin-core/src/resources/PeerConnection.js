
import Resource from "sp-resource";

export default class PeerConnection extends Resource {

  default(ctx) {
    return super.default().then((doc) => {
      return {
        ...doc,
        userId: ctx.user.id,
      };
    });
  }

  validate(ctx, doc) {
    return super.validate(ctx, doc).then(() => {
      if (doc.userId > doc.targetUserId) {
        throw new Resource.APIError({
          status: 422,
          code: "USERID_GREATER_THAN_TARGETUSERID",
          message: "All PeerConnection requests must be from the smaller user to the larger user"
        });
      }
    });
  }

  auth(ctx, doc) {
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }
}
