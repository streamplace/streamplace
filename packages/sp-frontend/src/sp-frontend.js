
import React, { Component } from "react";
import "normalize.css";
import "./App.css";
import SP from "sp-client";
import qs from "qs";
import SPRouter from "./sp-router";
import Streamplace from "./streamplace";
import styled, {injectGlobal} from "styled-components";
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

/* eslint-disable no-unused-expressions */
injectGlobal`
  a {
    text-decoration: none;
  }
`;

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
      ready: false,
      user: null,
      phase: START,
    };
  }

/**
 * Even if we don't have a token in localStorage, we still attempt login because that process has
 * sp-client download the schema for the server, populating the `loginUrl` parameter that we can
 * use to log the user in.
 */
  componentWillMount() {
    this.tryLogin(window.localStorage.getItem("SP_AUTH_TOKEN"));
  }

  tryLogin(token) {
    // This SPClient might not succeed in connection to the server 'cuz we're the login page, but
    // that's fine because we're just using it to get the schema.
    return SP.connect({token})
    .then((user) => {
      window.localStorage.setItem("SP_AUTH_TOKEN", SP.token);
      this.setState({
        phase: LOGGED_IN,
        user: user,
      });
    })
    .catch((err) => {
      SP.error(err);
      this.handleLogout();
    });
  }

  handleLogout() {
    const loginOrigin = SP.schema.plugins["sp-plugin-core"].loginUrl.slice(0, -1);
    const loginUrl = loginOrigin + "?" + qs.stringify({
      server: window.location.hostname,
      returnPath: "/"
    });
    this.setState({
      phase: LOGGED_OUT,
      loginOrigin: loginOrigin,
      loginUrl: loginUrl,
    });
    window.localStorage.removeItem("SP_AUTH_TOKEN");
  }

  handleLoginBtn(e) {
    e.preventDefault();
    const loginWindow = window.open(this.state.loginUrl);
    const theirOrigin = this.state.loginOrigin;
    const interval = setInterval(() => {
      loginWindow.postMessage("hello", theirOrigin);
    }, 10);
    const handleReply = (e) => {
      if (e.origin !== theirOrigin) {
        SP.error(`Rejected message from unknown origin ${e.origin}`);
        return;
      }
      if (e.data === "hello") {
        clearInterval(interval);
        SP.info(`Bidirectional communication with ${theirOrigin} established.`);
      }
      if (e.data.type === "token") {
        SP.info("Received token!");
        this.tryLogin(e.data.token);
      }
      else {
        SP.error(`Unknown message from ${theirOrigin}`, e.data);
      }
    };
    window.addEventListener("message", handleReply);
  }

  renderInner() {
    if (this.state.phase === START) {
      return <div>Loading...</div>;
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
          <a onClick={this.handleLoginBtn.bind(this)} href={this.state.loginUrl}>Log in?</a>
        </Centered>
      );
    }
  }


  render() {
    return (
      <Everything>
        {this.renderInner()}
      </Everything>
    );
  }
}

export default SPFrontend;
