import React, { Component } from "react";
import Rect from "./Rect";
import logo from "./logo.svg";
import "./App.css";
import Layer from "./Layer";
import Timeline from "./Timeline";
import Text from "./Text";
import Image from "./Image";
import RandomLayer from "./RandomLayer";
import Countdown from "./Countdown";
import cranky from "./cranky.gif";
import styled from "styled-components";
import "./FiraCode/stylesheet.css";

const Keyframe = Layer;

const Centered = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
`;

class App extends Component {
  render() {
    // return <div />;
    return <Centered />;
  }
}

export default App;
