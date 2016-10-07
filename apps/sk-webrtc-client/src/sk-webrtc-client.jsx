
import React from "react";
import {} from "twixtykit/base.scss";
import style from "./sk-webrtc-client.scss";
import VideoBox from "./video-box";

export default class SKWebRtcClient extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  render () {
    return (
      <div className={style.ChatContainer}>
        <VideoBox local />
      </div>
    );
  }
}

SKWebRtcClient.propTypes = {};
