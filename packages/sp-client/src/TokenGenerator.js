import nJwt from "njwt";
import config from "sp-configuration";
import os from "os";
import EE from "wolfy87-eventemitter";

const JWT_SECRET = config.require("JWT_SECRET");
const JWT_AUDIENCE = config.require("JWT_AUDIENCE");
const JWT_SECRET_DECODED = Buffer.from(JWT_SECRET, "base64");
const JWT_EXPIRATION = config.require("JWT_EXPIRATION");
const APP_NAME = config.require("APP_NAME");
const DOMAIN = config.require("DOMAIN");

const myIdentity = `https://${DOMAIN}/api/users/${APP_NAME}`;
const TOKEN_EXPIRY = 24 * 60 * 60 * 1000; // 1 day

export default class TokenGenerator extends EE {
  constructor() {
    super();
    setInterval(() => {
      this.emit("token", this.generate());
    }, Math.floor(TOKEN_EXPIRY / 2));
  }

  generate() {
    const claims = {
      iss: myIdentity,
      sub: myIdentity,
      aud: JWT_AUDIENCE,
      roles: ["SERVICE"]
    };
    const jwt = nJwt.create(claims, JWT_SECRET_DECODED);
    jwt.setExpiration(Date.now() + TOKEN_EXPIRY); // One day, plz
    return jwt.compact();
  }
}
