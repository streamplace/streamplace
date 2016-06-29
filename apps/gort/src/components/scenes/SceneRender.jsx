
import React from "react";
import twixty from "twixtykit";
import _ from "underscore";
import key from "keymaster";

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
    this._doUpdate = _.throttle(::this._doUpdate, 1000);
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
    this.unfocusRegion(); // so we unbind all our keys
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

  focusRegion(regionIdx) {
    this.unfocusRegion();
    this.setState({focusedRegionIdx: regionIdx});
    key("shift+f", ::this.maximize);
    key("up", ::this.nudgeUp);
    key("down", ::this.nudgeDown);
    key("left", ::this.nudgeLeft);
    key("right", ::this.nudgeRight);
  }

  unfocusRegion() {
    this.setState({focusedRegionIdx: null});
    key.unbind("shift+f");
    key.unbind("up");
    key.unbind("down");
    key.unbind("left");
    key.unbind("right");
  }

  updateFocusedRegion(obj) {
    const newScene = this.state.scene;
    Object.assign(newScene.regions[this.state.focusedRegionIdx], obj);
    this.setState({scene: newScene});
    this._doUpdate();
  }

  /**
   * Throttled version of an update so we don't DoS the server with nudges.
   */
  _doUpdate() {
    SK.scenes.update(this.state.scene.id, this.state.scene).catch(::twixty.error);
  }

  // Make one region huge plz
  maximize() {
    this.updateFocusedRegion({
      x: 0,
      y: 0,
      width: this.state.scene.width,
      height: this.state.scene.height,
    });
  }

  nudge(dx, dy) {
    const {x, y} = this.state.scene.regions[this.state.focusedRegionIdx];
    this.updateFocusedRegion({
      x: x + dx,
      y: y + dy,
    });
  }

  nudgeUp() {
    this.nudge(0, -1);
  }

  nudgeDown() {
    this.nudge(0, 1);
  }

  nudgeLeft() {
    this.nudge(-1, 0);
  }

  nudgeRight() {
    this.nudge(1, 0);
  }

  handleDragStart(region, regionIdx, e) {
    if (!this.props.draggable) {
      return;
    }
    if (this.state.focusedRegionIdx !== regionIdx) {
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

  handleRegionClick(region, regionIdx, e) {
    if (!this.props.draggable) {
      return;
    }
    e.stopPropagation();
    this.focusRegion(regionIdx);
  }

  handleClick() {
    this.unfocusRegion();
  }

  handleNullTargetRef(elem) {
    this.nullDragTarget = elem;
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
      if (!this.props.draggable) {
        inlineStyle.cursor = "default";
      }
      let label;
      if (this.props.text) {
        label = this.renderLabel(input.inputId);
      }
      const handleDragStart = this.handleDragStart.bind(this, input, i);
      const handleRegionClick = this.handleRegionClick.bind(this, input, i);
      const isFocused =  i === this.state.focusedRegionIdx;
      const className = isFocused ? style.RegionFocus : style.Region;
      return (
        <div onClick={handleRegionClick} draggable={isFocused} onDragStart={handleDragStart} onDrag={::this.handleDrag} onDragEnd={::this.handleDragEnd} style={inlineStyle} key={i} className={className}>
          {label}
        </div>
      );
    });
  }

  render () {
    const legendClassName = this.props.draggable ? style.Legend : style.LegendHidden;
    return (
      <div>
        <div ref={::this.handleRef} onClick={::this.handleClick} className={style.SceneBox}>
          {this.renderRegions()}
          <span style={{position: "absolute"}} ref={::this.handleNullTargetRef} />
        </div>
        <div className={legendClassName}>
          <em>Hotkeys: shift+f (maximize), arrow keys (nudge)</em>
        </div>
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
