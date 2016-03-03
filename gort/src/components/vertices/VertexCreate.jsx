
import React from "react";
import style from "./VertexCreate.scss";

export default React.createClass({
  displayName: "VertexCreate",
  getInitialState() {
    return {
      chosen: <MoteCreateInput />
    }
  },
  switchToInput() {
    this.setState({chosen: <MoteCreateInput />});
  },
  switchToOutput() {
    this.setState({chosen: <MoteCreateOutput />});
  },
  render() {
    return (
      <section className={style.moteCreate}>
        <div>
          <h2>New Mote</h2>
          <a onClick={this.switchToInput}>Input</a>
          <a onClick={this.switchToOutput}>Output</a>
        </div>
        {this.state.chosen}
      </section>
    )
  }
})

const MoteCreateInput = React.createClass({
  render() {
    return (
      <div>
        <h4>Create Input Mote</h4>
        <label>
          <span>title</span>
          <input type="text" />
        </label>
      </div>
    )
  }
});


const MoteCreateOutput = React.createClass({
  render() {
    return (
      <div>
        <h4>Create Output Mote</h4>
        <label>
          <span>RTMP URL</span>
          <input type="text" />
        </label>
      </div>
    )
  }
});
