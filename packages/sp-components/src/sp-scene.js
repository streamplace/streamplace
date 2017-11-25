import PropTypes from "prop-types";
import React, { Component } from "react";
import { watch, bindComponent } from "./sp-binding";
import SPComponent from "./sp-component";

export class SPScene extends Component {
  static propTypes = {
    sceneId: PropTypes.string.isRequired,
    scene: PropTypes.object
  };

  static subscribe(props) {
    return {
      scene: watch.one("scenes", { id: props.sceneId })
    };
  }

  render() {
    const { scene } = this.props;
    if (!scene) {
      return <div />;
    }
    return (
      <div>
        {scene.children.map(child => <SPComponent key={child.id} {...child} />)}
      </div>
    );
  }
}

export default bindComponent(SPScene);
