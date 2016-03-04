
import React from "react";
import dot from "dot-object";

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
    getInitialState() {
      return {
        title: "",
        rtmp: {
          "test": "test",
          url: ""
        }
      };
    },
    handleChange(field, e) {
      this.setState(dot.object({[field]: e.target.value}));
    },
    handleCreate() {

    },
    render() {
      return (
        <div>
          <h4>Create RTMP Input</h4>
          <label className={style.BlockLabel}>
            <span>Title</span>
            <input type="text" value={this.state.title} onChange={this.handleChange.bind(this, "title")} />
          </label>
          <label className={style.BlockLabel}>
            <span>RTMP URL</span>
            <input type="text" value={this.state.rtmp.url} onChange={this.handleChange.bind(this, "rtmp.url")} />
          </label>
          <button onClick={this.handleCreate}>Create</button>
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
