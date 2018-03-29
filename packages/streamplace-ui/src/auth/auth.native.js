import Auth0 from "react-native-auth0";
const auth0 = new Auth0({
  domain: "streamkitchen.auth0.com",
  clientID: "hZU06VmfYz2JLZCkjtJ7ltEy5SOsvmBA"
});

/**
 * no-op; exists for API compatibility with the web auth module
 */
export function checkLogin() {}

export function login({ email, password }) {
  auth0.auth.passwordRealm({
    username: email,
    password: password,
    realm: "Username-Password-Authentication"
  });
  // .then(console.log)
  // .catch(console.error);
}
