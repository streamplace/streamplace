
import Resource from "sk-resource";
import winston from "winston";

export default class User extends Resource {
  auth(ctx, doc) {
    return Promise.resolve();
  }

  // Nope.
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
        handle: `new-user-${Math.round(Math.random() * 100000)}`,
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
        return this.db.upsert(ctx, newUser);
      })
      .then((user) => {
        winston.info(`Created user ${user.handle} (${user.id}) for authToken ${authToken}`);
        return user;
      });
    });
  }
}

User.tableName = "users";
