
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
    SK.scenes.create({
      title: "New Scene",
      broadcastId: this.props.broadcastId,
      width: 1920,
      height: 1080,
      regions: [],
    }).catch(::twixty.error);
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
