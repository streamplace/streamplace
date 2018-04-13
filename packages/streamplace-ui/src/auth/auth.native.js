import Auth0 from "react-native-auth0";
import SP from "sp-client";
const auth0 = new Auth0({
  domain: "streamkitchen.auth0.com",
  clientId: "hZU06VmfYz2JLZCkjtJ7ltEy5SOsvmBA"
});

/**
 * no-op; exists for API compatibility with the web auth module
 */
export async function checkLogin() {}

export async function login({ email, password }) {
  const authRes = await auth0.auth.passwordRealm({
    username: email,
    password: password,
    realm: "Username-Password-Authentication"
  });
  const user = await SP.connect({
    token: authRes.idToken
  });
  return user;
}

export async function logout() {}
