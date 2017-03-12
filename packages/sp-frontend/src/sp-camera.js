
import React, { Component } from "react";
import SP from "sp-client";
import {relativeCoords} from "sp-utils";
import * as THREE from "three";

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

  constructor(props) {
    super(props);
    this.getRef = new Promise((resolve, reject) => {
      this._refResolve = resolve;
    });
  }

  ref(elem) {
    if (!elem) {
      return;
    }
    this._refResolve(elem);
  }

  componentWillMount() {
    let stream;
    let video;
    navigator.mediaDevices.getUserMedia({
      audio: true,
      video: {
        width: {min: 1280},
        height: {min: 720},
      },
    })
    .then((s) => {
      stream = s;
      return this.getRef;
    })
    .then((v) => {
      video = v;
      video.srcObject = stream;
      return new Promise((resolve, reject) => {
        const handler = () => {
          video.removeEventListener("loadedmetadata", handler);
          resolve();
        };
        video.addEventListener("loadedmetadata", handler);
      });
    })
    .then(() => {
      this.initThree(video);
    })
    .catch((err) => {
      SP.error(err);
    });
  }

  initThree(video) {
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

    const mesh = new THREE.Mesh(geometry, material);
    const [x, y] = relativeCoords(this.props.x, this.props.y, this.props.width, this.props.height, this.context.canvasWidth, this.context.canvasHeight);
    mesh.position.set( x, y, 0 );
    this.context.scene.add(mesh);
  }

  getThreeObject() {
    return "this";
  }

  render () {
    return (
      <video autoPlay ref={this.ref.bind(this)} muted />
    );
  }
}
