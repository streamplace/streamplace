import React from "react";
import ReactDOM from "react-dom";
import CorporateBullshit from "./corporate-bullshit";
import { injectGlobal } from "styled-components";
import StreamplaceUI, {
  IS_BROWSER,
  TOKEN_STORAGE_KEY,
  checkLogin
} from "streamplace-ui";

injectGlobal`
  body {
    font-family: "Open Sans", Helvetica, Arial, sans-serif;
    margin: 0;
  }
  main {
    display: block;
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
    if (localStorage.getItem(TOKEN_STORAGE_KEY)) {
      showBullshit = false;
    }
    this.state = {
      showBullshit,
      loading: true
    };
  }

  async componentDidMount() {
    // We only get a second to try before we show _something_.
    setTimeout(() => {
      this.setState({ loading: false });
    }, 2500);
    // But yeah, try and get the user.
    const user = await checkLogin();
    this.setState({ loading: false, showBullshit: !user });
  }

  render() {
    if (this.state.loading) {
      return <div />;
    }
    if (this.state.showBullshit && IS_BROWSER) {
      return (
        <CorporateBullshit
          onLogin={() => this.setState({ showBullshit: false })}
        />
      );
    }
    return (
      <StreamplaceUI
        onLoggedOut={async () => {
          // when the log out, self-destruct
          this.setState({
            loading: true
          });
          window.location.href = "/";
        }}
      />
    );
  }
}

ReactDOM.render(<StreamplaceWebRoot />, document.querySelector("main"));

// Uncomment me when it's go time:

// ReactDOM.render(<StreamplaceUI />, document.querySelector("main"));
