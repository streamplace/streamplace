import React, { Component } from "react";
import "./App.css";
import styled, { injectGlobal } from "styled-components";
import "./FiraCode/stylesheet.css";
import { BrowserRouter as Router, Route, Switch } from "react-router-dom";
import Countdown from "./Countdown";

// const Keyframe = Layer;

injectGlobal`
  html,
  body,
  #root {
    height: 100%;
    background-color: transparent;
    overflow: hidden;
  }

  body {
    background-color: black;
    color: white;
  }
`;

const Centered = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
`;

class App extends Component {
  render() {
    // return <div />;
    return (
      <Centered>
        <Router>
          <Switch>
            <Route
              path={"/countdown/:to"}
              component={props => (
                <Countdown to={new Date(props.match.params.to)} />
              )}
            />
            <Route component={props => <div>404</div>} />
          </Switch>
        </Router>
      </Centered>
    );
  }
}

export default App;
