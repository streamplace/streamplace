import React, { Component } from "react";
import { bindComponent, watch } from "sp-components";
import CreateMyChannel from "./create-my-channel";
import { FlexContainer, NoChannel, ChannelSelect } from "./home.style";

export class Home extends Component {
  static propTypes = {
    channels: React.PropTypes.array,
    ready: React.PropTypes.bool
  };

  static subscribe(props) {
    return {
      channels: watch("channels", { userId: props.SP.user.id })
    };
  }

  constructor() {
    super();
    this.state = {};
  }

  text() {
    if (!this.props.channels) {
      return <p>Loading...</p>;
    }
    if (this.props.channels.length < 1) {
      return <CreateMyChannel />;
    }
    return <NoChannel>no channel selected</NoChannel>;
  }

  render() {
    return (
      <FlexContainer>
        <ChannelSelect>
          {this.text()}
        </ChannelSelect>
      </FlexContainer>
    );
  }
}

export default bindComponent(Home);
