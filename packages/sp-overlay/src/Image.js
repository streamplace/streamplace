import React, { Component } from "react";
import styled from "styled-components";

const Image = styled.img`
  margin: 0;
  position: absolute;
  width: ${props => props.width}px;
  height: ${props => props.height}px;
  top: ${props => props.top}px;
  left: ${props => props.left}px;
`;

export default Image;
