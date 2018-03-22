import React from "react";
import { Text, View } from "react-native";
import styled from "styled-components";

const Centered = styled(Text)`
  display: flex;
  align-items: center;
  justify-content: center;
`;

export default class App extends React.Component {
  render() {
    return (
      <View>
        <Text>hello hi</Text>
      </View>
    );
  }
}

// const styles = StyleSheet.create({
//   container: {
//     flex: 1,
//     backgroundColor: "#fff",
//     alignItems: "center",
//     justifyContent: "center"
//   }
// });
