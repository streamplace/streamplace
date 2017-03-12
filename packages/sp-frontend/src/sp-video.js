
import React, { Component } from "react";

export default class SPVideo extends Component {
  constructor() {
    super();
    this.state = {};
  }

  render () {
    return (
      <video webkit-playsinline playsinline src=""></video>
    );
  }
}

SPVideo.propTypes = {
  "params": React.PropTypes.object.isRequired,
};
