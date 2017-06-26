import React, { Component } from "react";
import { SPCanvas, bindComponent, watch, SPScene } from "sp-components";
import { FlexContainer } from "./shared.style";
import ChannelUsers from "./channel-users";
import {
  TitleBar,
  ChannelName,
  CanvasWrapper,
  GoLiveButton
} from "./channel.style";

export class Channel extends Component {
  static propTypes = {
    channel: React.PropTypes.object,
    broadcasts: React.PropTypes.array,
    SP: React.PropTypes.object.isRequired
  };

  static subscribe(props) {
    return {
      channel: watch.one("channels", { slug: props.match.params.slug }),
      scenes: props.channel && watch("scenes", { channelId: props.channel.id }),
      broadcasts:
        props.channel &&
          watch("broadcasts", {
            channelId: props.channel.id,
            stopTime: null
          })
    };
  }

  constructor() {
    super();
    this.state = {};
    this.isAdding = false;
  }

  componentWillUnmount() {
    this.isAdding = true;
    const { channel, SP } = this.props;
    if (!channel) {
      return;
    }
    SP.channels
      .update(channel.id, {
        users: channel.users.filter(u => u.userId !== SP.user.id)
      })
      .catch(err => {
        SP.error(err);
      });
  }

  goLive() {
    const { channel, SP } = this.props;
    SP.broadcasts
      .create({
        channelId: channel.id,
        startTime: new Date()
      })
      .catch(err => {
        SP.error(err);
      });
  }

  stopLive() {
    const { broadcasts, SP } = this.props;
    // You really shouldn't be able to have more than one broadcast, but w/e
    const proms = broadcasts.map(broadcast => {
      return SP.broadcasts.update(broadcast.id, { stopTime: new Date() });
    });
    Promise.all(proms).catch(err => {
      SP.error(err);
    });
  }

  render() {
    const { channel, broadcasts, SP } = this.props;
    if (!channel || !channel.activeSceneId || !broadcasts) {
      return <FlexContainer />;
    }

    // For now... if you're not in a channel, auto-join yourself.
    if (!this.isAdding && !channel.users.find(u => u.userId === SP.user.id)) {
      this.isAdding = true;
      SP.channels
        .update(channel.id, {
          users: [
            ...channel.users,
            {
              userId: SP.user.id,
              muted: false
            }
          ]
        })
        .then(() => {
          this.isAdding = false;
        })
        .catch(err => {
          this.isAdding = false;
        });
    }

    const broadcastActive = !(broadcasts && broadcasts.length === 0);
    let goLive;
    if (broadcastActive) {
      goLive = (
        <GoLiveButton active onClick={() => this.stopLive()}>STOP</GoLiveButton>
      );
    } else {
      goLive = (
        <GoLiveButton onClick={() => this.goLive()}>GO LIVE</GoLiveButton>
      );
    }

    return (
      <FlexContainer>
        <TitleBar active={broadcastActive}>
          <div>
            <ChannelName>{channel.slug}</ChannelName>
          </div>
          {broadcastActive && <div>LIVE</div>}
          {goLive}
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
