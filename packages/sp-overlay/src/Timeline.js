import React, { Component } from "react";
import styled from "styled-components";

class Timeline extends Component {
  constructor(props) {
    super(props);
    this.state = {
      idx: 0
    };
  }

  componentWillMount() {
    this.interval = setInterval(() => this.timeUpdate(), 250);
  }

  timeUpdate() {
    this.setState({
      idx: (this.state.idx + 1) % this.props.children.length
    });
  }

  render() {
    if (!this.props.children || this.props.children.length === 0) {
      return null;
    }
    if (typeof this.props.children.length !== "number") {
      // It's not an array! Return the one child.
      return this.props.children;
    }
    return this.props.children[this.state.idx];
  }
}

export default Timeline;
