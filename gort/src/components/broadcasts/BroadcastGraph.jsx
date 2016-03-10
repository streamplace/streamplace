
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import ArcGraphNode from "../arcs/ArcGraphNode";
import style from "./BroadcastGraph.scss";

export default class BroadcastGraph extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      vertices: [],
      arcs: [],
      selected: {
        type: null,
        id: null,
      },
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
  handleClick(type, id) {
    this.setState({selected: {type, id}});
    this.props.onPick(type, id);
  }
  handleBackgroundClick(e) {
    // If they clicked the background itself, deselect everything
    if (e.target === e.currentTarget) {
      this.setState({
        selected: {type: null, id: null}
      });
      this.props.onPick(null, null);
    }
  }
  render() {
    const vertices = this.state.vertices.map((v) => {
      let className = style.GraphVertex;
      if (this.state.selected.type === "vertex" && this.state.selected.id === v.id) {
        className = style.GraphVertexSelected;
      }
      return (
        <div onClick={this.handleClick.bind(this, "vertex", v.id)} className={className} key={v.id}>
          <em>{v.type}</em>
          <h5>{v.title}</h5>
          <p>{v.timemark}</p>
        </div>
      );
    });
    const arcs = this.state.arcs.map((arc) => {
      const selected = this.state.selected.type === "arc" && this.state.selected.id === arc.id;
      const onClick = this.handleClick.bind(this, "arc", arc.id);
      return <ArcGraphNode onClick={onClick} selected={selected} arcId={arc.id} key={arc.id} />;
    });
    return (
      <div onClick={this.handleBackgroundClick.bind(this)} className={style.BroadcastGraph}>
        {vertices}
        {arcs}
      </div>
    );
  }
}

BroadcastGraph.propTypes = {
  broadcastId: React.PropTypes.string.isRequired,
  onPick: React.PropTypes.func.isRequired
};
