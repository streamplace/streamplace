
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./BroadcastGraph.scss";

export default class BroadcastGraph extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      vertices: [],
      arcs: [],
    };
  }
  componentDidMount() {
    const broadcastId = this.props.broadcastId;
    this.vertexHandle = SK.vertices.watch({broadcastId})
    .on("data", (vertices) => {
      this.setState({vertices});
    })
    .catch((...args) => {
      twixty.error(...args);
    });

    this.arcHandle = SK.arcs.watch({broadcastId})
    .on("data", (arcs) => {
      this.setState({arcs});
    })
    .catch((...args) => {
      twixty.error(...args);
    });
  }
  componentWillUnmount() {
    this.arcHandle.stop();
    this.vertexHandle.stop();
  }
  handleClick(thing, id) {
    this.props.onPick(thing, id);
  }
  render() {
    const vertices = this.state.vertices.map((v) => {
      return (
        <div onClick={this.handleClick.bind(this, "vertex", v.id)} className={style.GraphVertex} key={v.id}>
          <em>{v.type}</em>
          <h5>{v.title}</h5>
        </div>
      );
    });
    return (
      <div>
        {vertices}
      </div>
    );
  }
}

BroadcastGraph.propTypes = {
  broadcastId: React.PropTypes.string.isRequired,
  onPick: React.PropTypes.func.isRequired
};
