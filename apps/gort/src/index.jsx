
import React from "react";
import ReactDOM from "react-dom";
import { Router, Route, Link, browserHistory } from "react-router";

import BroadcastIndex from "./components/broadcasts/BroadcastIndex";
import BroadcastDetail from "./components/broadcasts/BroadcastDetail";
import NotFound from "./components/NotFound";

import {} from "./index.html.mustache";
import {} from "./preview.html.mustache";
import {} from "./components/main";
import {} from "font-awesome-webpack";
import {} from "twixtykit/base.scss";

ReactDOM.render((
  <Router history={browserHistory}>
    <Route path="/app" component={BroadcastIndex} />
    <Route path="/app/broadcasts/:broadcastId" component={BroadcastDetail} />
    <Route path="*" component={NotFound} />
  </Router>
), document.querySelector("main"));
