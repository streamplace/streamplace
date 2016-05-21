
import React from "react";
import twixty from "twixtykit";
import _ from "underscore";

import style from "./SceneRender.scss";
import SK from "../../SK";

const COLORS = [
  "#ff0066",
  "#00ccff",
  "#00ff22",
  "#0000f2",
  "#e2f200",
  "#f200a2",
  "#00f2c2",
  "#005ce6"
];

export default class SceneRender extends React.Component{
  constructor() {
    super();
    this.state = {};
    this.inputHandle = SK.inputs.watch({})
    .then((inputs) => {
      this.setState({inputs});
    })
    .catch(::twixty.error);
  }

  componentWillUnmount() {
    if (this.inputHandle) {
      this.inputHandle.stop();
    }
  }

  renderLabel(inputId) {
    if (!this.state.inputs) {
      return;
    }
    let input = _(this.state.inputs).findWhere({id: inputId});
    if (!input) {
      input = {title: "Error: input not found"};
    }
    return <div className={style.RegionLabel}>{input.title}</div>;
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
        backgroundColor: COLORS[i % COLORS.length],
        zIndex: i,
      };
      return (
        <div style={inlineStyle} key={i} className={style.Region}>
          {this.renderLabel(input.inputId)}
        </div>
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
