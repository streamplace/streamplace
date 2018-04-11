import Resource from "sp-resource";
import { randomId } from "sp-client";

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

  authStreamSource(ctx, doc) {
    const sourceResource = Object.keys(ctx)
      .map(key => ctx[key])
      .find(resource => {
        if (!resource.resource) {
          return;
        }
      });
  }

  // For now, end users don't interact with these at all
  auth(ctx, doc) {
    if (ctx.isPrivileged()) {
      return Promise.resolve();
    }
    return this.authStreamSource(ctx, doc);
  }

  authUpdate(ctx, doc, newDoc) {
    if (!ctx.isPrivileged()) {
      throw new Resource.APIError(
        403,
        "Streams are read-only, alter the source instead"
      );
    }
    return Promise.resolve();
  }

  authDelete(ctx, doc, newDoc) {
    if (!ctx.isPrivileged()) {
      throw new Resource.APIError(
        403,
        "Streams are read-only, alter the source instead"
      );
    }
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    if (ctx.isPrivileged()) {
      return Promise.resolve({});
    }
    if (!query.source || !query.source.id) {
      return Promise.resolve({ id: null });
    }
    // Right now if you know the id of a stream's source, you can query that
    // stream. This may need to change at some point.
    return Promise.resolve({ source: { id: query.source.id } });
  }
}
