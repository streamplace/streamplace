import React, { Component } from "react";
import pad from "left-pad";
import { TimecodeBox } from "./timecode.style.js";

const TIME_BASE = 90000 / 60;
/**
 * We're a timecode that gets periodic syncs and spits out an okay timecode
 */
export default class Timecode extends Component {
  static propTypes = {
    pts: React.PropTypes.number,
    time: React.PropTypes.number
  };

  constructor(props) {
    super(props);
    this.stop = false;
    const millis = Math.floor(props.pts / TIME_BASE) * 10;
    this.state = {
      millis: millis,
      startMillis: millis,
      time: props.time,
      startTime: props.time,
      lastUpdate: Date.now()
    };
    this.drift = 0;
    this.driftOffset = 0;
    this.last = Date.now();
    this.go = this.go.bind(this);
  }

  componentDidMount() {
    this.go();
  }

  go() {
    if (this.stop === true) {
      return;
    }
    this.forceUpdate();
    requestAnimationFrame(this.go);
  }

  componentWillUnmount() {
    this.stop = true;
  }

  componentWillReceiveProps(newProps) {
    const millis = Math.floor(newProps.pts / TIME_BASE) * 10;
    if (newProps.pts < this.props.pts) {
      // We went backward, like a video looping. Just reset.
      this.driftOffset = 0;
      this.drift = 0;
      this.setState({
        millis: millis,
        startMillis: millis,
        time: newProps.time,
        startTime: newProps.time,
        lastUpdate: Date.now()
      });
    } else if (newProps.pts !== this.props.pts) {
      const now = Date.now();
      const oldMillis =
        this.state.millis + (now - this.state.time) + this.driftOffset;
      const newMillis = millis + (now - newProps.time);
      this.drift = newMillis - oldMillis;
      // const lastInterval = now - this.state.lastUpdate;
      // console.log(
      //   `plz correct for ${this.drift}ms drift over ${lastInterval}ms`
      // );
      this.setState({
        millis: millis,
        time: newProps.time,
        lastUpdate: now
      });
    }
  }

  render() {
    if (this.drift > 1) {
      this.drift -= 1;
      this.driftOffset += 1;
    } else if (this.drift < 0) {
      this.drift += 1;
      this.driftOffset -= 1;
    }
    let millis =
      this.state.startMillis +
      (Date.now() - this.state.startTime) +
      this.driftOffset;
    if (millis < 0) {
      millis = 0;
    }
    let seconds = Math.floor(millis / 1000);
    millis -= seconds * 1000;
    let minutes = Math.floor(seconds / 60);
    seconds -= minutes * 60;
    let hours = Math.floor(minutes / 60);
    minutes -= hours * 60;
    return (
      <TimecodeBox>
        {pad(hours, 2, 0)}:{pad(minutes, 2, 0)}:{pad(seconds, 2, 0)}.{pad(
          millis,
          3,
          0
        )}
      </TimecodeBox>
    );
  }
}
