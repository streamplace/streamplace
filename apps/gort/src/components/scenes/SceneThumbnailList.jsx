
import React from "react";
import twixty from "twixtykit";

import Loading from "../Loading";
import SK from "../../SK";
import SceneThumbnail from "./SceneThumbnail";
import style from "./SceneThumbnailList.scss";

export default class SceneThumbnailList extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  componentDidMount() {
    const broadcastId = this.props.broadcastId;
    let filter = {};
    if (broadcastId) {
      filter = {broadcastId};
      this.broadcastHandle = SK.broadcasts.watch({id: broadcastId})
      .on("data", ([broadcast]) => {
        this.setState({broadcast});
      })
      .catch(::twixty.error);
    }
    this.sceneHandle = SK.scenes.watch(filter)
    .on("data", (scenes) => {
      scenes = scenes.sort((a, b) => {
        if (a.title > b.title) {
          return 1;
        }
        return -1;
      });
      this.setState({scenes});
    })
    .catch((err) => {
      twixty.error(err);
    });
  }

  componentWillUnmount() {
    this.sceneHandle.stop();
    if (this.broadcastHandle) {
      this.broadcastHandle.stop();
    }
  }

  handleClick(id) {
    this.props.onPick(id);
  }

  render () {
    if (this.state.scenes === undefined) {
      return <Loading />;
    }
    const scenes = this.state.scenes.map((scene) => {
      const isLive = this.state.broadcast && this.state.broadcast.activeSceneId === scene.id;
      const isSelected = this.props.selectedScene === scene.id;
      let className;
      if (isLive && isSelected) {
        className = style.ThumbnailLiveSelected;
      }
      else if (isLive) {
        className = style.ThumbnailLive;
      }
      else if (isSelected) {
        className = style.ThumbnailSelected;
      }
      else {
        className = style.Thumbnail;
      }
      return (
        <li className={className} onClick={this.handleClick.bind(this, scene.id)}>
          <SceneThumbnail key={scene.id} sceneId={scene.id} />
        </li>
      );
    });
    return (
      <div>
        <ul className="item-list">
          {scenes}
        </ul>
      </div>
    );
  }
}

SceneThumbnailList.propTypes = {
  "broadcastId": React.PropTypes.string,
  "selectedScene": React.PropTypes.string,
  "onPick": React.PropTypes.func.isRequired,
};
