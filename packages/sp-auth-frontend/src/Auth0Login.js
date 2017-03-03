
import React from "react";
import Auth0Lock from "auth0-lock";

export default class Auth0Login extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  componentDidMount() {
    this.lock = new Auth0Lock(this.props.auth0Audience, this.props.auth0Domain, {
      theme: {
        // logo: logoURL,
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
      }
    });
    this.lock.show();
    this.lock.on("authenticated", ({idToken}) => {
      this.props.onLogin(idToken);
    });
    this.lock.on("authorization_error", (err) => {
      throw new Error(err);
    });
  }

  componentWillUnmount() {
    this.lock.hide();
  }

  render () {
    return (
      <div id="Lock0-Container"></div>
    );
  }
}

Auth0Login.propTypes = {
  "auth0Audience": React.PropTypes.string.isRequired,
  "auth0Domain": React.PropTypes.string.isRequired,
  "onLogin": React.PropTypes.func.isRequired,
};
