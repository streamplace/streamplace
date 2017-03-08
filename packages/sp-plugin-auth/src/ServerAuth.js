
import Resource from "sp-resource";
import winston from "winston";
import {v4} from "node-uuid";
import {spawnSync} from "child_process";
import {resolve} from "path";
import config from "sp-configuration";
import {parse} from "url";
import dove from "dove-jwt";

dove.useSystemCertAuthorities();

// Download correct TLS cert from kubernetes.
let stderr;
let cert;
let key;
const AUTH_ISSUER = config.require("AUTH_ISSUER");

try {
  const {hostname} = parse(AUTH_ISSUER);
  const results = spawnSync("kubectl", ["get", "secret", "-o", "json", hostname]);
  stderr = results.stderr.toString();
  const stdout = results.stdout.toString();
  const tls = JSON.parse(stdout);
  cert = Buffer.from(tls.data["tls.crt"], "base64").toString();
  key = Buffer.from(tls.data["tls.key"], "base64").toString();
}
catch(e) {
  winston.error("Error getting cert to create auth dove-jwts", e);
  if (stderr) {
    winston.error(stderr);
  }
  throw e;
}

export default class ServerAuth extends Resource {
  auth(ctx, doc) {
    return Promise.resolve();
  }

  authUpdate(ctx, doc, newDoc) {
    throw new Resource.APIError(403, "You may not modify existing ServerAuths.");
  }

  authCreate(ctx, doc) {
    if (!ctx.user || ctx.user.id !== doc.userId) {
      throw new Resource.APIError(403, "You may only make ServerAuths for yourself");
    }
  }

  authDelete(ctx, doc) {
    if (!ctx.user || ctx.user.id !== doc.userId) {
      throw new Resource.APIError(403, "You may only delete your own ServerAuths.");
    }
  }

  authQuery(ctx, query) {
    return Promise.resolve({userId: ctx.user.id});
  }

  /**
   * Give 'em a jwt on the way out! It's only polite.
   */
  transform(ctx, doc) {
    return super.transform(ctx, doc).then((doc) => {
      const jwt = dove.sign({}, key, cert, {
        audience: `https://${doc.server}/`,
        expiresIn: "3m", // Expires really quick 'cuz it should get used right away
        subject: `${AUTH_ISSUER}api/users/${ctx.user.id}`,
      });
      return {...doc, jwt};
    });
  }
}
