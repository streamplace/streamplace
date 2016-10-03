
import nJwt from "njwt";
import config from "sk-config";
import os from "os";

const JWT_SECRET = config.require("JWT_SECRET");
const PUBLIC_JWT_AUDIENCE = config.require("PUBLIC_JWT_AUDIENCE");
const JWT_SECRET_DECODED = Buffer.from(JWT_SECRET, "base64");
const JWT_EXPIRY = config.require("JWT_EXPIRY");
const APP_NAME = config.require("APP_NAME");

export default class TokenGenerator {
  constructor({app}) {
    this.expiry = JWT_EXPIRY;
  }

  generate() {
    const claims = {
      iss: "https://stream.kitchen",
      sub: `service|${APP_NAME}`,
      aud: PUBLIC_JWT_AUDIENCE
    };
    const jwt = nJwt.create(claims, JWT_SECRET_DECODED);
    jwt.setExpiration(Date.now() + (24 * 60 * 60 * 1000)); // One day, plz
    return jwt.compact();
  }
}
