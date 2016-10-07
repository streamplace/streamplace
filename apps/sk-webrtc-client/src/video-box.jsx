
import React from "react";
import style from "./video-box.scss";
import adapter from "webrtc-adapter";
import twixty from "twixtykit";

export default class VideoBox extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  componentWillMount() {
    if (this.props.local === true) {
      this.getStream = navigator.mediaDevices.getUserMedia({
        audio: true,
        video: {width: 1920, height: 1080}
      })
      .catch((err) => {
        twixty.error(err);
      });
    }
  }

  ref(elem) {
    if (this.props.local === true) {
      this.getStream.then((stream) => {
        elem.srcObject = stream;
      });
    }
  }

  render () {
    return (
      <div className={style.VideoBox}>
        <video autoPlay ref={::this.ref} className={style.Video} muted={this.props.local} />
      </div>
    );
  }
}

VideoBox.propTypes = {
  "local": React.PropTypes.bool,
};
