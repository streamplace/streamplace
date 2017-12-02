import React, { Component } from "react";
import styled from "styled-components";
import Layer from "./Layer";

class Timeline extends Component {
  constructor(props) {
    super(props);
    this.state = {
      idx: 0
    };
  }

  static defaultProps = {
    tickDuration: 250,
    timingFunction: "unset"
  };

  componentWillMount() {
    this.setState({
      startTime: Date.now()
    });
    this.interval = setInterval(
      () => this.timeUpdate(),
      this.props.tickDuration
    );
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
    let children = this.props.children;
    if (typeof children.length !== "number") {
      // It's not an array! Return the one child.
      children = [this.props.children];
    }

    const lastChild = children[children.length - 1];
    const totalDuration = lastChild.props.time;
    const currentDuration = (Date.now() - this.state.startTime) % totalDuration;

    let props = {};
    for (const child of children) {
      if (currentDuration < child.props.time) {
        break;
      }
      props = {
        ...props,
        ...child.props
      };
    }

    const ret = <Layer {...props} />;

    return ret;
  }
}

export default Timeline;
