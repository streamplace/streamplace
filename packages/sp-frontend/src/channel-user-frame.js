import React, { Component } from "react";
import { SPCanvas, SPCamera, bindComponent, watch } from "sp-components";
import { normalizeRegions } from "./boring-1080p-regions";
import {
  CanvasWrapper,
  UserFrame,
  UserTitle,
  ActionButton,
  ActionBar
} from "./channel-user-frame.style";

export class ChannelUserFrame extends Component {
  static propTypes = {
    userId: React.PropTypes.string.isRequired,
    user: React.PropTypes.object,
    channelId: React.PropTypes.string.isRequired,
    channel: React.PropTypes.object,
    scene: React.PropTypes.object,
    SP: React.PropTypes.object
  };

  static subscribe(props) {
    return {
      user: watch.one("users", { id: props.userId }),
      channel: watch.one("channels", { id: props.channelId }),
      scene:
        props.channel &&
        watch.one("scenes", { id: props.channel.activeSceneId })
    };
  }

  constructor() {
    super();
    this.state = {};
  }

  handleRemove() {
    const { SP, channel, user } = this.props;
    this.removeFromScene();
    // Not loaded yet, we're SOL
    if (!channel || !user) {
      return;
    }
    const users = channel.users.filter(u => u.userId !== user.id);
    SP.channels.update(channel.id, { users }).catch(SP.error);
  }

  /**
   * Add the user to the currently-active scene. This will eventually get much, much more
   * complicated, but for now we just use a hardcoded series of values based on the number of
   * people currently in the scene.
   */
  addToScene() {
    const { SP, scene, userId } = this.props;
    const children = normalizeRegions([
      ...scene.children,
      {
        id: `${Math.round(Math.random() * 100000000)}`, // someday, server-generated plz
        kind: "SPCamera",
        x: 0,
        y: 0,
        width: 1920,
        height: 1080,
        zIndex: 0,
        userId: userId
      }
    ]);
    SP.scenes.update(scene.id, { children }).catch(SP.error);
  }

  mute(status) {
    const { SP, channel, user } = this.props;
    const users = channel.users.map(u => {
      if (u.userId === user.id) {
        return { ...u, muted: status };
      }
      return u;
    });
    SP.channels.update(channel.id, { users }).catch(SP.error);
  }

  /**
   * Remove this user from the scene. This one is closer to correct compared to addToScene.
   */
  removeFromScene() {
    const { SP, scene, userId } = this.props;
    const children = normalizeRegions(
      scene.children.filter(child => child.userId !== userId)
    );
    SP.scenes.update(scene.id, { children }).catch(SP.error);
  }

  renderSceneToggle() {
    const { scene, user } = this.props;
    const hasUser = !!scene.children.find(child => {
      return child.kind === "SPCamera" && child.userId === user.id;
    });
    if (hasUser) {
      return (
        <ActionButton onClick={() => this.removeFromScene()}>
          <i className="fa fa-minus" />
        </ActionButton>
      );
    } else {
      return (
        <ActionButton onClick={() => this.addToScene()}>
          <i className="fa fa-plus" />
        </ActionButton>
      );
    }
  }

  render() {
    const { user, userId, scene, channel, SP } = this.props;
    if (!user || !scene || !channel) {
      return <div />;
    }

    let muted = !!channel.users.find(u => {
      return u.userId === user.id && u.muted === true;
    });

    let mutedButton;
    if (muted) {
      mutedButton = (
        <ActionButton onClick={() => this.mute(false)}>
          <i className="fa fa-microphone-slash" />
        </ActionButton>
      );
    } else {
      mutedButton = (
        <ActionButton onClick={() => this.mute(true)}>
          <i className="fa fa-microphone" />
        </ActionButton>
      );
    }

    // Never let the user hear themselves. We don't want them to get vain.
    if (userId === SP.user.id) {
      muted = true;
    }

    return (
      <UserFrame>
        <CanvasWrapper>
          <SPCanvas width={500} height={500}>
            <SPCamera
              muted={muted}
              x={0}
              y={0}
              width={500}
              height={500}
              userId={this.props.userId}
            />
          </SPCanvas>
        </CanvasWrapper>
        <ActionBar>
          {this.renderSceneToggle()}
          {mutedButton}
          <ActionButton onClick={() => this.handleRemove()}>
            <i className="fa fa-remove" />
          </ActionButton>
        </ActionBar>
        <UserTitle>{user.handle}</UserTitle>
      </UserFrame>
    );
  }
}

export default bindComponent(ChannelUserFrame);
