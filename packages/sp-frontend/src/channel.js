
import React, { Component } from "react";
import SPCanvas from "./sp-canvas";
import SPScene from "./sp-scene";
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
    if (!channel || !channel.activeSceneId) {
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
              <SPScene sceneId={channel.activeSceneId} />
            </SPCanvas>
          </CanvasWrapper>
          <ChannelUsers channelId={channel.id} />
        </FlexContainer>
      </FlexContainer>
    );
  }
}

export default bindComponent(Channel);
