import React, { Component } from "react";
import { bindComponent, watch } from "sp-components";
import CreateMyChannel from "./create-my-channel";
import {
  FlexContainer,
  BigInput,
  NiceForm,
  NiceLabel,
  NiceSubmit
} from "./shared.style";
import { NoChannel, ChannelSelect } from "./home.style";
import { TitleBar, ChannelName } from "./channel.style.js";
import Loading from "./loading";
import { Link } from "react-router-dom";

export class Home extends Component {
  static propTypes = {
    channels: React.PropTypes.array,
    ready: React.PropTypes.bool,
    SP: React.PropTypes.object,
    broadcasts: React.PropTypes.array
  };

  static subscribe(props) {
    return {
      broadcasts: watch("broadcasts", { userId: props.SP.user.id })
    };
  }

  constructor() {
    super();
    this.state = {
      newBroadcastName: "",
      creating: false
    };
  }

  updateName(e) {
    this.setState({
      newBroadcastName: e.target.value
    });
  }

  submitNewBroadcast(e) {
    e.preventDefault();
    this.setState({
      creating: true
    });
    this.props.SP.broadcasts
      .create({
        title: this.state.newBroadcastName
      })
      .then(newBroadcast => {
        this.setState({
          newBroadcastName: "",
          creating: false
        });
      })
      .catch(err => {
        this.props.SP.log(err);
        this.setState({
          creating: false
        });
      });
  }

  renderForm() {
    if (this.state.creating) {
      return <Loading />;
    }
    return (
      <NiceForm onSubmit={e => this.submitNewBroadcast(e)}>
        <NiceLabel>
          <strong>Broadcast Name</strong>
          <BigInput
            onChange={e => this.updateName(e)}
            placeholder="My Broadcast"
          />
        </NiceLabel>
        <NiceSubmit>Create Broadcast</NiceSubmit>
      </NiceForm>
    );
  }

  render() {
    if (!this.props.broadcasts) {
      return <Loading />;
    }
    return (
      <FlexContainer padded>
        <TitleBar>
          <div>
            <ChannelName>Your Broadcasts</ChannelName>
          </div>
        </TitleBar>
        <div>
          <ul>
            {this.props.broadcasts.map(broadcast =>
              <li key={broadcast.id}>
                <Link to={`/:broadcasts/${broadcast.id}`}>
                  {broadcast.title}
                </Link>
              </li>
            )}
          </ul>
          {this.renderForm()}
        </div>
      </FlexContainer>
    );
  }
}

export default bindComponent(Home);
