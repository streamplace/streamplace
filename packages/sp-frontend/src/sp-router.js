
import React, { Component } from "react";
import {
  BrowserRouter as Router,
  Route,
  // Link
} from "react-router-dom";
import Home from "./home";
import icon from "./icon.svg";
import {subscribe} from "./sp-binding";
import styled from "styled-components";

const AppContainer = styled.div`
  height: 100%;
  display: flex;
  flex-direction: row;
`;

const Sidebar = styled.header`
  background-color: #333;
`;

const ChannelIcon = styled.img`
  opacity: 0.8;
  margin: 0.2em;
  width: 3em;
`;

export class SPRouter extends Component {
  constructor(props) {
    super();
    this.state = {};
  }

  render () {
    return (
      <AppContainer>
        <Sidebar>
          <ChannelIcon src={icon} />
        </Sidebar>
        <Router>
          <Route exact path="/" component={Home} />
        </Router>
      </AppContainer>
    );
  }
}

// Stuff that we want to be perma-subscribed to goes here. Should be used sparingly.
export default subscribe(SPRouter, (props, SP) => {
  return {
    users: SP.users.watch({id: SP.user.id}),
  }
});
