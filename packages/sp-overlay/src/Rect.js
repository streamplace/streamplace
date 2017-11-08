import React, { Component } from "react";
import styled from "styled-components";

const Rect = styled.div`
  margin: 0;
  background-color: ${props => props.backgroundColor};
  position: absolute;
  width: ${props => props.width}px;
  height: ${props => props.height}px;
  top: ${props => props.top}px;
  left: ${props => props.left}px;
  opacity: 0.5;
`;

export default Rect;
