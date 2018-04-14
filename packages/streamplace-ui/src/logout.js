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
import { logout, getProfile } from "./auth/auth";
import { IS_NATIVE } from "./polyfill";

const Overall = styled.View`
  align-items: center;
  justify-content: flex-start;
  flex-grow: 1;
  padding: 15px;
  flex: 1;
  align-items: center;
  justify-content: center;
  ${!IS_NATIVE && "-webkit-app-region: drag"};
`;

const Centered = styled.View``;

const LogoutButtonContainer = styled.View`
  margin-top: 30px;
`;

const LogoutButton = styled.Button`
  height: 80px;
  padding-top: 30px;
  margin-top: 30px;
`;

const Paragraph = styled.View`
  margin-bottom: 15px;
`;

let MESSAGE = `
  Thanks for trying Streamplace! It doesn't do very many things yet.

  Send feedback to eli@stream.place.
`;

const message = () => {
  return MESSAGE.trim()
    .split("\n")
    .map(str => str.trim())
    .filter(str => str)
    .map((line, i) => (
      <Paragraph key={i}>
        <Text>{line}</Text>
      </Paragraph>
    ));
};

export default class Logout extends React.Component {
  constructor() {
    super();
    this.state = {
      loading: true
    };
  }
  async logout() {
    await logout();
    this.props.onLoggedOut && (await this.props.onLoggedOut());
  }

  async componentDidMount() {
    try {
      const profile = await getProfile();
      this.setState({
        profile: profile,
        loading: false
      });
    } catch (e) {
      this.setState({
        profile: null,
        loading: false
      });
    }
  }

  render() {
    if (this.state.loading) {
      return <View />;
    }
    if (!this.state.profile) {
      return (
        <Overall>
          <Centered>
            <LogoutButtonContainer>
              <LogoutButton onPress={() => this.logout()} title="Log Out" />
            </LogoutButtonContainer>
          </Centered>
        </Overall>
      );
    }
    return (
      <Overall>
        <Centered>
          <Paragraph>
            <Text>Logged in as {this.state.profile.name}.</Text>
          </Paragraph>
          {message()}
          <LogoutButtonContainer>
            <LogoutButton onPress={() => this.logout()} title="Log Out" />
          </LogoutButtonContainer>
        </Centered>
      </Overall>
    );
  }
}
