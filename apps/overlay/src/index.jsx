
import React from "react";
import ReactDOM from "react-dom";
import { Router, Route, Link, browserHistory } from "react-router";

import {} from "./index.html.mustache";
import {} from "./index.scss";
import NotFound from "./NotFound";
import InputOverlay from "./InputOverlay";

ReactDOM.render((
  <Router history={browserHistory}>
    <Route path="/:overlayKey" component={InputOverlay} />
    <Route path="*" component={NotFound} />
  </Router>
), document.querySelector("main"));
