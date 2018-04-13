import React from "react";
import ReactDOM from "react-dom";
import "./index.css";
import CorporateBullshit from "./corporate-bullshit";
import { injectGlobal } from "styled-components";
import StreamplaceUI, { IS_ELECTRON, TOKEN_STORAGE_KEY } from "streamplace-ui";

injectGlobal`
  body {
    font-family: "Open Sans", Helvetica, Arial, sans-serif;
    margin: 0;
  }
`;

/**
 * THis component is the root of all web-based StreamplaceUI stuff, which includes Electron. In
 * Electron we just delegate everything to StreamplaceUI, but on browser we wanna show the
 * corporate site sometimes, so...
 */
export default class StreamplaceWebRoot extends React.Component {
  constructor() {
    super();
    let showBullshit = true;
    if (IS_ELECTRON) {
      showBullshit = false;
    } else if (localStorage.getItem(TOKEN_STORAGE_KEY)) {
      showBullshit = false;
    }
    this.state = {
      showBullshit
    };
  }

  render() {
    if (this.state.showBullshit) {
      return (
        <CorporateBullshit
          onLogin={() => this.setState({ showBullshit: false })}
        />
      );
    }
    return <StreamplaceUI />;
  }
}

ReactDOM.render(<StreamplaceWebRoot />, document.querySelector("main"));

// Uncomment me when it's go time:

// ReactDOM.render(<StreamplaceUI />, document.querySelector("main"));
