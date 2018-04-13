import React from "react";
import { injectGlobal } from "styled-components";
import { IS_NATIVE } from "./polyfill";

export * from "./constants";
export * from "./polyfill";

if (!IS_NATIVE) {
  injectGlobal`
    html,
    body,
    main {
      height: 100%;
      font-family: "Open Sans", Helvetica, sans-serif !important;
    }
    main {
      display: flex;
    }
  `;
}

import Login from "./login";

export default class StreamplaceUI extends React.Component {
  render() {
    return <Login />;
  }
}
