import PropTypes from "prop-types";
import React from "react";
import Auth0Lock from "auth0-lock";
import "./Auth0Login.css";
import logoUrl from "./streamplace_tight.png";

export default class Auth0Login extends React.Component {
  constructor() {
    super();
    this.state = {
      done: false
    };
    this.handleAuthSuccess = this.handleAuthSuccess.bind(this);
    this.handleAuthError = this.handleAuthError.bind(this);
  }

  componentDidMount() {
    this.lock = new Auth0Lock(
      this.props.auth0Audience,
      this.props.auth0Domain,
      {
        theme: {
          logo: logoUrl,
          primaryColor: "#333333"
        },
        languageDictionary: {
          title: ""
        },
        closable: false,
        redirectUrl: "http://drumstick.iame.li:60000",
        responseType: "code",
        auth: {
          sso: false,
          redirect: false
        },
        usernameStyle: "email"
      }
    );
    window.lock = this.lock;
    this.lock.on("authenticated", this.handleAuthSuccess);
    this.lock.on("authorization_error", this.handleAuthError);
    this.lock.show();
  }

  handleAuthSuccess({ idToken }) {
    this.props.onLogin(idToken);
  }

  handleAuthError(err) {
    // console.error(err);
  }

  componentWillUnmount() {
    this.lock.removeListener("authenticated", this.handleAuthSuccess);
    this.lock.removeListener("authorization_error", this.handleAuthError);
    this.lock.hide();
  }

  render() {
    return <div id="Lock0-Container" />;
  }
}

Auth0Login.propTypes = {
  auth0Audience: PropTypes.string.isRequired,
  auth0Domain: PropTypes.string.isRequired,
  onLogin: PropTypes.func.isRequired
};
