import React, { Component } from "react";
import Rect from "./Rect";
import logo from "./logo.svg";
import "./App.css";
import Layer from "./Layer";
import Timeline from "./Timeline";
import Text from "./Text";
import Image from "./Image";
import RandomLayer from "./RandomLayer";
import cranky from "./cranky.gif";

class App extends Component {
  render() {
    return (
      <Layer>
        <Text fontSize={50} top={1000} left={750}>
          chat: https://chat.stream.place
        </Text>
        <Layer width={1920} height={1080} left={0} top={0}>
          <Timeline>
            <RandomLayer>
              <Image src={cranky} />
            </RandomLayer>
            <RandomLayer>
              <Image src={cranky} />
            </RandomLayer>
          </Timeline>
        </Layer>
        <Layer top={875} scale={0.5}>
          <Image src={cranky} />
        </Layer>
        <Layer top={0} left={0}>
          <Timeline>
            <Rect backgroundColor="blue" width={700} height={200} />
            <Rect backgroundColor="red" width={300} height={200} top={200} />
            <Rect backgroundColor="green" width={500} height={200} top={400} />
          </Timeline>
        </Layer>
        <Layer top={100} left={1300}>
          <Timeline>
            <Text>hello</Text>
            <Text>how</Text>
            <Text>are</Text>
            <Text>you</Text>
          </Timeline>
        </Layer>
      </Layer>
    );
  }
}

export default App;
