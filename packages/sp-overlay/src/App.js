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
    return (
      <Centered>
        <a href="https://github.com/streamplace/streamplace">
          <img
            style={{ position: "absolute", top: 0, left: 0, border: 0 }}
            src="https://cdn.rawgit.com/tholman/github-corners/0a2c2767/svg/github-corner-left.svg"
            alt="Fork me on GitHub"
            data-canonical-src="https://s3.amazonaws.com/github/ribbons/forkme_left_white_ffffff.png"
          />
        </a>
        <Countdown
          to={new Date("Tuesday November 14, 2017 16:00:00 GMT-0800")}
        />
      </Centered>
    );
  }
}

export default App;
