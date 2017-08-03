import Resource from "sp-resource";
import winston from "winston";
import { v4 } from "node-uuid";
import uuidCheck from "uuid-validate";
import config from "sp-configuration";
import { parse as parseUrl } from "url";

export default class User extends Resource {
  auth(ctx, doc) {
    return Promise.resolve();
  }

  authUpdate(ctx, doc, newDoc) {
    if (newDoc.handle.indexOf("new-user") !== -1) {
      throw new Resource.APIError(
        422,
        "Your handle may not contain 'new-user'."
      );
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
    return super.default().then(doc => {
      return {
        ...doc,
        id: v4(), // Necessary because authToken is our primary key
        handle: `new-user-${Math.round(Math.random() * 1000000)}`
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
    // TODO this should really be thought through more... for now this handles the normalization
    // of auth0 tokens to streamplace tokens.
    let identity = ctx.jwt.sub;

    return this.db.find(ctx, { identity }).then(users => {
      if (users.length === 1) {
        return Promise.resolve(users[0]);
      }
      if (users.length > 1) {
        throw new Error(
          `Data integrity violation: multiple users with identity ${identity}`
        );
      }

      // Okay, zero matches, need to create a user. By this point, the API server has made sure
      // that we have something of the form https://stream.place/users/uuid-goes-here, so let's
      // sanity check that and proceed.
      const identity = ctx.jwt.sub;
      const { path } = parseUrl(identity);
      const [apiPath, usersPath, newId, ...rest] = path
        .split("/")
        .filter(s => s !== "");
      if (
        apiPath !== "api" ||
        usersPath !== "users" ||
        !newId ||
        rest.length > 0
      ) {
        throw new Error(`Unknown form for user identity: ${ctx.jwt.sub}`);
      }
      let newUser;
      return this.default()
        .then(def => {
          newUser = {
            ...def,
            identity: identity,
            roles: []
          };
          if (uuidCheck(newId)) {
            newUser.id = newId; // Regular users already have one, services get a random one
          } else if (ctx.jwt.roles && ctx.jwt.roles.includes("SERVICE")) {
            newUser.roles = ctx.jwt.roles;
            newUser.handle = newId;
          } else {
            winston.error(
              "MAYDAY we're supposed to create a user but it's not a service and doesn't have a UUID?"
            );
            throw new Error("User creation failed");
          }
          return this.validate(ctx, newUser);
        })
        .then(() => {
          return new Promise((resolve, reject) => {
            // Avoid a concurrency problem -- if multiple attempts happen, only let one succeed.
            this.db
              .insert(ctx, newUser)
              .then(user => {
                winston.info(
                  `Created user ${user.handle} (${user.id}) for identity ${identity}`
                );
                resolve(user);
              })
              .catch(err => {
                if (err.message.indexOf("Duplicate primary key") === -1) {
                  return reject(err);
                }
                // Okay, someone else created it, let's look at theirs...
                this.db.find(ctx, { identity }).catch(reject).then(docs => {
                  if (docs.length === 1) {
                    return resolve(docs[0]);
                  }
                  throw new Error(
                    "MAYDAY: can't create a user, can't find a user?!?"
                  );
                });
              });
          });
        })
        .then(user => {
          return user;
        });
    });
  }
}

User.primaryKey = "identity";
User.indices = ["id"];
