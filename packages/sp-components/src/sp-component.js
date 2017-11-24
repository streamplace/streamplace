import PropTypes from "prop-types";
import React, { Component } from "react";
import SPCamera from "./sp-camera";

export default class SPComponent extends Component {
  static propTypes = {
    kind: PropTypes.string.isRequired
  };

  render() {
    if (this.props.kind === "SPCamera") {
      return <SPCamera {...this.props} />;
    }
    throw new Error(`Unknown component type: ${this.props.kind}`);
  }
}
