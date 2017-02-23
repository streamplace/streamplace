
import Resource from "sp-resource";
import winston from "winston";
import {v4} from "node-uuid";

export default class User extends Resource {
  auth(ctx, doc) {
    return Promise.resolve();
  }

  authUpdate(ctx, doc, newDoc) {
    if (newDoc.handle.indexOf("new-user") !== -1) {
      throw new Resource.APIError(422, "Your handle may not contain 'new-user'.");
    }
  }

  // Nope. Happens through the context method further down.
  authCreate(ctx, doc) {
    throw new Resource.APIError(403, "You may not create users");
  }

  // Also nope.
  authDelete(ctx, doc) {
    throw new Resource.APIError(403, "You may not delete users");
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }

  default() {
    return super.default().then((doc) => {
      return {
        ...doc,
        id: v4(), // Necessary because authToken is our primary key
        handle: `new-user-${Math.round(Math.random() * 1000000)}`,
      };
    });
  }

  /**
   * Used upon new incoming traffic to create new users for folks as needed.
   */
  findOrCreateFromContext(ctx) {
    if (!ctx.jwt || typeof ctx.jwt.sub !== "string") {
      throw new Error("don't have ctx.jwt.sub");
    }
    const authToken = ctx.jwt.sub;
    return this.db.find(ctx, {authToken})
    .then((users) => {
      if (users.length === 1) {
        return Promise.resolve(users[0]);
      }
      if (users.length > 1) {
        throw new Error(`Data integrity violation: multiple users iwth authToken ${authToken}`);
      }

      // Okay, zero matches, need to create a user.
      const [userType, authId, ...rest] = authToken.split("|");
      if (!userType || !authId || rest.length > 0) {
        throw new Resource.APIError(422, "jwt.sub must be in the form 'userType|id'");
      }
      let newUser;
      return this.default().then((def) => {
        newUser = {
          ...def,
          authToken: authToken,
          roles: [],
        };
        if (userType === "service") {
          newUser.roles.push("SERVICE");
          newUser.handle = `service|${authId}`;
        }
        return this.validate(ctx, newUser);
      })
      .then(() => {
        return new Promise((resolve, reject) => {
          // Avoid a concurrency problem -- if multiple attempts happen, only let one succeed.
          this.db.insert(ctx, newUser).then((user) => {
            winston.info(`Created user ${user.handle} (${user.id}) for authToken ${authToken}`);
            resolve(user);
          })
          .catch((err) => {
            if (err.message.indexOf("Duplicate primary key") === -1) {
              return reject(err);
            }
            // Okay, someone else created it, let's look at theirs...
            this.db.find(ctx, {authToken}).catch(reject).then((docs) => {
              if (docs.length === 1) {
                return resolve(docs[0]);
              }
              throw new Error("MAYDAY: can't create a user, can't find a user?!?");
            });
          });
        });
      })
      .then((user) => {
        return user;
      });
    });
  }
}

User.tableName = "users";
User.primaryKey = "authToken";
User.indices = [
  "id",
];
