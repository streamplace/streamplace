
import React, { Component } from "react";
import {bindComponent, watch} from "./sp-binding";
import SPCanvas from "./sp-canvas";
import SPCamera from "./sp-camera";
import {
  UserFrame,
  UserTitle,
  RemoveButton,
  SceneToggleButton,
} from "./channel-user-frame.style";

export class ChannelUserFrame extends Component {
  static propTypes = {
    "userId": React.PropTypes.string.isRequired,
    "user": React.PropTypes.object,
    "channelId": React.PropTypes.string.isRequired,
    "channel": React.PropTypes.object,
    "scene": React.PropTypes.object,
    "SP": React.PropTypes.object,
  };

  static subscribe(props) {
    return {
      user: watch.one("users", {id: props.userId}),
      channel: watch.one("channels", {id: props.channelId}),
      scene: props.channel && watch.one("scenes", {id: props.channel.activeSceneId}),
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

  /**
   * Add the user to the currently-active scene. This will eventually get much, much more
   * complicated, but for now we just use a hardcoded series of values based on the number of
   * people currently in the scene.
   */
  addToScene() {
    const {SP, scene, userId} = this.props;
    SP.scenes.update(scene.id, {
      children: [
        ...scene.children,
        {
          kind: "SPCamera",
          x: 0,
          y: 0,
          width: 1920,
          height: 1080,
          zIndex: 0,
          userId: userId,
        }
      ]
    })
    .catch(SP.error);
  }

  /**
   * Remove this user from the scene. This one is closer to correct compared to addToScene.
   */
  removeFromScene() {
    const {SP, scene, userId} = this.props;
    SP.scenes.update(scene.id, {
      children: scene.children.filter((child) => child.userId !== userId)
    });
  }

  render () {
    const {user} = this.props;
    if (!user) {
      return <div />;
    }
    let sceneToggle;
    if (this.props.scene) {
      const hasUser = !!this.props.scene.children.find(child => {
        return child.kind === "SPCamera" && child.userId === user.id;
      });
      if (hasUser) {
        sceneToggle = (
          <SceneToggleButton onClick={() => this.removeFromScene()}>
            <i className="fa fa-minus" />
          </SceneToggleButton>
        );
      }
      else {
        sceneToggle = (
          <SceneToggleButton onClick={() => this.addToScene()}>
            <i className="fa fa-plus" />
          </SceneToggleButton>
        );
      }
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
        {sceneToggle}
      </UserFrame>
    );
  }
}

export default bindComponent(ChannelUserFrame);
