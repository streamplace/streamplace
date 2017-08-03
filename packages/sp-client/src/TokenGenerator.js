import nJwt from "njwt";
import config from "sp-configuration";
import os from "os";

const JWT_SECRET = config.require("JWT_SECRET");
const JWT_AUDIENCE = config.require("JWT_AUDIENCE");
const JWT_SECRET_DECODED = Buffer.from(JWT_SECRET, "base64");
const JWT_EXPIRATION = config.require("JWT_EXPIRATION");
const APP_NAME = config.require("APP_NAME");
const DOMAIN = config.require("DOMAIN");

const myIdentity = `https://${DOMAIN}/api/users/${APP_NAME}`;

export default class TokenGenerator {
  constructor({ app }) {
    this.expiry = JWT_EXPIRATION;
  }

  generate() {
    const claims = {
      iss: myIdentity,
      sub: myIdentity,
      aud: JWT_AUDIENCE,
      roles: ["SERVICE"]
    };
    const jwt = nJwt.create(claims, JWT_SECRET_DECODED);
    jwt.setExpiration(Date.now() + 24 * 60 * 60 * 1000); // One day, plz
    return jwt.compact();
  }
}
