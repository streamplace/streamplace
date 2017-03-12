
import React, { Component } from "react";
import "./App.css";
import SP from "sp-client";
import qs from "qs";
import SPRouter from "./sp-router";

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
    });
  }

  handleLoginBtn(e) {
    e.preventDefault();
    const loginWindow = window.open(this.state.loginUrl, "StreamplaceLogin");
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

  handleLogout() {
    window.localStorage.removeItem("SP_AUTH_TOKEN");
    window.location = window.location;
  }

  render() {
    if (this.state.phase === START) {
      return <div>Loading...</div>;
    }
    if (this.state.phase === LOGGED_IN) {
      return (
        <SPRouter />
      );
    }
    if (this.state.phase === LOGGED_OUT) {
      return (
        <div>
          <a onClick={this.handleLoginBtn.bind(this)} href={this.state.loginUrl}>Log in?</a>
        </div>
      );
    }
  }
}

export default SPFrontend;
