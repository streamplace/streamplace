import React from "react";
import { injectGlobal } from "styled-components";
import { IS_NATIVE, IS_ELECTRON } from "./polyfill";

export * from "./constants";
export * from "./polyfill";
export * from "./auth/auth";

if (!IS_NATIVE) {
  injectGlobal`
    html,
    body,
    main {
      height: 100%;
      font-family: "Open Sans", Helvetica, sans-serif !important;
      ${IS_ELECTRON && "overflow-y: hidden;"}
    }
  `;
}

import Login from "./login";

export default class StreamplaceUI extends React.Component {
  render() {
    return <Login onLoggedOut={this.props.onLoggedOut} />;
  }
}
