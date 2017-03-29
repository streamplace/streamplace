
import React, { Component } from "react";
import SPCanvas from "./sp-canvas";
import SPCamera from "./sp-camera";
import {bindComponent, watch} from "./sp-binding";
import {FlexContainer} from "./shared.style";
import ChannelUsers from "./channel-users";
import {TitleBar, ChannelName, CanvasWrapper} from "./channel.style";

export class Channel extends Component {
  static propTypes = {
    channel: React.PropTypes.object,
  };

  static contextTypes = {
    SP: React.PropTypes.object.isRequired,
  };

  static subscribe(props) {
    return {
      channel: watch.one("channels", {slug: props.match.params.slug}),
      scenes: props.channel && watch("scenes", {channelId: props.channel.id}),
    };
  }

  constructor() {
    super();
    this.state = {};
  }

  render () {
    const channel = this.props.channel;
    if (!channel) {
      return <FlexContainer />;
    }
    return (
      <FlexContainer>
        <TitleBar>
          📹 <ChannelName>{channel.slug}</ChannelName>
        </TitleBar>
        <FlexContainer>
          <CanvasWrapper>
            <SPCanvas width={1920} height={1080}>
              <SPCamera x={0} y={0} width={960} height={270} userId="8145ebde-cf2d-44e9-8462-92aac7fe0074"></SPCamera>
              <SPCamera x={960} y={0} width={960} height={1080} userId="12006157-fa7e-4262-8152-abda9acae2f6"></SPCamera>
              <SPCamera x={0} y={270} width={960} height={810} userId="8145ebde-cf2d-44e9-8462-92aac7fe0074"></SPCamera>
            </SPCanvas>
          </CanvasWrapper>
          <ChannelUsers channelId={channel.id} />
        </FlexContainer>
      </FlexContainer>
    );
  }
}

export default bindComponent(Channel);
