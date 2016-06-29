
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
    .on("data", (inputs) => {
      this.setState({inputs});
    })
    .catch(::twixty.error);
  }

  componentDidMount() {
    this.sceneHandle = SK.scenes.watch({id: this.props.sceneId})
    .on("data", ([scene]) => {
      this.setState({scene});
    });
  }

  componentWillUnmount() {
    if (this.inputHandle) {
      this.inputHandle.stop();
    }
    if (this.sceneHandle) {
      this.sceneHandle.stop();
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

  handleDragStart(region, regionIdx, e) {
    if (!this.props.draggable) {
      return;
    }
    e.dataTransfer.setDragImage(this.nullDragTarget, 0, 0);
    const [{width, height}] = this.sceneBox.getClientRects();
    this.boxWidth = width;
    this.boxHeight = height;
    this.startX = e.screenX;
    this.startY = e.screenY;
    this.dragRegion = {...region};
    this.dragRegionIdx = regionIdx;
  }

  handleDrag(e) {
    if (!this.props.draggable) {
      return;
    }
    // There's one weird extra event after we drop
    if (e.screenX === 0 && e.screenY === 0) {
      return;
    }
    const dx = Math.floor(((e.screenX - this.startX) / this.boxWidth) * this.state.scene.width);
    const dy = Math.floor(((e.screenY - this.startY) / this.boxHeight) * this.state.scene.height);
    this.startX = e.screenX;
    this.startY = e.screenY;
    this.dragRegion.x += dx;
    this.dragRegion.y += dy;
    const newRegions = this.state.scene.regions;
    newRegions[this.dragRegionIdx] = this.dragRegion;
    this.setState({
      scene: {
        ...this.state.scene,
        regions: newRegions,
      }
    });
  }

  handleDragEnd(e) {
    SK.scenes.update(this.state.scene.id, {regions: this.state.scene.regions}).catch(::twixty.error);
  }

  handleRef(ref) {
    this.sceneBox = ref;
  }

  renderRegions() {
    if (!this.state.scene) {
      return;
    }
    const scene = this.state.scene;
    return scene.regions.map((input, i) => {
      const inlineStyle = {
        left: `${input.x / scene.width * 100}%`,
        top: `${input.y / scene.height * 100}%`,
        width: `${input.width / scene.width * 100}%`,
        height: `${input.height / scene.height * 100}%`,
        backgroundColor: COLORS[i % COLORS.length],
        zIndex: i,
      };
      if (this.props.draggable) {
        inlineStyle.cursor = "move";
      }
      let label;
      if (this.props.text) {
        label = this.renderLabel(input.inputId);
      }
      const handleDragStart = this.handleDragStart.bind(this, input, i);
      return (
        <div draggable={this.props.draggable} onDragStart={handleDragStart} onDrag={::this.handleDrag} onDragEnd={::this.handleDragEnd} style={inlineStyle} key={i} className={style.Region}>
          {label}
        </div>
      );
    });
  }

  handleNullTargetRef(elem) {
    this.nullDragTarget = elem;
  }

  render () {
    return (
      <div ref={::this.handleRef} className={style.SceneBox}>
        {this.renderRegions()}
        <span style={{position: "absolute"}} ref={::this.handleNullTargetRef} />
      </div>
    );
  }
}

SceneRender.propTypes = {
  "sceneId": React.PropTypes.string.isRequired,
  "inputs": React.PropTypes.object.isRequired,
  "text": React.PropTypes.boolean,
  "draggable": React.PropTypes.boolean,
};
