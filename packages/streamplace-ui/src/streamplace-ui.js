import React from "react";
import {
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
import styled, { injectGlobal } from "styled-components";

const IS_NATIVE = typeof navigator.userAgent === "undefined";

if (!IS_NATIVE) {
  injectGlobal`
    html,
    body,
    main {
      height: 100%;
    }
    main {
      display: flex;
    }
  `;
}

if (!styled.View) {
  styled.View = styled(View);
}
if (!styled.TextInput) {
  styled.TextInput = styled(TextInput);
}
if (!styled.Image) {
  styled.Image = styled(Image);
}

const Overall = styled.View`
  align-items: center;
  justify-content: flex-start;
  flex-grow: 1;
  padding: 15px;
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
  height: 200px;
  width: 100%;
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
          <Button
            title="Log In"
            onPress={() => Linking.openURL("https://stream.place/")}
          />
        </RestCentered>
      </Overall>
    );
  }
}
