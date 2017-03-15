
import React, { Component } from "react";
import {
  BrowserRouter as Router,
  Route,
  NavLink,
} from "react-router-dom";
import Home from "./home";
import icon from "./icon.svg";
import {subscribe} from "./sp-binding";
import styled from "styled-components";
import ChannelRoute from "./channel-route";

const AppContainer = styled.div`
  height: 100%;
  display: flex;
  flex-direction: row;
`;

// Ordinarily for a sidebar I'd use `em`s and the size of the icons for the width, but we want
// this sidebar to line up exactly with Mac OS control icons. So here we are.
const Sidebar = styled.header`
  background-color: #333;
  padding-top: 1em;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  width: 82px;
  -webkit-user-select: none;
  -webkit-app-region: drag;
`;

const PageContainer = styled.div`
  display: flex;
  flex-direction: column;
  flex-grow: 1;
`;

const TopBar = styled.header`
  background-color: white;
  border-bottom: 1px solid #ccc;
  height: 50px;
`;

const oColor = "#cccccc";
const oSize = "3px";

const ChannelIcon = styled(NavLink)`
  width: 3em;
  height: 3em;
  display: block;
  cursor: pointer;
  border-radius: 0.4em;
  margin: 1.2em 0;
  background-color: ${props => props.icon ? "transparent" : "white"};
  color: black;
  overflow: hidden;
  display: flex;
  user-select: none;
  align-items: center;
  background-image: ${props => props.icon ? `url("${props.icon}")` : "none"};
  background-size: contain;
  opacity: 0.5;

  &:hover {
    box-shadow: ${oSize} ${oSize} 0px ${oColor}, -${oSize} -${oSize} 0px ${oColor}, ${oSize} -${oSize} 0px ${oColor}, -${oSize} ${oSize} 0px ${oColor};
  }

  &.active {
    opacity: 1;
  }
`;

const ChannelIconText = styled.span`
  font-size: 2em;
`;

export class SPRouter extends Component {
  static propTypes = {
    channels: React.PropTypes.array,
  };

  constructor(props) {
    super();
    this.state = {};
  }

  renderChannelIcon(channel) {
    return (
      <ChannelIcon activeClassName="active" icon={channel.icon} key={channel.id} to={`/${channel.slug}`}>
        <ChannelIconText>{channel.slug}</ChannelIconText>
      </ChannelIcon>
    );
  }

  render () {
    return (
      <Router>
        <AppContainer>
          <Sidebar>
            {this.renderChannelIcon({slug: "", icon: icon, id: "home"})}
            {this.props.channels.map(c => this.renderChannelIcon(c))}
          </Sidebar>
          <PageContainer>
            <TopBar>
              <i className="fa fa-gear" />
            </TopBar>
            <Route exact path="/" component={Home} />
            <Route path="/:slug" component={ChannelRoute} />
          </PageContainer>
        </AppContainer>
      </Router>
    );
  }
}

// Stuff that we want to be perma-subscribed to goes here. Should be used sparingly.
export default subscribe(SPRouter, (props, SP) => {
  return {
    users: SP.users.watch({id: SP.user.id}),
    channels: SP.channels.watch(),
  };
});
