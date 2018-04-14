import { config } from "sp-client";
import { IS_NATIVE } from "./polyfill";
import url from "url";

export const TOKEN_STORAGE_KEY = "SP_TOKEN";
export const PROFILE_STORAGE_KEY = "SP_PROFILE";
export const ID_TOKEN = "id_token";
export const AUTH0_CLIENT_ID = config.require("JWT_AUDIENCE");
export const AUTH0_REALM = "Username-Password-Authentication";

// convert https://streamkitchen.auth0.com/ to streamkitchen.auth0.com
export const AUTH_ISSUER = config.require("AUTH_ISSUER");
const issuerUrl = url.parse(AUTH_ISSUER);
export const AUTH0_DOMAIN = issuerUrl.host;
