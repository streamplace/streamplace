
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./VertexDetail.scss";
import VertexPositionEditor from "./VertexPositionEditor";

export default class VertexDetail extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      unloading: true,
      vertex: {
        rtmp: {}
      }
    };
  }
  doSubscribe(vertexId) {
    this.vertexHandle = SK.vertices.watch({id: vertexId})
    .on("data", (vertices) => {
      this.setState({
        unloading: false,
        vertex: vertices[0]
      });
    })
    .catch((err) => {
      twixty.error(err);
    });
  }
  componentWillMount() {
    this.doSubscribe(this.props.vertexId);
  }
  componentWillReceiveProps(nextProps) {
    if (this.vertexHandle) {
      this.vertexHandle.stop();
    }
    this.doSubscribe(nextProps.vertexId);
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
    let previewLink;
    if (v.params.rtmp && v.params.rtmp.url) {
      const encodedURL = encodeURI(v.params.rtmp.url);
      previewLink = (
        <p>
          RTMP URL: {v.params.rtmp.url}<br/>
          <a href={`/preview.html?url=${encodedURL}`} target="_blank">Preview</a>
        </p>
      );
    }
    let positionEditor;
    if (v.params.positions) {
      positionEditor = <VertexPositionEditor vertexId={v.id} positions={v.params.positions} />;
    }
    const inputs = v.inputs.map((input) => {
      const sockets = input.sockets.map((socket) => {
        return <span key={socket.url}>{socket.type}: {socket.url}<br/></span>;
      });
      return (
        <p key={input.name}>
          Name: {input.name}<br/>
          {sockets}
        </p>
      );
    });
    const outputs = v.outputs.map((output) => {
      const sockets = output.sockets.map((socket) => {
        return <span key={socket.url}>{socket.type}: {socket.url}<br/></span>;
      });
      return (
        <p key={output.name}>
          Name: {output.name}<br/>
          {sockets}
        </p>
      );
    });
    return (
      <div>
        <h4>Title: {v.title}</h4>
        <p>id: {v.id}</p>
        {positionEditor}
        <p><strong>Inputs</strong></p>
        {inputs}
        <p><strong>Outputs</strong></p>
        {outputs}
        <button onClick={this.handleDelete.bind(this)}>Delete</button>
        {previewLink}
      </div>
    );
  }
}

VertexDetail.propTypes = {
  vertexId: React.PropTypes.string.isRequired
};
