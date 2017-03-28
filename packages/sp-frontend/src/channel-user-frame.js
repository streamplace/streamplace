
import React, { Component } from "react";
import {subscribe} from "./sp-binding";
import SPCanvas from "./sp-canvas";
import SPCamera from "./sp-camera";
import {UserFrame, UserTitle, RemoveButton} from "./channel-user-frame.style";

export class ChannelUserFrame extends Component {
  static propTypes = {
    "userId": React.PropTypes.string.isRequired,
    "users": React.PropTypes.array,
    "channelId": React.PropTypes.string.isRequired,
    "channels": React.PropTypes.array,
    "SP": React.PropTypes.object,
  };

  constructor() {
    super();
    this.state = {};
  }

  handleRemove() {
    const {SP} = this.props;
    const [channel] = this.props.channels;
    const [user] = this.props.users;
    // Not loaded yet, we're SOL
    if (!channel || !user) {
      return;
    }
    const users = channel.users.filter(u => u.userId !== user.id);
    SP.channels.update(channel.id, {users}).catch(SP.error);
  }

  render () {
    const [user] = this.props.users;
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

export default subscribe(ChannelUserFrame, (props, SP) => {
  return {
    users: SP.users.watch({id: props.userId}),
    channels: SP.channels.watch({id: props.channelId}),
  };
});
