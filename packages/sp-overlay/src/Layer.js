import React, { Component } from "react";
import styled from "styled-components";

const getScale = props => {
  if (!props.scale) {
    return `scale(1, 1)`;
  }
  return `scale(${props.scale}, ${props.scale})`;
};

const StyledLayer = styled.div`
  position absolute;
  left: ${props => props.left || 0}px;
  top: ${props => props.top || 0}px;
  ${props => props.width && `width: ${props.width}px;`}
  ${props => props.height && `height: ${props.height}px;`}
  transform: ${props => getScale(props)};
`;

class Layer extends Component {
  render() {
    return (
      <StyledLayer {...this.props}>
        {this.props.children}
      </StyledLayer>
    );
  }
}

export default Layer;
