
import React from "react";
import twixty from "twixtykit";
import _ from "underscore";

import SceneRender from "./SceneRender";
import SceneRegionEditor from "./SceneRegionEditor";
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
    this.broadcastHandle = SK.broadcasts.watch({id: this.props.params.broadcastId})
    .on("data", ([broadcast]) => {
      this.setState({broadcast});
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
    this.inputHandle.stop();
    this.broadcastHandle.stop();
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

  handleGoLive() {
    SK.broadcasts.update(this.state.broadcast.id, {activeSceneId: this.state.scene.id})
    .catch(::twixty.error);
  }

  handleAddRegion(id) {
    this.setState({inputMenuOpen: false});
    const regions = [...this.state.scene.regions];
    regions.push({
      inputId: id,
      x: 0,
      y: 0,
      width: 640,
      height: 360,
    });
    SK.scenes.update(this.state.scene.id, {regions})
    .catch(::twixty.error);
  }

  handleEditRegion(idx, newRegion) {
    const newScene = {...this.state.scene};
    newScene.regions[idx] = newRegion;
    SK.scenes.update(this.state.scene.id, newScene).catch(::twixty.error);
  }

  handleDeleteRegion(idx) {
    const newScene = {...this.state.scene};
    newScene.regions = _(newScene.regions).without(newScene.regions[idx]);
    SK.scenes.update(this.state.scene.id, newScene).catch(::twixty.error);
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
    const regions = this.state.scene.regions.map((region, i) => {
      const handleEdit = this.handleEditRegion.bind(this, i);
      const handleDelete = this.handleDeleteRegion.bind(this, i);
      return <SceneRegionEditor onChange={handleEdit} onDelete={handleDelete} region={region} key={i}/>;
    }).reverse(); // top-to-bottom, please.
    return <div>{regions}</div>;
  }

  renderGoLive() {
    if (this.state.broadcast.activeSceneId === this.state.scene.id) {
      return (
        <div>
          <button disabled>Live!</button>
        </div>
      );
    }
    return (
      <div>
        <button onClick={::this.handleGoLive}>Go Live</button>
      </div>
    );
  }

  render() {
    if (!this.state.scene) {
      return <div className={style.Container}><Loading /></div>;
    }
    const scene = this.state.scene;
    return (
      <div className={style.Container}>
        <section className={style.Header}>
          {this.renderGoLive()}
          <h5>
            <input type="text" value={this.state.newTitle} onFocus={::this.handleFocusTitle} onBlur={::this.handleBlurTitle} onChange={::this.handleChangeTitle} />
          </h5>
          <div>
            <button className="danger" onClick={::this.handleDelete}>Delete</button>
          </div>
        </section>

        <SceneRender sceneId={this.state.scene.id} text={true} draggable={true} />

        <section className={style.RegionsContainer}>
          <h5>Regions</h5>
          {this.renderRegions()}
          <div className={style.InputMenuContainer}>
            {this.renderInputMenu()}
            <button className={`outline ${style.AddRegionButton}`} onBlur={::this.handleInputButtonBlur} onClick={::this.handleOpenInputMenu}>
              <i className="fa fa-plus-square-o" />
              Add Region
            </button>
          </div>
        </section>
      </div>
    );
  }
}

SceneComposer.propTypes = {
  "params": React.PropTypes.object.isRequired,
};
