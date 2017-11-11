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

const Keyframe = Layer;

class App extends Component {
  render() {
    // return <div />;
    return (
      <Layer>
        <Text fontSize={50} top={1000} left={750}>
          chat: https://chat.stream.place
        </Text>
        <Countdown
          top={250}
          left={1000}
          to={new Date("Tuesday November 14, 2017 16:00:00 GMT-0800")}
        />
        <Timeline>
          <Keyframe time={0} left={300} top={875} scaleX={0.5} scaleY={0.5}>
            <Image src={cranky} />
          </Keyframe>
          <Keyframe time={2500} left={1250} />
          <Keyframe time={2700} scaleX={-0.5} />
          <Keyframe time={5200} left={300} />
          <Keyframe time={5400} scaleX={0.5} />
        </Timeline>

        {/* <Layer width={1920} height={1080} left={0} top={0}>
          <Timeline>
            <RandomLayer>
              <Image src={cranky} />
            </RandomLayer>
            <RandomLayer>
              <Image src={cranky} />
            </RandomLayer>
          </Timeline>
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
        </Layer> */}
      </Layer>
    );
  }
}

export default App;
