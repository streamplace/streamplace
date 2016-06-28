
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./SceneCreate.scss";

export default class SceneCreate extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  handleClick() {
    let sceneId;
    SK.scenes.create({
      title: "New Scene",
      broadcastId: this.props.broadcastId,
      width: 1920,
      height: 1080,
      regions: [],
    })
    .then((newScene) => {
      sceneId = newScene.id;
      return SK.broadcasts.findOne(this.props.broadcastId);
    })
    .then((broadcast) => {
      if (!broadcast.active) {
        return SK.broadcasts.update(this.props.broadcastId, {activeSceneId: sceneId});
      }
    })
    .catch(::twixty.error);
  }

  render () {
    return (
      <div className={style.ButtonContainer}>
        <button className="outline" onClick={::this.handleClick}>
          <i className="fa fa-plus-square-o"></i>
          New Scene
        </button>
      </div>
    );
  }
}

SceneCreate.propTypes = {
  "broadcastId": React.PropTypes.string.isRequired
};
