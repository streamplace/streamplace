import PropTypes from "prop-types";
import React from "react";
import SP from "sp-client";
import styled from "styled-components";
import color from "color";

const Container = styled.div`
  height: 100%;
  display: flex;
  color: white;
  font-size: 1.3em;
  -webkit-font-smoothing: antialiased;
`;

const Emphasis = styled.span`
  color: #ffcf00;
  display: block;
  margin: 0.5em;
`;

const Centered = styled.div`
  margin: auto;
  width: 500px;
  text-align: center;
  padding: 1em;
`;

const Big = styled.h1`
  font-weight: 200;
`;

const Aside = styled.aside`
  font-style: oblique;
  margin: 1.5em auto;
  line-height: 1.2em;
`;

const Button = styled.button`
  font-size: 1.3em;
  background-color: transparent;
  padding: 0.7em 1.2em;
  margin: 1em;
  border: 1px solid black;
  cursor: pointer;
  &:focus {
    outline: none;
  }
`;

const lighter = color("#ffcf00")
  .lighten(0.5)
  .toString();

const GoodButton = styled(Button)`
  color: black;
  border-color: #ffcf00;
  background-color: #ffcf00;
  color: #333;
  &:hover,
  &:focus {
    background-color: ${lighter};
  }
`;

// formerly #ff2d2d
const BadButton = styled(Button)`
  color: #ffcf00;
  border-color: #ffcf00;
  &:hover,
  &:focus {
    background-color: #555;
    color: ${lighter};
    border-color: ${lighter};
  }
`;

// If you're working on this, this snippet will likely be helpful.
// SP.serverauths.find().then(auths => auths.forEach(auth => SP.serverauths.delete(auth.id)))

export default class AuthorizeServer extends React.Component {
  constructor(props) {
    super();
    this.state = {
      server: props.server,
      ready: false,
      needAuth: false
    };
  }

  componentWillMount() {
    this.serverAuthSub = SP.serverauths
      .watch({
        userId: SP.user.id,
        server: this.state.server
      })
      .on("data", ([auth]) => {
        if (auth) {
          this.setState({
            ready: true,
            needAuth: false
          });
          this.redirect(auth.jwt);
        } else {
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
      needAuth: false
    });
    SP.serverauths
      .create({
        userId: SP.user.id,
        server: this.state.server
      })
      .catch(err => {
        SP.error(err);
      });
  }

  handleNah() {
    // Send them back without a token. Sad!
    window.location = `${this.state.returnUrl}?authRejected=true`;
  }

  renderInner() {
    if (!this.state.ready) {
      return <div />;
    }
    if (this.state.needAuth === false) {
      // We're in the process of redirecting, just sit tight.
      return <div />;
    }
    return (
      <div>
        <Big>
          Log in to
          <Emphasis>{this.state.server}</Emphasis>
          using your Streamplace account?
        </Big>
        <Aside>
          All this means is that they get to see your email address. If we need
          other permissions, we&apos;ll ask again later.
        </Aside>
        <GoodButton onClick={this.handleYeah.bind(this)}>Yeah</GoodButton>
        <BadButton onClick={this.handleNah.bind(this)}>Nah</BadButton>
      </div>
    );
  }

  render() {
    return (
      <Container>
        <Centered>{this.renderInner()}</Centered>
      </Container>
    );
  }
}

AuthorizeServer.propTypes = {
  server: PropTypes.string.isRequired,
  returnPath: PropTypes.string.isRequired,
  onToken: PropTypes.func.isRequired,
  onRejected: PropTypes.func.isRequired
};
