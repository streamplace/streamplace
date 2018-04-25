import React from "react";
import ReactNative, {
  Text,
  View,
  StyleSheet,
  Modal,
  TouchableHighlight,
  TextInput,
  Image,
  Button,
  Linking
} from "react-native";
import { IS_NATIVE, IS_ANDROID, IS_BROWSER, IS_ELECTRON } from "./polyfill";
import styled from "./styled";
import Form from "./form";
import Logout from "./logout";
import { login, signup, checkLogin, resetPassword } from "./auth/auth";
import SP from "sp-client";
import Icon from "./icons";

const Overall = styled.View`
  align-items: center;
  justify-content: flex-start;
  flex-grow: 1;
  height: 100%;
  padding: 15px;
  flex: 1;
  ${!IS_NATIVE && "-webkit-app-region: drag"};
`;

const RestCentered = styled(Form)`
  align-items: stretch;
  max-width: 550px;
  justify-content: flex-start;
  flex-grow: 1;
  width: 100%;
`;

const UserName = styled.TextInput`
  border-bottom-color: black;
  border-bottom-width: ${IS_ANDROID ? "0px" : "1px"};
  border-style: solid;
  height: 55px;
  font-size: 20px;
  width: 100%;
  ${!IS_NATIVE && "-webkit-app-region: no-drag"};
  margin-bottom: 10px;
  padding-left: 5px;
  padding-right: 5px;
`;

const LogoImage = styled.Image`
  height: 100px;
  width: 100%;
  ${IS_NATIVE && "margin-top: 30px"};
`;

const ButtonView = styled.View`
  width: 100%;
  margin-bottom: 15px;
`;

const LoginButton = styled.Button`
  height: 80px;
`;

const ErrorText = styled.Text`
  color: red;
  text-align: center;
`;

let logoSource = require("./streamplace-logo.svg");
if (IS_NATIVE) {
  logoSource = require("./streamplace-logo.png");
}

export default class Login extends React.Component {
  constructor() {
    super();
    this.state = {
      email: "",
      password: "",
      loading: true,
      loggedIn: false,
      error: null
    };
  }
  componentWillMount() {
    checkLogin()
      .then(user => {
        if (user) {
          return this.setState({
            loading: false,
            loggedIn: true
          });
        }
        return this.setState({
          loading: false,
          loggedIn: false
        });
      })
      .catch(err => {
        this.setState({
          loading: false,
          loggedIn: false
        });
      });
  }
  login() {
    this.setState({ error: null });
    return login({
      email: this.state.email,
      password: this.state.password
    })
      .then(() => {
        this.setState({
          loading: !IS_NATIVE,
          loggedIn: true,
          email: "",
          password: ""
        });
      })
      .catch(err => {
        this.setState({
          error: err.description || err.message,
          loading: false
        });
      });
  }

  signup() {
    this.setState({ error: null, loading: true });
    return signup({
      email: this.state.email,
      password: this.state.password
    })
      .then(() => {
        return this.login();
      })
      .catch(err => {
        this.setState({
          error: err.description || err.message,
          loading: false
        });
      });
  }

  resetPassword() {
    this.setState({ error: null, loading: true });
    return resetPassword({
      email: this.state.email
    })
      .then(() => {
        return this.setState({
          loading: false,
          error: "Sent. Check your email."
        });
      })
      .catch(err => {
        this.setState({
          error: err.description || err.message,
          loading: false
        });
      });
  }

  render() {
    if (this.state.loading) {
      return <View />;
    }
    if (this.state.loggedIn) {
      return (
        <Overall>
          <Logout
            onLoggedOut={async () => {
              this.props.onLoggedOut && (await this.props.onLoggedOut());
              this.setState({
                loading: false,
                loggedIn: false
              });
            }}
          />
        </Overall>
      );
    }
    return (
      <Overall>
        <RestCentered>
          <LogoImage
            draggable={IS_ELECTRON ? false : undefined}
            resizeMode={"contain"}
            source={logoSource}
          />
          <UserName
            onChangeText={email =>
              this.setState({
                email
              })
            }
            editable={true}
            value={this.state.email}
            placeholder="email"
            keyboardType="email-address"
            onSubmitEditing={() => {
              if (IS_NATIVE) {
                this.passwordInput.focus();
              } else {
                this.login();
              }
            }}
            returnKeyType="next"
          />
          <UserName
            onChangeText={password =>
              this.setState({
                password
              })
            }
            editable={true}
            secureTextEntry={true}
            onSubmitEditing={() => this.login()}
            value={this.state.password}
            placeholder="password"
            returnKeyType="go"
            innerRef={ref => (this.passwordInput = ref)}
          />
          <ErrorText>{this.state.error || " "}</ErrorText>

          <ButtonView>
            <LoginButton title="Log In" onPress={() => this.login()} />
          </ButtonView>

          <ButtonView>
            <LoginButton title="Create Account" onPress={() => this.signup()} />
          </ButtonView>

          <ButtonView>
            <LoginButton
              title="Forgot Password"
              onPress={() => this.resetPassword()}
            />
          </ButtonView>
        </RestCentered>
      </Overall>
    );
  }
}
