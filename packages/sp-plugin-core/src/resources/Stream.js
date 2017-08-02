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

  create(ctx, newDoc) {
    let newStream;
    return super
      .create(ctx, newDoc)
      .then(_newStream => {
        newStream = _newStream;
        return this.find(ctx, { source: { id: newStream.source.id } });
      })
      .then(otherStreams => {
        // Don't delete yourself, you dingus
        otherStreams = otherStreams.filter(
          stream => stream.id !== newStream.id
        );
        return Promise.all(
          otherStreams.map(stream => {
            return this.delete(ctx, stream.id);
          })
        );
      })
      .then(() => {
        return newStream;
      });
  }

  auth(ctx, doc) {
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }
}
