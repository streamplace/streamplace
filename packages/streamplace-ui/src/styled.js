import styled from "styled-components";
import ReactNative from "react-native";
import React from "react";
import { IS_NATIVE } from "./polyfill";

if (!IS_NATIVE) {
  // mildly hacky check to give styled-components all the RN stuff
  Object.keys(ReactNative).forEach(key => {
    if (!styled[key] && ReactNative[key].prototype instanceof React.Component) {
      styled[key] = styled(ReactNative[key]);
    }
  });
}

export default styled;
