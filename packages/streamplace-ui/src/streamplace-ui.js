import React from "react";
import { Text, View, StyleSheet } from "react-native";
import styled, { injectGlobal } from "styled-components";

if (typeof navigator.userAgent !== "undefined") {
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

// const Centered = (styled.View || styled(View))`
//   display: flex;
//   align-items: center;
//   justify-content: center;
//   height: 100%;
// `;

export default class App extends React.Component {
  render() {
    return (
      <View style={styles.container}>
        <Text>{`${typeof navigator.userAgent === "undefined"}`}</Text>
        <Text>hello hi woo</Text>
        <Text>hello hi woo</Text>
        <Text>hello hi woo</Text>
      </View>
    );
  }
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#fff",
    alignItems: "center",
    justifyContent: "center"
  }
});
