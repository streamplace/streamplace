
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./VertexDetail.scss";

export default class VertexDetail extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      vertex: {}
    };
  }
  componentDidMount() {
    const vertexId = this.props.vertexId;
    this.vertexHandle = SK.vertices.watch({id: vertexId})
    .on("data", (vertices) => {
      this.setState({vertex: vertices[0]});
    })
    .catch((err) => {
      twixty.error(err);
    });
  }
  componentWillUnmount() {
    this.vertexHandle.stop();
  }
  render() {
    return (
      <div>{this.state.vertex.id}</div>
    );
  }
}

VertexDetail.propTypes = {
  vertexId: React.PropTypes.string.isRequired
};
