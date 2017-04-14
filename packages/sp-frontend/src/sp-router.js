
import React, { Component } from "react";
import {
  BrowserRouter as Router,
  Route,
} from "react-router-dom";
import Home from "./home";
import icon from "./icon.svg";
import {watch, bindComponent} from "sp-components";
import Channel from "./channel";
import TopBar from "./top-bar.js";
import Options from "./options";
import {
  AppContainer,
  Sidebar,
  PageContainer,
  ChannelIcon,
  ChannelIconText,
} from "./sp-router.style";

export class SPRouter extends Component {
  static propTypes = {
    channels: React.PropTypes.array,
    onLogout: React.PropTypes.func.isRequired,
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
      <ChannelIcon id={`channel-${channel.id}`} activeClassName="active" icon={channel.icon} key={channel.id} to={`/${channel.slug}`}>
        <ChannelIconText>{channel.slug.slice(0, 1)}</ChannelIconText>
      </ChannelIcon>
    );
  }

  render () {
    return (
      <Router>
        <AppContainer>
          <Sidebar>
            {this.renderChannelIcon({slug: "", icon: icon, id: "home"})}
            {this.props.channels && this.props.channels.map(c => this.renderChannelIcon(c))}
            <ChannelIcon id="channel-options" activeClassName="active" key="options" to="/streamplace:options">
              <ChannelIconText>
                <i className="fa fa-gear" />
              </ChannelIconText>
            </ChannelIcon>
          </Sidebar>
          <PageContainer>
            <TopBar onLogout={this.props.onLogout} />
            <Route exact path="/" component={Home} />
            <Route path="/streamplace\:options" render={() => <Options onLogout={this.props.onLogout} />} />
            <Route path="/:slug" component={Channel} />
          </PageContainer>
        </AppContainer>
      </Router>
    );
  }
}

// Stuff that we want to be perma-subscribed to goes here. Should be used sparingly.
export default bindComponent(SPRouter);
