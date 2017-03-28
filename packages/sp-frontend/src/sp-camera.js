
import React, { Component } from "react";
import SP from "sp-client";
import {relativeCoords} from "sp-utils";
import * as THREE from "three";
import {getPeer} from "sp-peer-stream";

export default class SPCamera extends Component {
  static propTypes = {
    userId: React.PropTypes.string.isRequired,
    x: React.PropTypes.number.isRequired,
    y: React.PropTypes.number.isRequired,
    width: React.PropTypes.number.isRequired,
    height: React.PropTypes.number.isRequired,
  };

  static contextTypes = {
    scene: React.PropTypes.object.isRequired,
    canvasWidth: React.PropTypes.number.isRequired,
    canvasHeight: React.PropTypes.number.isRequired,
  };

  getRef(elem) {
    if (!elem) {
      return;
    }
    this.ref = elem;
    this.start();
  }

  componentWillMount() {
    this.peer = getPeer(this.props.userId);
  }

  start() {
    // Retrieve is like "on" but immediately resolves if we already have one.
    this.peer.retrieve("stream", (stream) => {
      this.ref.srcObject = stream;
      return new Promise((resolve, reject) => {
        const handler = () => {
          this.ref.removeEventListener("loadedmetadata", handler);
          resolve();
        };
        this.ref.addEventListener("loadedmetadata", handler);
      })
      .then(() => {
        this.initThree(this.ref);
      })
      .catch((err) => {
        SP.error(err);
      });
    });
  }

  initThree(video) {
    if (this.mesh) {
      this.context.scene.remove(this.mesh);
    }
    const geometry = new THREE.PlaneGeometry(this.props.width, this.props.height);

    const {videoWidth, videoHeight} = video;

    const videoAspect = videoWidth / videoHeight;
    const myAspect = this.props.width / this.props.height;

    const texture = new THREE.VideoTexture(video);
    texture.mapping = THREE.CubeReflectionMapping;
    texture.minFilter = THREE.LinearFilter;
    texture.magFilter = THREE.LinearFilter;
    texture.format = THREE.RGBFormat;

    if (videoAspect > myAspect) {
      const newWidth = myAspect * videoHeight;
      const textureAdjustment = newWidth / videoWidth;
      texture.repeat.x = textureAdjustment;
      const cutOffPixels = videoWidth - newWidth;
      texture.offset.x = cutOffPixels / videoWidth / 2;
    }
    else if (videoAspect < myAspect) {
      const newHeight = (1 / myAspect) * videoWidth;
      const textureAdjustment = newHeight / videoHeight;
      texture.repeat.y = textureAdjustment;
      const cutOffPixels = videoHeight - newHeight;
      texture.offset.y = cutOffPixels / videoHeight / 2;
    }

    const material = new THREE.MeshBasicMaterial({ map: texture });

    this.mesh = new THREE.Mesh(geometry, material);
    const [x, y] = relativeCoords(this.props.x, this.props.y, this.props.width, this.props.height, this.context.canvasWidth, this.context.canvasHeight);
    this.mesh.position.set( x, y, 0 );
    this.context.scene.add(this.mesh);
  }

  render () {
    return (
      <video autoPlay ref={this.getRef.bind(this)} muted />
    );
  }
}
