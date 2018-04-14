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
import SP from "sp-client";
import styled from "./styled";
import { logout } from "./auth/auth";
import { IS_NATIVE } from "./polyfill";

const Overall = styled.View`
  align-items: center;
  justify-content: flex-start;
  flex-grow: 1;
  padding: 15px;
  ${!IS_NATIVE && "-webkit-app-region: drag"};
`;

const LogoutButton = styled.Button`
  height: 80px;
  padding-top: 30px;
`;

export default class Logout extends React.Component {
  async logout() {
    await logout();
    this.props.onLoggedOut && (await this.props.onLoggedOut());
  }

  render() {
    return (
      <Overall>
        <Text>Logged in as {SP.user.id}</Text>
        <LogoutButton onPress={() => this.logout()} title="Log Out" />
      </Overall>
    );
  }
}
