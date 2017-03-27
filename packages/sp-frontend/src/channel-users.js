
import React, { Component } from "react";
import {UserBar, JoinButton} from "./channel-users.style";
import {subscribe} from "./sp-binding";
import SP from "sp-client";
import ChannelUserFrame from "./channel-user-frame";

export class ChannelUsers extends Component {
  static propTypes = {
    "channelId": React.PropTypes.string.isRequired,
    "channels": React.PropTypes.array,
  };

  constructor() {
    super();
    this.state = {};
  }

  handleJoin() {
    const [channel] = this.props.channels;
    SP.channels.update(this.props.channelId, {
      users: [
        ...channel.users,
        {
          userId: SP.user.id,
          muted: false,
        }
      ]
    })
    .catch((err) => {
      SP.error(err);
    });
  }

  amInChannel() {
    const [channel] = this.props.channels;
    return !!channel.users.find(u => u.userId === SP.user.id);
  }

  render () {
    const [channel] = this.props.channels;
    if (!channel) {
      return <UserBar />;
    }
    return (
      <UserBar>
        {channel.users.map(u => <ChannelUserFrame key={u.userId} userId={u.userId} channelId={this.props.channelId}>{u.userId}</ChannelUserFrame>)}
        {this.amInChannel() ? "" : <JoinButton onClick={() => this.handleJoin()}>Join Channel</JoinButton>}
      </UserBar>
    );
  }
}

export default subscribe(ChannelUsers, (props, SP) => {
  return {
    channels: SP.channels.watch({id: props.channelId}),
  };
});
