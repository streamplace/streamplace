import PropTypes from "prop-types";
import React, { Component } from "react";
import { BrowserRouter as Router, Route } from "react-router-dom";
import Home from "./home";
import icon from "./icon.svg";
import { watch, bindComponent } from "sp-components";
import TopBar from "./top-bar.js";
import BroadcastDetail from "./broadcast-detail";
import Options from "./options";
import {
  AppContainer,
  Sidebar,
  PageContainer,
  ChannelIcon,
  ChannelIconText
} from "./sp-router.style";

export class SPRouter extends Component {
  static propTypes = {
    channels: PropTypes.array,
    onLogout: PropTypes.func.isRequired
  };

  static subscribe(props) {
    return {
      channels: watch("channels", {})
    };
  }

  constructor(props) {
    super();
    this.state = {};
  }

  renderChannelIcon(channel) {
    return (
      <ChannelIcon
        id={`channel-${channel.id}`}
        activeClassName="active"
        icon={channel.icon}
        key={channel.id}
        to={`/${channel.slug}`}
      >
        <ChannelIconText>{channel.slug.slice(0, 1)}</ChannelIconText>
      </ChannelIcon>
    );
  }

  render() {
    return (
      <Router>
        <AppContainer>
          <Sidebar>
            {this.renderChannelIcon({ slug: "", icon: icon, id: "home" })}
            <ChannelIcon
              id="channel-inputs"
              activeClassName="active"
              key="inputs"
              to="/streamplace:inputs"
            >
              <ChannelIconText>
                <i className="fa fa-gear" />
              </ChannelIconText>
            </ChannelIcon>
            <ChannelIcon
              id="channel-options"
              activeClassName="active"
              key="options"
              to="/streamplace:options"
            >
              <ChannelIconText>
                <i className="fa fa-gear" />
              </ChannelIconText>
            </ChannelIcon>
          </Sidebar>
          <PageContainer>
            <TopBar onLogout={this.props.onLogout} />
            <Route exact path="/" component={Home} />
            <Route
              path="/streamplace\:options"
              render={() => <Options onLogout={this.props.onLogout} />}
            />
            <Route
              path="/\:broadcasts/:broadcastId"
              render={({ match }) => (
                <BroadcastDetail broadcastId={match.params.broadcastId} />
              )}
            />
          </PageContainer>
        </AppContainer>
      </Router>
    );
  }
}

// Stuff that we want to be perma-subscribed to goes here. Should be used sparingly.
export default bindComponent(SPRouter);
