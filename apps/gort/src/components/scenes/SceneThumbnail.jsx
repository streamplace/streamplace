
import React from "react";
import twixty from "twixtykit";

import SceneRender from "./SceneRender";
import style from "./SceneThumbnail.scss";
import SK from "../../SK";

export default class SceneThumbnail extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  componentDidMount() {
    const sceneId = this.props.sceneId;
    this.sceneHandle = SK.scenes.watch({id: sceneId})
    .on("data", ([scene]) => {
      this.setState({scene});
    })
    .catch((err) => {
      twixty.error(err);
    });
  }

  componentWillUnmount() {
    this.sceneHandle.stop();
  }

  render () {
    if (!this.state.scene) {
      return <div />;
    }
    return (
      <div className={style.ThumbnailContainer}>
        <em>{this.state.scene.title}</em>
        <SceneRender scene={this.state.scene} />
      </div>
    );
  }
}

SceneThumbnail.propTypes = {
  "sceneId": React.PropTypes.string.isRequired,
};
