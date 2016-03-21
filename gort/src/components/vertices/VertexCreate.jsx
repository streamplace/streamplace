
import React from "react";
import dot from "dot-object";
import Twixty from "twixtykit";

import ArcCreate from "../arcs/ArcCreate";
import SK from "../../SK";
import style from "./VertexCreate.scss";

export default class VertexCreate extends React.Component {
  constructor(params) {
    super(params);
    this.state = {
      chosen: null
    };
  }

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
        <Chosen broadcastId={this.props.broadcastId} />
      </section>
    );
  }
}

VertexCreate.propTypes = {
  broadcastId: React.PropTypes.string.isRequired
};

const RTMPInputDefaultState = {
  title: "",
  params: {
    rtmp: {
      url: ""
    },
  },
  inputs: {},
  outputs: {
    default: {},
  },
  broadcastId: ""
};

class RTMPInputVertex extends React.Component {
  constructor(props) {
    super(props);
    this.state = {...RTMPInputDefaultState};
  }

  handleChange(field, e) {
    this.setState(dot.object({[field]: e.target.value}));
  }

  handleCreate() {
    SK.vertices.create({
      broadcastId: this.props.broadcastId,
      title: this.state.title,
      type: "RTMPInput",
      inputs: {},
      outputs: {
        default: {},
      },
      params: {
        rtmp: {
          url: this.state.params.rtmp.url,
        },
      }
    })
    .then((vertex) => {
      Twixty.info(`Created vertex ${vertex.id}`);
    })
    .catch((err) => {
      Twixty.error(err);
    });
    this.setState({...RTMPInputDefaultState} );
  }

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
          <input type="text" value={this.state.params.rtmp.url} onChange={this.handleChange.bind(this, "params.rtmp.url")} />
        </label>
        <label className={style.BlockLabel}>
          <span>Broadcast ID</span>
          <input type="text" disabled value={this.props.broadcastId} onChange={this.handleChange.bind(this, "broadcastId")} />
        </label>
        <button onClick={this.handleCreate.bind(this)}>Create</button>
      </div>
    );
  }
}

RTMPInputVertex.propTypes = {
  broadcastId: React.PropTypes.string.isRequired
};

const RTMPOutputDefaultState = {
  title: "",
  params: {
    rtmp: {
      url: ""
    },
  },
  inputs: {},
  outputs: {},
  broadcastId: ""
};

class RTMPOutputVertex extends React.Component {
  constructor(props) {
    super(props);
    this.state = {...RTMPOutputDefaultState};
  }

  handleChange(field, e) {
    this.setState(dot.object({[field]: e.target.value}));
  }

  handleCreate() {
    SK.vertices.create({
      broadcastId: this.props.broadcastId,
      title: this.state.title,
      type: "RTMPOutput",
      inputs: {
        default: {}
      },
      outputs: {},
      params: {
        rtmp: {
          url: this.state.params.rtmp.url,
        },
      }
    })
    .then((vertex) => {
      Twixty.info(`Created vertex ${vertex.id}`);
    })
    .catch((err) => {
      Twixty.error(err);
    });
    this.setState({...RTMPOutputDefaultState} );
  }

  render() {
    return (
      <div>
        <h4>Create RTMP Output</h4>
        <label className={style.BlockLabel}>
          <span>Title</span>
          <input type="text" value={this.state.title} onChange={this.handleChange.bind(this, "title")} />
        </label>
        <label className={style.BlockLabel}>
          <span>RTMP URL</span>
          <input type="text" value={this.state.params.rtmp.url} onChange={this.handleChange.bind(this, "params.rtmp.url")} />
        </label>
        <label className={style.BlockLabel}>
          <span>Broadcast ID</span>
          <input type="text" disabled value={this.props.broadcastId} onChange={this.handleChange.bind(this, "broadcastId")} />
        </label>
        <button onClick={this.handleCreate.bind(this)}>Create</button>
      </div>
    );
  }
}

RTMPOutputVertex.propTypes = {
  broadcastId: React.PropTypes.string.isRequired
};

const vertexCreators = {RTMPInputVertex, RTMPOutputVertex, ArcCreate};
