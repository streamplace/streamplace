
import React, { Component } from "react";
import SP from "sp-client";
import Auth0Login from "./Auth0Login";

const START = Symbol();
const LOGGED_IN = Symbol();
const LOGGED_OUT = Symbol();

class App extends Component {
  constructor() {
    super();
    this.state = {
      phase: START
    };
  }

  componentWillMount() {
    // This SPClient might not succeed in connection to the server 'cuz we're the login page, but
    // that's fine because we're just using it to get the schema.
    SP.connect()
    .then((user) => {
      this.setState({phase: LOGGED_IN});
    })
    .catch((err) => {
      this.setState({phase: LOGGED_OUT});
    });
  }

  componentDidMount() {

  }

  handleLogin(token) {
    SP.connect({token})
    .then((user) => {

    })
    .catch((err) => {

    });
  }

  render() {
    if (this.state.phase === START) {
      return <div>Loading</div>;
    }
    else if (this.state.phase === LOGGED_IN) {
      return <div>Logged in!</div>;
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
