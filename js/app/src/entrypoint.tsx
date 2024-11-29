// Don't add anything to this file! It needs to be minimal so that
// hot module reloading works properly on web.

import "@expo/metro-runtime";
import "./polyfills";
import Router from "./router";
import { registerRootComponent } from "expo";

registerRootComponent(Router);
