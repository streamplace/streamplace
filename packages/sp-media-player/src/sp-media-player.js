import React from "react";
import ReactDOM from "react-dom";
import "videojs-contrib-dash";
import "dashjs";
import videojs from "video.js";
import "video.js/dist/video-js.css";
import styled, { injectGlobal } from "styled-components";

injectGlobal`
  html,
  body {
    height: 100%;
  }
`;

const Entry = styled.form``;

const Player = styled.div`
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: space-between;
`;

export default class SPMediaPlayer extends React.Component {
  constructor() {
    super();
    this.state = { url: "" };
  }
  componentDidMount() {
    // instantiate Video.js
    this.player = videojs(this.videoNode, this.props, function onPlayerReady() {
      // console.log("onPlayerReady", this);
    });
  }

  // destroy player on unmount
  componentWillUnmount() {
    if (this.player) {
      this.player.dispose();
    }
  }

  onChange(e) {
    this.setState({ url: e.target.value });
  }

  onSubmit(e) {
    e.preventDefault();
    this.player.src(this.state.url);
    this.setState({ url: "" });
    this.player.play();
  }

  // wrap the player in a div with a `data-vjs-player` attribute
  // so videojs won't create additional wrapper in the DOM
  // see https://github.com/videojs/video.js/pull/3856
  render() {
    return (
      <Player>
        <div data-vjs-player>
          <video ref={node => (this.videoNode = node)} className="video-js" />
        </div>
        <Entry onSubmit={e => this.onSubmit(e)}>
          <input value={this.state.url} onChange={e => this.onChange(e)} />
        </Entry>
      </Player>
    );
  }
}

document.addEventListener("DOMContentLoaded", () => {
  for (const node of document.querySelectorAll("Streamplace")) {
    ReactDOM.render(<SPMediaPlayer />, node);
  }
});
