
import React from "react";
import dot from "dot-object";
import Twixty from "twixtykit";

import ArcCreate from "../arcs/ArcCreate";
import SK from "../../SK";
import style from "./VertexCreate.scss";

import Combine2x1Creator from "./creators/Combine2x1Creator";
import Combine2x2Creator from "./creators/Combine2x2Creator";
import RTMPOutputCreator from "./creators/RTMPOutputCreator";
import RTMPInputCreator from "./creators/RTMPInputCreator";
import DelayCreator from "./creators/DelayCreator";
import AudioMixCreator from "./creators/AudioMixCreator";

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

const vertexCreators = {
  RTMPInputCreator,
  RTMPOutputCreator,
  Combine2x1Creator,
  Combine2x2Creator,
  DelayCreator,
  AudioMixCreator,
  ArcCreate
};

VertexCreate.propTypes = {
  broadcastId: React.PropTypes.string.isRequired
};
