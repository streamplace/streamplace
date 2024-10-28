// Don't add anything to this file! It needs to be minimal so that
// hot module reloading works properly on web.

import "@expo/metro-runtime";
import { registerRootComponent } from "expo";
import Router from "./router";

registerRootComponent(Router);
