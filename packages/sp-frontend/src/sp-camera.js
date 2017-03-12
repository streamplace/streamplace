
import React, { Component } from "react";
import SP from "sp-client";

export default class SPCamera extends Component {

  ref(elem) {
    this.getStream.then((stream) => {
      elem.srcObject = stream;
    });
  }

  componentWillMount() {
    this.getStream = navigator.mediaDevices.getUserMedia({
      audio: true,
      video: {
        width: {min: 1280},
        height: {min: 720},
      },
    })
    .catch((err) => {
      SP.error(err);
    });
  }

  render () {
    return (
      <video id="the-video" autoPlay ref={this.ref.bind(this)} muted />
    );
  }
}

SPCamera.propTypes = {
  "userId": React.PropTypes.string.isRequired,
};
