
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./VertexDetail.scss";

export default class VertexDetail extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      unloading: false,
      vertex: {
        rtmp: {}
      }
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
  handleDelete() {
    this.setState({unloading: true});
    SK.vertices.delete(this.props.vertexId)
    .then(() => {
      twixty.info("Deletion successful.");
    })
    .catch((err) => {
      twixty.error(err);
    });
  }
  render() {
    if (this.state.unloading) {
      return <div />;
    }
    const v = this.state.vertex;
    return (
      <div>
        <h4>Title: {v.title}</h4>
        <p>id: {v.id}</p>
        <p>RTMP URL: {v.rtmp.url}</p>
        <button onClick={this.handleDelete.bind(this)}>Delete</button>
      </div>
    );
  }
}

VertexDetail.propTypes = {
  vertexId: React.PropTypes.string.isRequired
};
