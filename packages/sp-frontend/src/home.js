
import React, { Component } from "react";
import SPChannel from "./sp-channel";
import SPView from "./sp-view";
import SPCamera from "./sp-camera";
import styled from "styled-components";

const FlexContainer = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
`;

const Centered = styled.div`
  margin: auto;
`;

export default class Home extends Component{
  constructor() {
    super();
    this.state = {};
  }

  render () {
    return (
      <FlexContainer>
        <SPChannel width={1920} height={1080}>
          <SPCamera x={0} y={0} width={960} height={270} userId="8145ebde-cf2d-44e9-8462-92aac7fe0074"></SPCamera>
          <SPCamera x={960} y={0} width={960} height={1080} userId="8145ebde-cf2d-44e9-8462-92aac7fe0074"></SPCamera>
          <SPCamera x={0} y={270} width={960} height={810} userId="8145ebde-cf2d-44e9-8462-92aac7fe0074"></SPCamera>
        </SPChannel>
      </FlexContainer>
    );
  }
}
