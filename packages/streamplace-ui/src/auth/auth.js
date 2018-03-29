// This file is web auth only, native auth gets redirected to auth.ios and auth.android
import auth0 from "auth0-js";
import jwtDecode from "jwt-decode";

const ID_TOKEN = "id_token";

const webAuth = new auth0.WebAuth({
  domain: "streamkitchen.auth0.com",
  clientID: "hZU06VmfYz2JLZCkjtJ7ltEy5SOsvmBA",
  responseType: "id_token",
  redirectUri: window.location.href
});

/**
 * Check if we're already logged in via auth0 giving us a hash parameter
 */
export async function checkLogin() {
  if (document.location.hash.includes(ID_TOKEN)) {
    const params = new URLSearchParams(document.location.hash.substring(1));
    if (!params.has(ID_TOKEN)) {
      return null;
    }
    // remove the hash portion of the url, icky
    window.history.replaceState(
      {},
      "",
      window.location.pathname + window.location.search
    );
    return params.get(ID_TOKEN);
  }
  return null;
}

export async function login({ email, password }) {
  webAuth.login({
    realm: "Username-Password-Authentication",
    username: email,
    password: password
  });
}
