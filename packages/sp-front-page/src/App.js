import React, { Component } from "react";
import Countdown from "sp-overlay/dist/Countdown";
import styled, { injectGlobal } from "styled-components";
import YouTube from "react-youtube";

injectGlobal`
  html,
  body,
  #root {
    height: 100%;
  }

  #BigVideo {
    position: absolute;
    width: 100%;
    height: 100%;
  }
`;

const Centered = styled.div`
  display: flex;
  height: 100%;
  align-items: center;
  justify-content: center;
  flex-direction: column;
`;

const VideoOuter = styled.div`
  width: 100%;
  padding-bottom: 56.25%;
  opacity: 0.2;
`;

const YOUTUBE_THRESHOLD = 60 * 10 * 1000; // 10 mins

// https://www.youtube.com/embed/live_stream?channel=UC_0VBEHybbwCoaJHsUmQZEg
class App extends Component {
  constructor(props) {
    super(props);
    this.state = { nextTime: new Date("November 17, 2017 16:00:00 GMT-0800") };
  }

  componentDidMount() {
    this.interval = setInterval(() => {
      this.setState({ now: Date.now() });
    }, 500);
  }

  render() {
    if (this.state.nextTime.getTime() - Date.now() < YOUTUBE_THRESHOLD) {
      const youtubeOpts = {
        playerVars: {
          autoplay: 1,
          modestbranding: 1,
          channel: "UC_0VBEHybbwCoaJHsUmQZEg"
        }
      };
      return (
        <VideoOuter>
          <YouTube id="BigVideo" videoId="live_stream" opts={youtubeOpts} />
        </VideoOuter>
      );
    }

    return (
      <Centered>
        <img
          style={{ position: "absolute", top: 0, left: 0, border: 0 }}
          src="https://cdn.rawgit.com/tholman/github-corners/0a2c2767/svg/github-corner-left.svg"
          alt="Fork me on GitHub"
          data-canonical-src="https://s3.amazonaws.com/github/ribbons/forkme_left_white_ffffff.png"
        />
        <Countdown to={this.state.nextTime} />
      </Centered>
    );
  }
}

export default App;
