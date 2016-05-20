
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import browserHistory from "../../history";
import SceneThumbnailList from "./SceneThumbnailList";
import SceneCreate from "./SceneCreate";
import style from "./SceneEditor.scss";

export default class SceneEditor extends React.Component {
  constructor(params) {
    super(params);
    this.state = {
      activeScene: null
    };
  }

  componentDidMount() {
    const broadcastId = this.props.params.broadcastId;
    this.sceneHandle = SK.broadcasts.watch({broadcastId})
    .on("data", (broadcasts) => {
      this.setState({broadcast: broadcasts[0]});
    })
    .catch((...args) => {
      twixty.error(...args);
    });
  }

  componentWillUnmount() {
    this.sceneHandle.stop();
  }

  handlePick(id) {
    browserHistory.push(`/broadcasts/${this.props.params.broadcastId}/scenes/${id}`);
  }

  render() {
    return (
      <section className={style.TwoPanels}>
        <div className={style.LeftPanel}>
          <SceneThumbnailList broadcastId={this.props.params.broadcastId} onPick={::this.handlePick} />
          <SceneCreate broadcastId={this.props.params.broadcastId} />
        </div>
        <div className={style.RightPanel}>
          {this.props.children}
        </div>
      </section>
    );
  }
}

SceneEditor.propTypes = {
  "broadcastId": React.PropTypes.string.isRequired,
  "params": React.PropTypes.object.isRequired,
  "children": React.PropTypes.object
};
