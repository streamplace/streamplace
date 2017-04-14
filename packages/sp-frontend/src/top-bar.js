
import React, { Component } from "react";
import styled from "styled-components";

const Bar = styled.header`
  background-color: white;
  border-bottom: 1px solid #ccc;
  height: 50px;
  display: flex;
  position: relative;
  -webkit-user-select: none;
  -webkit-app-region: drag;
`;

export default class TopBar extends Component {

  render () {
    return (
      <Bar />
    );
  }
}
