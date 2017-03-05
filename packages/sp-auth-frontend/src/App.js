
import React, { Component } from "react";
import SP from "sp-client";
import qs from "qs";
import Auth0Login from "./Auth0Login";
import AuthorizeServer from "./AuthorizeServer";

const START = Symbol();
const LOGGED_IN = Symbol();
const LOGGED_OUT = Symbol();

class App extends Component {
  constructor() {
    super();
    const query = qs.parse(document.location.search.slice(1));
    this.state = {
      phase: START,
      logout: query.logout === "true",
      server: query.server || "stream.place",
      returnPath: query.returnPath || "/",
      noRedirect: query.noRedirect === "true",
    };
  }

  componentWillMount() {
    if (this.state.logout) {
      window.localStorage.removeItem("SP_AUTH_TOKEN");
    }
    // This SPClient might not succeed in connection to the server 'cuz we're the login page, but
    // that's fine because we're just using it to get the schema.
    const token = window.localStorage.getItem("SP_AUTH_TOKEN");
    SP.connect({token})
    .then((user) => {
      this.setState({phase: LOGGED_IN});
    })
    .catch((err) => {
      this.setState({phase: LOGGED_OUT});
      window.localStorage.removeItem("SP_AUTH_TOKEN");
    });
  }

  componentDidMount() {

  }

  handleLogin(token) {
    SP.connect({token})
    .then((user) => {
      this.setState({phase: LOGGED_IN});
      window.localStorage.setItem("SP_AUTH_TOKEN", SP.token);
    })
    .catch((err) => {
      SP.error(err);
      this.setState({phase: LOGGED_OUT});
    });
  }

  render() {
    if (this.state.phase === START) {
      return <div>Loading</div>;
    }
    else if (this.state.phase === LOGGED_IN) {
      return <AuthorizeServer
        server={this.state.server}
        returnPath={this.state.returnPath}
        noRedirect={this.state.noRedirect} />;
    }
    else if (this.state.phase === LOGGED_OUT) {
      const {auth0Audience, auth0Domain} = SP.schema.plugins["sp-auth"];
      return (
        <Auth0Login
          auth0Audience={auth0Audience}
          auth0Domain={auth0Domain}
          onLogin={this.handleLogin.bind(this)} />
      );
    }
  }
}

export default App;
