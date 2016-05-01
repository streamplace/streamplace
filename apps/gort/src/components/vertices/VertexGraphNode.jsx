
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./VertexGraphNode.scss";

export default class VertexGraphNode extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      vertex: {}
    };
  }

  componentDidMount() {
    const vertexId = this.props.vertexId;
    this.vertexHandle = SK.vertices.watch({id: vertexId})
    .on("data", ([vertex]) => {
      this.setState({vertex});
    })
    .catch(function(err) {
      twixty.error(err);
    });
  }

  componentWillUnmount() {
    this.vertexHandle.stop();
  }

  handleClick() {
    this.props.onClick && this.props.onClick();
  }

  render() {
    const v = this.state.vertex;
    if (!v.id) {
      return <div/>;
    }
    let className = style.VertexNode;
    if (this.props.selected) {
      className = style.VertexNodeSelected;
    }
    let statusText;
    if (v.status === "ACTIVE") {
      statusText = <p>{v.timemark}</p>;
    }
    else {
      statusText = <p>{v.status}</p>;
    }
    return (
      <div onClick={this.handleClick.bind(this)} className={className}>
        <em>{v.type}</em>
        <h5>{v.title}</h5>
        {statusText}
      </div>
    );
  }
}

VertexGraphNode.propTypes = {
  vertexId: React.PropTypes.string.isRequired,
  selected: React.PropTypes.bool,
  onClick: React.PropTypes.func,
};
