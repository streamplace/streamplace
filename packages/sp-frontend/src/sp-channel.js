
import React, { Component } from "react";
import styled from "styled-components";
import * as THREE from "three";

const ChannelContainer = styled.div`

`;

// Channel contents is all position: none, 'cuz we manually grab stuff and render with WebGL
const ChannelContents = styled.div`
  position: absolute;
  display: none;
`;

const Canvas = styled.canvas`
  width: 1920px;
  height: 1080px;
  background-color: black;
`;

export default class SPChannel extends Component {
  constructor() {
    super();
    this.state = {};
  }

  ref(canvas) {
    const interval = setInterval(() => {
      const video = document.getElementById("the-video");
      if (video) {
        clearInterval(interval);
        this.start(canvas, video);
      }
    }, 100);
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
    });

    renderer.setSize( this.props.width, this.props.height );

    const animate = () => {
      requestAnimationFrame( animate );
      renderer.render( scene, camera );
    };

    animate();
  }

  render () {
    return (
      <ChannelContainer>
        <Canvas innerRef={this.ref.bind(this)} />
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
