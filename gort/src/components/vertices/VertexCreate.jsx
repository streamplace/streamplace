
import React from "react";
import style from "./VertexCreate.scss";

export default React.createClass({
  displayName: "VertexCreate",
  getInitialState() {
    return {
      chosen: null
    };
  },
  render() {
    const options = Object.keys(vertexCreators).map((name) => {
      const pick = () => {
        this.setState({chosen: name});
      };
      const className = this.state.chosen === name ? style.VertexCreatorSelected : "";
      return (
        <li key={name}>
          <a className={className} onClick={pick}>{name}</a>
        </li>
      );
    });
    const Chosen = vertexCreators[this.state.chosen] || "br";
    return (
      <section className={style.VertexCreate}>
        <div>
          <h2>New Vertex</h2>
          <ul>
            {options}
          </ul>
        </div>
        <Chosen />
      </section>
    );
  }
});

const vertexCreators = {
  RTMPInputVertex: React.createClass({
    render() {
      return (
        <div>
          <h4>Create Input Vertex</h4>
          <label>
            <span>title</span>
            <input type="text" />
          </label>
        </div>
      );
    }
  }),
  RTMPOutputVertex: React.createClass({
    render() {
      return (
        <div>
          <h4>Create Output Vertex</h4>
          <label>
            <span>RTMP URL</span>
            <input type="text" />
          </label>
        </div>
      );
    }
  })
};
