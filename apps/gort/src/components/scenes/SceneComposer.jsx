
import React from "react";
import twixty from "twixtykit";

import SceneRender from "./SceneRender";
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
    this.inputHandle = SK.inputs.watch({})
    .on("data", (inputs) => {
      this.setState({inputs});
    })
    .catch(::twixty.error);
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
    this.setState({
      scene: null,
      newTitle: "",
      inputMenuOpen: false
    });
    this.sceneHandle = SK.scenes.watch({id: sceneId})
    .on("data", ([scene]) => {
      this.setState({scene});
      if (!this.editingTitle) {
        this.setState({newTitle: scene.title});
      }
    })
    .on("deleted", () => {
      browserHistory.push(`/broadcasts/${this.props.params.broadcastId}/scenes/`);
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

  handleOpenInputMenu() {
    this.setState({inputMenuOpen: !this.state.inputMenuOpen});
  }

  // Poor man's clickOutside
  handleInputButtonBlur(e) {
    this.setState({inputMenuOpen: false});
  }

  handleAddRegion(id) {
    this.setState({inputMenuOpen: false});
    const regions = [...this.state.scene.regions];
    regions.push({
      inputId: id,
      x: 0,
      y: 0,
      width: 640,
      height: 480,
    });
    SK.scenes.update(this.state.scene.id, {regions})
    .catch(::twixty.error);
  }

  renderInputMenu() {
    if (this.state.inputMenuOpen !== true) {
      return;
    }
    const inputs = this.state.inputs.map((input) => {
      return (
        <li onMouseDown={this.handleAddRegion.bind(this, input.id)} key={input.id}>
          {input.title}
        </li>
      );
    });
    return (
      <div className={style.InputMenu}>
        <ul className={`item-list ${style.InputList}`}>
          {inputs}
        </ul>
      </div>
    );
  }

  renderRegions() {
    const regions = this.state.scene.regions.reverse().map((input) => {
      return <div key={input.inputId}>{input.inputId}</div>;
    });
    return <div>{regions}</div>;
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

        <SceneRender scene={this.state.scene} />

        <div>
          <h5>Regions</h5>
          {this.renderRegions()}
          <div className={style.InputMenuContainer}>
            {this.renderInputMenu()}
            <button onBlur={::this.handleInputButtonBlur} onClick={::this.handleOpenInputMenu}>
              Add Region
            </button>
          </div>
        </div>
      </div>
    );
  }
}

SceneComposer.propTypes = {
  "params": React.PropTypes.object.isRequired,
};
