
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./VertexDetail.scss";

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
    const encodedURL = encodeURI(v.params.rtmp.url);
    const inputs = Object.keys(v.inputs).map((inputName) => {
      const input = v.inputs[inputName];
      return (
        <p key={inputName}>
          Name: {inputName}<br/>
          Type: {input.type}<br/>
          Socket: {input.socket}
        </p>
      );
    });
    const outputs = Object.keys(v.outputs).map((outputName) => {
      const output = v.outputs[outputName];
      return (
        <p key={outputName}>
          Name: {outputName}<br/>
          Type: {output.type}<br/>
          Socket: {output.socket}
        </p>
      );
    });
    return (
      <div>
        <h4>Title: {v.title}</h4>
        <p>id: {v.id}</p>
        <p>RTMP URL: {v.params.rtmp.url}</p>
        <p><strong>Inputs</strong></p>
        {inputs}
        <p><strong>Outputs</strong></p>
        {outputs}
        <button onClick={this.handleDelete.bind(this)}>Delete</button>
        <p>
          <a href={`/preview.html?url=${encodedURL}`} target="_blank">Preview</a>
        </p>
      </div>
    );
  }
}

VertexDetail.propTypes = {
  vertexId: React.PropTypes.string.isRequired
};
