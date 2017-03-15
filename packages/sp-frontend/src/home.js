
import React, { Component } from "react";
import styled from "styled-components";
import {subscribe} from "./sp-binding";
import CreateMyChannel from "./create-my-channel";

const FlexContainer = styled.div`
  width: 100%;
  height: 100%;
  flex-grow: 1;
  display: flex;
`;

const NoChannel = styled.p`
  font-style: oblique;
`;

const ChannelSelect = styled.div`
  margin: auto;
  font-size: 1.4em;
  font-weight: 200;
  opacity: 0.8;
`;

export class Home extends Component {
  static propTypes = {
    channels: React.PropTypes.array,
    ready: React.PropTypes.bool,
  };

  constructor() {
    super();
    this.state = {};
  }

  text() {
    if (!this.props.ready) {
      return <p>Loading...</p>;
    }
    if (this.props.channels.length < 1) {
      return <CreateMyChannel />;
    }
    return <NoChannel>no channel selected</NoChannel>;
  }

  render () {
    return (      <FlexContainer>
        <ChannelSelect>
          {this.text()}
        </ChannelSelect>
      </FlexContainer>
    );
  }
}

export default subscribe(Home, (props, SP) => {
  return {
    channels: SP.channels.watch({userId: SP.user.id}),
  };
});
