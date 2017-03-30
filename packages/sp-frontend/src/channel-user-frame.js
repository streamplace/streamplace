
import React, { Component } from "react";
import {bindComponent, watch} from "./sp-binding";
import SPCanvas from "./sp-canvas";
import SPCamera from "./sp-camera";
import {UserFrame, UserTitle, RemoveButton} from "./channel-user-frame.style";

export class ChannelUserFrame extends Component {
  static propTypes = {
    "userId": React.PropTypes.string.isRequired,
    "user": React.PropTypes.object,
    "channelId": React.PropTypes.string.isRequired,
    "channel": React.PropTypes.object,
    "SP": React.PropTypes.object,
  };

  static subscribe(props) {
    return {
      user: watch.one("users", {id: props.userId}),
      channel: watch.one("channels", {id: props.channelId}),
    };
  }

  constructor() {
    super();
    this.state = {};
  }

  handleRemove() {
    const {SP, channel, user} = this.props;
    // Not loaded yet, we're SOL
    if (!channel || !user) {
      return;
    }
    const users = channel.users.filter(u => u.userId !== user.id);
    SP.channels.update(channel.id, {users}).catch(SP.error);
  }

  render () {
    const {user} = this.props;
    if (!user) {
      return <div>Loading</div>;
    }
    return (
      <UserFrame>
        <SPCanvas width={500} height={500}>
          <SPCamera x={0} y={0} width={500} height={500} userId={this.props.userId} />
        </SPCanvas>
        <UserTitle>{user.handle}</UserTitle>
        <RemoveButton onClick={() => this.handleRemove()}>
          <i className="fa fa-remove" />
        </RemoveButton>
      </UserFrame>
    );
  }
}

export default bindComponent(ChannelUserFrame);
