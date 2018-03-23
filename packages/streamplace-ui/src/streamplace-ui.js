import React from "react";

import styled, { injectGlobal } from "styled-components";

const IS_NATIVE = typeof navigator.userAgent === "undefined";
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
const componentPkg = IS_NATIVE ? "react-native" : "react-native-web";

if (!IS_NATIVE) {
  injectGlobal`
    html,
    body,
    main {
      height: 100%;
      font-family: "Open Sans", Helvetica, sans-serif !important;
    }
    main {
      display: flex;
    }
  `;

  // mildly hacky check to give styled-components all the RN stuff
  Object.keys(ReactNative).forEach(key => {
    if (!styled[key] && ReactNative[key].prototype instanceof React.Component) {
      styled[key] = styled(ReactNative[key]);
    }
  });
}

const Overall = styled.View`
  align-items: center;
  justify-content: flex-start;
  flex-grow: 1;
  padding: 15px;
  ${!IS_NATIVE && "-webkit-app-region: drag;"};
`;

const RestCentered = styled.View`
  align-items: stretch;
  max-width: 550px;
  justify-content: flex-start;
  flex-grow: 1;
  width: 100%;
`;

const UserName = styled.TextInput`
  border-bottom-color: black;
  border-bottom-width: 1px;
  border-style: solid;
  height: 80px;
  font-size: 30px;
  width: 100%;
  margin-bottom: 10px;
`;

const LogoImage = styled.Image`
  height: 100px;
  width: 100%;
  ${IS_NATIVE && "margin-top: 50px;"};
`;

const LoginButton = styled.Button`
  height: 80px;
`;

let logoSource = require("./streamplace-logo.svg");
if (IS_NATIVE) {
  logoSource = require("./streamplace-logo.png");
}

export default class App extends React.Component {
  constructor() {
    super();
    this.state = {
      value: ""
    };
  }
  render() {
    return (
      <Overall>
        <RestCentered>
          <LogoImage resizeMode={"contain"} source={logoSource} />
          <UserName
            onChangeText={text =>
              this.setState({
                value: text
              })
            }
            editable={true}
            value={this.state.value}
            placeholder="eli@iame.li"
          />
          <UserName
            onChangeText={text =>
              this.setState({
                value: text
              })
            }
            editable={true}
            value={this.state.value}
            placeholder="**********"
          />
          <LoginButton
            title="Log In"
            onPress={() => Linking.openURL("https://stream.place/")}
          />
        </RestCentered>
      </Overall>
    );
  }
}
