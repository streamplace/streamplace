
import React from "react";
import dot from "dot-object";
import Twixty from "twixtykit";

import SK from "../../../SK";
import style from "../VertexCreate.scss";

export default class BaseCreator extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      vertex: this.getDefaultVertex(props)
    };
  }

  getDefaultVertex(props) {
    return {
      kind: "vertex",
      title: "",
      params: {},
      inputs: [],
      outputs: [],
      broadcastId: props.broadcastId
    };
  }

  handleCreate() {
    this.setState({vertex: this.getDefaultVertex(this.props)});
    return SK.vertices.create(this.state.vertex)
    .then((vertex) => {
      Twixty.info(`Created vertex ${vertex.id}`);
    })
    .catch((err) => {
      Twixty.error(err);
    });
  }

  handleChange(field, e) {
    const newVertex = {...this.state.vertex};
    dot.set(field, e.target.value, newVertex);
    this.setState({vertex: newVertex});
    return newVertex;
  }

  /**
   * Shortcut for returning a bound handleChange for a field
   */
  setField(fieldName) {
    return this.handleChange.bind(this, fieldName);
  }

  getFields(v) {
    return [(
      <label key="title" className={style.BlockLabel}>
        <span>Title</span>
        <input type="text" value={v.title} onChange={this.setField("title")} />
      </label>
    )];
  }

  render() {
    const v = this.state.vertex;
    return (
      <div>
        <h2>Create {v.type}</h2>
        {this.getFields(v)}
        <button onClick={this.handleCreate.bind(this)}>Create</button>
      </div>
    );
  }
}

export class BaseDetail extends React.Component {

}
