
import React from "react";
import ReactDOM from "react-dom";
import createBrowserHistory from "history/lib/createBrowserHistory";
import { Router, Route, Redirect, Link, useRouterHistory, RouteHandler } from "react-router";

import Home from "./components/Home";
import BroadcastDetail from "./components/broadcasts/BroadcastDetail";
import BroadcastGraphView from "./components/broadcasts/BroadcastGraphView";
import SceneEditor from "./components/scenes/SceneEditor";
import InputDetail from "./components/inputs/InputDetail";
import NotFound from "./components/NotFound";

import logoUrl from "./sk_small.svg";
import style from "./index.scss";

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

class Handler extends React.Component{
  render () {
    return (
      <div className={style.RootContainer}>
        <header className={style.Header}>
          <Link to="/" className={style.backButton}>
            <img src={logoUrl} className={style.Logo} />
          </Link>
        </header>
        {this.props.children}
      </div>
    );
  }
}

Handler.propTypes = {
  "children": React.PropTypes.object
};

ReactDOM.render((
  <Router history={browserHistory}>
    <Route path="" component={Handler}>
      <Route path="/" component={Home} />
      <Route path="/inputs/:inputId" component={InputDetail} />
      <Route path="/broadcasts/:broadcastId/" component={BroadcastDetail}>
        <Route path="scenes" component={SceneEditor} />
        <Route path="graph" component={BroadcastGraphView} />
      </Route>
      <Redirect from="*" to="/" />
    </Route>
  </Router>
), document.querySelector("main"));
