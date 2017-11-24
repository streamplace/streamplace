import React, { Component } from "react";
import "normalize.css";
import "./App.css";
import SP from "sp-client";
import qs from "qs";
import SPRouter from "./sp-router";
import Streamplace from "./streamplace";
import styled from "styled-components";
import "font-awesome/css/font-awesome.css";
import cookie from "cookie";

// We want a few things to behave differently if we're an app, so let's get a CSS class to make
// that easy.
if (typeof document !== "undefined" && document.cookie) {
  const cookies = cookie.parse(document.cookie);
  if (cookies.appMode === "true") {
    document.body.className = "app";
  }
}

const Everything = styled.div`
  height: 100%;
`;

const Centered = styled.div`
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const START = Symbol();
const LOGGED_IN = Symbol();
const LOGGED_OUT = Symbol();

class SPFrontend extends Component {
  constructor() {
    super();
    this.state = {
      unexpectedError: false,
      ready: false,
      user: null,
      phase: START
    };
  }

  /**
   * Even if we don't have a token in localStorage, we still attempt login because
   * that process has sp-client download the schema for the server, populating the
   * `loginUrl` parameter that we can use to log the user in.
   */
  componentWillMount() {
    const query = qs.parse(document.location.search.slice(1));
    let token = window.localStorage.getItem("SP_AUTH_TOKEN");
    // If we have a token in our URL, use it to log in a nd clear it from the URL
    if (query.token) {
      token = query.token;
      delete query.token;
      const queryString = qs.stringify(query);
      let newUrl = window.location.origin + window.location.pathname;
      // If there were other params in the query string, keep them around
      if (queryString !== "") {
        newUrl += `?${queryString}`;
      }
      window.history.replaceState({}, "", newUrl);
    }
    this.tryLogin(token);
  }

  tryLogin(token) {
    // This SPClient might not succeed in connection to the server 'cuz we're
    // the login page, but that's fine because we're just using it to get the
    // schema.
    return SP.connect({ token })
      .then(user => {
        window.localStorage.setItem("SP_AUTH_TOKEN", SP.token);
        this.setState({
          phase: LOGGED_IN,
          user: user
        });
      })
      .catch(err => {
        // This is kinda interesting, it's the highest level catch() in the app.
        // It catches a lot of things in development, 'cuz if you typo anywhere
        // it ends up here.
        if (err.status === 401 || err.status === 403) {
          this.handleLogout();
        } else {
          SP.error("Unhandled exception upon login", err);
          this.setState({ unexpectedError: true });
        }
      });
  }

  handleLogout() {
    const loginOrigin = SP.schema.plugins["sp-plugin-core"].loginUrl.slice(
      0,
      -1
    );
    const loginUrl =
      loginOrigin +
      "?" +
      qs.stringify({
        server: window.location.hostname,
        returnPath: "/"
      });
    window.localStorage.removeItem("SP_AUTH_TOKEN");
    window.location = loginUrl;
    // this.setState({
    //   phase: LOGGED_OUT,
    //   loginOrigin: loginOrigin,
    //   loginUrl: loginUrl,
    // });
  }

  handleLoginBtn(e) {
    // Hello other developers. This thing used to work with iframe postmessages. It was kind of
    // cool, but also kind of silly? Now it works with redirects, which is easier, but kind of
    // less badass? Let me know if you have smarter ideas for how this should operate.
    // e.preventDefault();
    // const loginWindow = window.open(this.state.loginUrl, "SPLogin");
    // const theirOrigin = this.state.loginOrigin;
    // const interval = setInterval(() => {
    //   loginWindow.postMessage("hello", theirOrigin);
    // }, 10);
    // const handleReply = (e) => {
    //   if (e.origin !== theirOrigin) {
    //     SP.error(`Rejected message from unknown origin ${e.origin}`);
    //     return;
    //   }
    //   if (e.data === "hello") {
    //     clearInterval(interval);
    //     SP.info(`Bidirectional communication with ${theirOrigin} established.`);
    //   }
    //   if (e.data.type === "token") {
    //     SP.info("Received token!");
    //     this.tryLogin(e.data.token);
    //   }
    //   else {
    //     SP.error(`Unknown message from ${theirOrigin}`, e.data);
    //   }
    // };
    // window.addEventListener("message", handleReply);
  }

  renderInner() {
    if (this.state.phase === START) {
      return <div />;
    }
    if (this.state.phase === LOGGED_IN) {
      return (
        <Streamplace SP={SP}>
          <SPRouter onLogout={() => this.handleLogout()} foo="bar" />
        </Streamplace>
      );
    }
    if (this.state.phase === LOGGED_OUT) {
      return (
        <Centered>
          <a
            onClick={this.handleLoginBtn.bind(this)}
            href={this.state.loginUrl}
          >
            Log in NAOW
          </a>
        </Centered>
      );
    }
  }

  render() {
    if (this.state.unexpectedError) {
      return (
        <p>
          Unexpected error.&nbsp;
          <a
            href="#"
            onClick={() => {
              this.handleLogout();
            }}
          >
            Log out?
          </a>
        </p>
      );
    }
    return <Everything>{this.renderInner()}</Everything>;
  }
}

export default SPFrontend;
