// This file is web auth only, native auth gets redirected to auth.ios and auth.android
import { IS_NATIVE } from "./polyfill";
import "auth0-js";
export async function login({ email, password }) {}
