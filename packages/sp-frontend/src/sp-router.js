
import React, { Component } from "react";
import {
  BrowserRouter as Router,
  Route,
  // Link
} from "react-router-dom";
import Home from "./home";

export default class SPRouter extends Component {
  constructor() {
    super();
    this.state = {};
  }

  render () {
    return (
      <Router>
        <Route exact path="/" component={Home} />
      </Router>
    );
  }
}

SPRouter.propTypes = {};
