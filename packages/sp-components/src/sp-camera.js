import React, { Component } from "react";
import SP from "sp-client";
import { relativeCoords } from "sp-utils";
import * as THREE from "three";
import { getPeer } from "sp-peer-stream";

export default class SPCamera extends Component {
  static propTypes = {
    userId: React.PropTypes.string.isRequired,
    x: React.PropTypes.number.isRequired,
    y: React.PropTypes.number.isRequired,
    width: React.PropTypes.number.isRequired,
    height: React.PropTypes.number.isRequired,
    muted: React.PropTypes.bool
  };

  static contextTypes = {
    scene: React.PropTypes.object.isRequired,
    canvasWidth: React.PropTypes.number.isRequired,
    canvasHeight: React.PropTypes.number.isRequired
  };

  constructor() {
    super();
    this.initStream = this.initStream.bind(this);
  }

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

  componentWillUnmount() {
    this.peer.off("stream", this.initStream);
    this.cleanupThree();
  }

  start() {
    // Retrieve is like "on" but immediately resolves if we already have one.
    this.peer.retrieve("stream", this.initStream);
  }

  initStream(stream) {
    if (this.ref.srcObject === stream) {
      return;
    }
    this.ref.srcObject = stream;
    return new Promise((resolve, reject) => {
      const handler = () => {
        this.ref.removeEventListener("loadedmetadata", handler);
        resolve();
      };
      this.ref.addEventListener("loadedmetadata", handler);
    })
      .then(() => {
        this.initThree(this.props);
      })
      .catch(err => {
        SP.error(err);
      });
  }

  initThree(props) {
    this.cleanupThree();

    const video = this.ref;

    const geometry = new THREE.PlaneGeometry(props.width, props.height);

    const { videoWidth, videoHeight } = video;

    const videoAspect = videoWidth / videoHeight;
    const myAspect = props.width / props.height;

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
    } else if (videoAspect < myAspect) {
      const newHeight = 1 / myAspect * videoWidth;
      const textureAdjustment = newHeight / videoHeight;
      texture.repeat.y = textureAdjustment;
      const cutOffPixels = videoHeight - newHeight;
      texture.offset.y = cutOffPixels / videoHeight / 2;
    }

    const material = new THREE.MeshBasicMaterial({ map: texture });

    this.mesh = new THREE.Mesh(geometry, material);
    const [x, y] = relativeCoords(
      props.x,
      props.y,
      props.width,
      props.height,
      this.context.canvasWidth,
      this.context.canvasHeight
    );
    this.mesh.position.set(x, y, 0);
    this.context.scene.add(this.mesh);
  }

  /**
   * Bad! Should dynamically move around, not reboot the whole dang mesh.
   */
  componentWillReceiveProps(newProps) {
    const { x, y, width, height } = newProps;
    if (
      x !== this.props.x ||
      y !== this.props.y ||
      width !== this.props.width ||
      height !== this.props.height
    ) {
      this.initThree(newProps);
    }
  }

  cleanupThree() {
    if (this.mesh) {
      this.context.scene.remove(this.mesh);
    }
    this.mesh = null;
  }

  render() {
    return (
      <video
        autoPlay
        ref={this.getRef.bind(this)}
        muted={!(this.props.muted === false)}
      />
    );
  }
}
