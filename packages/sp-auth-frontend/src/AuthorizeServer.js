
import React from "react";
import SP from "sp-client";
import qs from "qs";

// If you're working on this, this snippet will likely be helpful.
// SP.serverauths.find().then(auths => auths.forEach(auth => SP.serverauths.delete(auth.id)))

export default class AuthorizeServer extends React.Component{
  constructor(props) {
    super();
    this.state = {
      server: props.server,
      ready: false,
      needAuth: false,
    };
  }

  componentWillMount() {
    this.serverAuthSub = SP.serverauths.watch({
      userId: SP.user.id,
      server: this.state.server,
    })
    .on("data", ([auth]) => {
      if (auth) {
        this.setState({
          ready: true,
          needAuth: false
        });
        this.redirect(auth.jwt);
      }
      else {
        this.setState({
          ready: true,
          needAuth: true
        });
      }
    });
  }

  componentWillUnmount() {
    this.serverAuthSub.stop();
  }

  redirect(token) {
    if (token) {
      this.props.onToken(token);
    }
    this.props.onRejected();
  }

  handleYeah() {
    this.setState({
      ready: false,
      needAuth: false,
    });
    SP.serverauths.create({
      userId: SP.user.id,
      server: this.state.server,
    })
    .catch((err) => {
      SP.error(err);
    });
  }

  handleNah() {
    // Send them back without a token. Sad!
    window.location = `${this.state.returnUrl}?authRejected=true`;
  }

  render () {
    if (!this.state.ready) {
      return <div>Loading...</div>;
    }
    if (this.state.needAuth === false) {
      return (
        <div>
          <div>Logged in! Returning you to {this.state.server}</div>
        </div>
      );
    }
    return (
      <div>
        <div>Log in to {this.state.server} using your Streamplace account?</div>
        <aside>(All this means is that they get to see your email address. If we need other permissions, we'll ask again later.)</aside>
        <button onClick={this.handleYeah.bind(this)}>Yeah</button>
        <button onClick={this.handleNah.bind(this)}>Nah</button>
      </div>
    );
  }
}

AuthorizeServer.propTypes = {
  "server": React.PropTypes.string.isRequired,
  "returnPath": React.PropTypes.string.isRequired,
  "onToken": React.PropTypes.func.isRequired,
  "onRejected": React.PropTypes.func.isRequired,
};
