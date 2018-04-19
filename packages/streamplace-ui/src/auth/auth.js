// This file is web auth only, native auth gets redirected to auth.ios and auth.android
import auth0 from "auth0-js";
import jwtDecode from "jwt-decode";
import SP from "sp-client";
import { IS_ELECTRON } from "../polyfill";
import {
  TOKEN_STORAGE_KEY,
  PROFILE_STORAGE_KEY,
  AUTH0_DOMAIN,
  AUTH0_CLIENT_ID,
  ID_TOKEN,
  AUTH0_REALM
} from "../constants";

const webAuth = new auth0.WebAuth({
  domain: AUTH0_DOMAIN,
  clientID: AUTH0_CLIENT_ID,
  responseType: ID_TOKEN,
  redirectUri: window.location.href
});

/**
 * Wrapper for checkLogin that emits events for Electron as appopriate.
 */
export async function checkLogin() {
  const result = await _checkLogin();
  if (IS_ELECTRON) {
    if (result) {
      window.streamplaceElectronCallback({
        profile: await getProfile(),
        token: SP.token
      });
    } else {
      window.streamplaceElectronCallback(null);
    }
  }
  return result;
}

/**
 * Check if we're already logged in. There are two ways that can be true.
 */
export async function _checkLogin() {
  let token;
  /**
   * Way one: we're returning from an Auth0 redirect with a shiny id_token
   */
  if (document.location.hash.includes(ID_TOKEN)) {
    const params = new URLSearchParams(document.location.hash.substring(1));
    if (params.has(ID_TOKEN)) {
      // remove the hash portion of the url, icky
      window.history.replaceState(
        {},
        "",
        window.location.pathname + window.location.search
      );
      token = params.get(ID_TOKEN);
      try {
        const profile = jwtDecode(token);
        window.localStorage.setItem(
          PROFILE_STORAGE_KEY,
          JSON.stringify(profile)
        );
      } catch (e) {
        token = null;
      }
    }
  }
  /**
   * Way two:
   */
  if (!token) {
    token = window.localStorage.getItem(TOKEN_STORAGE_KEY);
  }
  if (!token) {
    return null;
  }
  try {
    const user = await SP.connect({ token });
    // Allow the API server to issue us a new token, then...
    window.localStorage.setItem(TOKEN_STORAGE_KEY, SP.token);
    return user;
  } catch (e) {
    // 999999 reasons this can happen
    return null;
  }
}

export async function login({ email, password }) {
  webAuth.login({
    realm: AUTH0_REALM,
    username: email,
    password: password
  });
}

export async function logout() {
  SP.disconnect();
  window.localStorage.removeItem(TOKEN_STORAGE_KEY);
  window.localStorage.removeItem(PROFILE_STORAGE_KEY);
}

export async function getProfile() {
  const profileStr = await window.localStorage.getItem(PROFILE_STORAGE_KEY);
  if (!profileStr) {
    return null;
  }
  return JSON.parse(profileStr);
}
