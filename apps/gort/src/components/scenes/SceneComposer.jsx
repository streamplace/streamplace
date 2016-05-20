
import React from "react";
import twixty from "twixtykit";

import browserHistory from "../../history";
import Loading from "../Loading";
import style from "./SceneComposer.scss";
import SK from "../../SK";

export default class SceneComposer extends React.Component{
  constructor() {
    super();
    this.state = {
      newTitle: ""
    };
  }

  componentDidMount() {
    this.subscribeScene(this.props.params.sceneId);
  }

  componentWillReceiveProps(props) {
    this.subscribeScene(props.params.sceneId);
  }

  subscribeScene(sceneId) {
    if (this.sceneHandle) {
      this.sceneHandle.stop();
    }
    this.editingTitle = false;
    this.setState({scene: null, newTitle: ""});
    this.sceneHandle = SK.scenes.watch({id: sceneId})
    .on("data", ([scene]) => {
      this.setState({scene});
      if (!this.editingTitle) {
        this.setState({newTitle: scene.title});
      }
    })
    .catch(::twixty.error);
  }

  componentWillUnmount() {
    if (this.sceneHandle) {
      this.sceneHandle.stop();
    }
  }

  handleChangeTitle(e) {
    this.setState({newTitle: e.target.value});
  }

  handleFocusTitle() {
    this.editingTitle = true;
  }

  handleBlurTitle() {
    this.editingTitle = false;
    SK.scenes.update(this.props.params.sceneId, {title: this.state.newTitle})
    .catch(::twixty.error);
  }

  handleDelete() {
    browserHistory.push(`/broadcasts/${this.props.params.broadcastId}/scenes/`);
    SK.scenes.delete(this.props.params.sceneId);
  }

  render() {
    if (!this.state.scene) {
      return <div className={style.Container}><Loading /></div>;
    }
    const scene = this.state.scene;
    return (
      <div className={style.Container}>
        <section className={style.Header}>
          <div>
            <button>Go Live</button>
          </div>
          <h5>
            <input type="text" value={this.state.newTitle} onFocus={::this.handleFocusTitle} onBlur={::this.handleBlurTitle} onChange={::this.handleChangeTitle} />
          </h5>
          <div>
            <button className="danger" onClick={::this.handleDelete}>Delete</button>
          </div>
        </section>
        <div className={style.SceneBox}></div>
      </div>
    );
  }
}

SceneComposer.propTypes = {
  "params": React.PropTypes.object.isRequired,
};
