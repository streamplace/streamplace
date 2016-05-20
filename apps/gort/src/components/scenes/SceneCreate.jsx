
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
      broadcastId: this.props.broadcastId
    }).catch(::twixty.error);
  }

  render () {
    return (
      <div className={style.ButtonContainer}>
        <button className="outline" onClick={::this.handleClick}>New Scene</button>
      </div>
    );
  }
}

SceneCreate.propTypes = {
  "broadcastId": React.PropTypes.string.isRequired
};
