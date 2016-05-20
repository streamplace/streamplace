
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
    }
    this.sceneHandle = SK.scenes.watch(filter)
    .on("data", (scenes) => {
      this.setState({scenes});
    })
    .catch((err) => {
      twixty.error(err);
    });
  }

  componentWillUnmount() {
    this.sceneHandle.stop();
  }

  handleClick(id) {
    this.props.onPick(id);
  }

  render () {
    if (this.state.scenes === undefined) {
      return <Loading />;
    }
    const scenes = this.state.scenes.map((scene) => {
      return (
        <li className={style.ThumbnailPicker} onClick={this.handleClick.bind(this, scene.id)}>
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
  "onPick": React.PropTypes.func.isRequired,
};
