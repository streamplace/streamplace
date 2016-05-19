
import React from "react";
import ReactDOM from "react-dom";
import createBrowserHistory from "history/lib/createBrowserHistory";
import { Router, Route, Link, useRouterHistory, RouteHandler } from "react-router";

import Home from "./components/Home";
import BroadcastDetail from "./components/broadcasts/BroadcastDetail";
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
      <div>
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
      <Route path="/broadcasts/:broadcastId" component={BroadcastDetail} />
    </Route>
    <Route path="*" component={NotFound} />
  </Router>
), document.querySelector("main"));
