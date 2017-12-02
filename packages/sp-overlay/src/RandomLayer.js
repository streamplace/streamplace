import React, { Component } from "react";
import styled from "styled-components";

const getScale = props => {
  if (!props.scale) {
    return `scale(1, 1)`;
  }
  return `scale(${props.scale}, ${props.scale})`;
};

const StyledRandomLayer = styled.div`
  position absolute;
  left: ${() => Math.floor(Math.random() * 90)}%;
  top: ${() => Math.floor(Math.random() * 90)}%;
  ${props => props.width && `width: ${props.width}px;`}
  ${props => props.height && `height: ${props.height}px;`}
  transform: ${props => getScale(props)};
`;

class RandomLayer extends Component {
  render() {
    return (
      <StyledRandomLayer {...this.props}>
        {this.props.children}
      </StyledRandomLayer>
    );
  }
}

export default RandomLayer;
