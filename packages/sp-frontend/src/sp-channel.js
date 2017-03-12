
import React, { Component } from "react";
import styled from "styled-components";
import * as THREE from "three";

const ChannelContainer = styled.div`
  flex-grow: 1;
  overflow: hidden;
  position: relative;
`;

// Channel contents is all position: none, 'cuz we manually grab stuff and render with WebGL
const ChannelContents = styled.div`
  position: absolute;
  display: none;
`;

const Canvas = styled.canvas`
  background-color: black;
  position: absolute;
  left: 50%;
  top: 50%;
`;

export default class SPChannel extends Component {
  constructor(props) {
    super();
    this.state = {
      scale: 1,
    };
    this.containerProm = new Promise((resolve, reject) => {
      this._resolveContainer = resolve;
    });
    this.canvasProm = new Promise((resolve, reject) => {
      this._resolveCanvas = resolve;
    });
    this.canvasProm.then((canvas) => {
      const interval = setInterval(() => {
        const video = document.getElementById("the-video");
        if (video) {
          clearInterval(interval);
          this.start(canvas, video);
        }
      }, 100);
    });
    Promise.all([this.containerProm, this.canvasProm]).then(([container, canvas]) => {
      this.handleResize(container, canvas);
      this.windowListener = this.handleResize.bind(this, container, canvas);
      window.addEventListener("resize", this.windowListener, {passive: true});
    });
  }

  componentWillUnmount() {
    if (this.windowListner) {
      window.removeEventListener(this.windowListener);
    }
  }

  containerRef(container) {
    this._resolveContainer(container);
  }

  handleResize(container, canvas) {
    const {width, height} = this.props;
    const {width: containerWidth, height: containerHeight} = container.getClientRects()[0];
    const aspect = width / height;
    const containerAspect = containerWidth / containerHeight;
    if (aspect > containerAspect) {
      this.setState({
        scale: containerWidth < width ? containerWidth / width : 1,
      });
    }
    else {
      this.setState({
        scale: containerHeight < height ? containerHeight / height : 1,
      });
    }
  }

  ref(canvas) {
    this._resolveCanvas(canvas);
  }

  start(canvas, video) {
    const scene = new THREE.Scene();

    const camera = new THREE.OrthographicCamera(this.props.width / -2, this.props.width / 2, this.props.height / 2, this.props.height / -2, 1, 1000);
    camera.position.z = 1000;

    const geometry = new THREE.PlaneGeometry(960, 540);

    const texture = new THREE.VideoTexture( video );
    texture.minFilter = THREE.LinearFilter;
    texture.magFilter = THREE.LinearFilter;
    texture.format = THREE.RGBFormat;

    const material = new THREE.MeshBasicMaterial( { map: texture } );

    const mesh = new THREE.Mesh( geometry, material );
    mesh.position.set( -480, 270, 0 );
    scene.add( mesh );

    const renderer = new THREE.WebGLRenderer({
      canvas,
      antialias: false,
      preserveDrawingBuffer: true,
    });

    renderer.setSize( this.props.width, this.props.height );

    const animate = () => {
      requestAnimationFrame( animate );
      renderer.render( scene, camera );
    };

    animate();
  }

  render () {
    const canvasStyle = {
      width: `${this.props.width}px`,
      height: `${this.props.height}px`,
      marginLeft: `-${this.props.width / 2}px`,
      marginTop: `-${this.props.height / 2}px`,
      transform: `scale(${this.state.scale})`,
    };
    return (
      <ChannelContainer innerRef={this.containerRef.bind(this)}>
        <Canvas style={canvasStyle} innerRef={this.ref.bind(this)} />
        <ChannelContents>
          {this.props.children}
        </ChannelContents>
      </ChannelContainer>
    );
  }
}

SPChannel.propTypes = {
  "width": React.PropTypes.number.isRequired,
  "height": React.PropTypes.number.isRequired,
  "children": React.PropTypes.object,
};
