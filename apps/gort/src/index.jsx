
import React from "react";
import ReactDOM from "react-dom";
import createBrowserHistory from "history/lib/createBrowserHistory";
import { Router, Route, Link, useRouterHistory } from "react-router";

import BroadcastIndex from "./components/broadcasts/BroadcastIndex";
import BroadcastDetail from "./components/broadcasts/BroadcastDetail";
import NotFound from "./components/NotFound";

import {} from "./index.html.mustache";
import {} from "./preview.html.mustache";
import {} from "./components/main";
import {} from "font-awesome-webpack";
import {} from "twixtykit/base.scss";

if (!window.SK_PARAMS || window.SK_PARAMS.BASE_URL === undefined) {
  throw new Error("Missing required environment variable: BASE_URL");
}

const browserHistory = useRouterHistory(createBrowserHistory)({
  basename: window.SK_PARAMS.BASE_URL,
});

ReactDOM.render((
  <Router history={browserHistory}>
    <Route path="/" component={BroadcastIndex} />
    <Route path="/broadcasts/:broadcastId" component={BroadcastDetail} />
    <Route path="*" component={NotFound} />
  </Router>
), document.querySelector("main"));
