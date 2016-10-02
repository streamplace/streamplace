
import winston from "winston";

const req = function() {
  throw new Error("Missing required parameter");
};

export default class RethinkDbDriver {
  constructor({tableName = req()} = req()) {
    this.name = tableName;
    this._initCalled = false;
    this._initPromise = new Promise((resolve, reject) => {
      this._initResolve = resolve;
      this._initReject = reject;
    });
  }

  /**
   * Idempotent function to make sure our table exists, creating it if it doesn't, on startup. Can
   * be called at the start of every outward-facing function to queue everything up while we're
   * getting ready.
   */
  _init(ctx = req()) {
    if (!this._initCalled) {
      this._initCalled = true;
      const done = () => {
        ctx.rethink.table(this.name).wait().run(ctx.conn).then(this._initResolve).catch(this._initReject);
      };
      ctx.rethink.tableCreate(this.name).run(ctx.conn).then(done)
      .catch((err) => {
        if (!err.msg || err.msg.indexOf("alreadyExists") !== -1) {
          return this._initReject(err);
        }
        done();
      });
    }
    return this._initPromise;
  }

  find(ctx = req(), query = req()) {
    return this._init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).filter(query).run(ctx.conn);
    })
    .then((cursor) => {
      return cursor.toArray();
    });
  }

  findOne(ctx = req(), id = req()) {
    return this._init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).get(id).run(ctx.conn);
    });
  }

  upsert(ctx = req(), doc = req()) {
    return this._init(ctx)
    .then(() => {
      if (doc.id) {
        return ctx.rethink.table(this.name).get(doc.id).replace(doc).run(ctx.conn)
        .then((response) => {
          return doc;
        });
      }
      else {
        return ctx.rethink.table(this.name).insert(doc).run(ctx.conn)
        .then(({generated_keys}) => {
          doc.id = generated_keys[0];
          return doc;
        });
      }
    });
  }

  delete(ctx = req(), id = req()) {
    return this._init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).get(id).delete().run(ctx.conn);
    });
  }

  multiDelete(ctx = req(), query = req()) {
    return this._init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).filter(query).delete().run(ctx.conn);
    });
  }

  watch(ctx = req(), query = req()) {
    return this._init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).filter(query).changes({
        includeTypes: true,
        includeInitial: true,
        includeStates: true
      }).run(ctx.conn);
    });
  }
}
