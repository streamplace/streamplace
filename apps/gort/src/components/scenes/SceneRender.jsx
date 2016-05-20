
import React from "react";

import style from "./SceneRender.scss";

export default class SceneRender extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  renderRegions() {
    if (!this.props.scene) {
      return;
    }
    const scene = this.props.scene;
    return scene.regions.map((input, i) => {
      const inlineStyle = {
        left: `${input.x / scene.width * 100}%`,
        top: `${input.y / scene.height * 100}%`,
        width: `${input.width / scene.width * 100}%`,
        height: `${input.height / scene.height * 100}%`,
        zIndex: i,
      };
      return (
        <div style={inlineStyle} key={i} className={style.Region} />
      );
    });
  }

  render () {
    return (
      <div className={style.SceneBox}>
        {this.renderRegions()}
      </div>
    );
  }
}

SceneRender.propTypes = {
  "scene": React.PropTypes.object.isRequired,
  "inputs": React.PropTypes.object.isRequired,
};
