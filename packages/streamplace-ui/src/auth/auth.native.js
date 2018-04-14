import Auth0 from "react-native-auth0";
import SP from "sp-client";
import {
  TOKEN_STORAGE_KEY,
  AUTH0_DOMAIN,
  AUTH0_CLIENT_ID,
  ID_TOKEN
} from "../constants";
import { AsyncStorage } from "react-native";

const auth0 = new Auth0({
  domain: AUTH0_DOMAIN,
  clientId: AUTH0_CLIENT_ID
});

/**
 * no-op; exists for API compatibility with the web auth module
 */
export async function checkLogin() {
  const token = await AsyncStorage.getItem(TOKEN_STORAGE_KEY);
  const user = await SP.connect({ token });
  // Allow the API server to issue us a new token, then...
  await AsyncStorage.setItem(TOKEN_STORAGE_KEY, SP.token);
  return user;
}

export async function login({ email, password }) {
  const authRes = await auth0.auth.passwordRealm({
    username: email,
    password: password,
    realm: "Username-Password-Authentication"
  });
  const user = await SP.connect({
    token: authRes.idToken
  });
  await AsyncStorage.setItem(TOKEN_STORAGE_KEY, SP.token);
  return user;
}

export async function logout() {
  await AsyncStorage.removeItem(TOKEN_STORAGE_KEY);
  SP.disconnect();
}
