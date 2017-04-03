
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
    this.isAdding = false;
  }

  componentWillUnmount() {
    this.isAdding = true;
    const {channel, SP} = this.props;
    SP.channels.update(channel.id, {
      users: channel.users.filter(u => u.userId !== SP.user.id)
    })
    .catch((err) => {
      SP.error(err);
    });
  }

  render () {
    const {channel, SP} = this.props;
    if (!channel || !channel.activeSceneId) {
      return <FlexContainer />;
    }

    // For now... if you're not in a channel, auto-join yourself.
    if (!this.isAdding && !channel.users.find(u => u.userId === SP.user.id)) {
      this.isAdding = true;
      SP.channels.update(channel.id, {
        users: [
          ...channel.users,
          {
            userId: SP.user.id,
            muted: false,
          }
        ]
      })
      .then(() => {
        this.isAdding = false;
      })
      .catch((err) => {
        this.isAdding = false;
      });
    }

    return (
      <FlexContainer>
        <TitleBar>
          📹 <ChannelName>{channel.slug}</ChannelName>
        </TitleBar>
        <FlexContainer>
          <CanvasWrapper>
            <SPCanvas className="main" width={1920} height={1080}>
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
