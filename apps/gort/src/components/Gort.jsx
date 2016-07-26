
import React from "react";
import { Router, Route, Redirect, Link, RouteHandler } from "react-router";

import browserHistory from "../history";

import SK from "../SK";
import Home from "./Home";
import BroadcastDetail from "./broadcasts/BroadcastDetail";
import BroadcastGraphView from "./broadcasts/BroadcastGraphView";
import BroadcastOutputs from "./broadcasts/BroadcastOutputs";
import SceneEditor from "./scenes/SceneEditor";
import SceneComposer from "./scenes/SceneComposer";
import InputDetail from "./inputs/InputDetail";
import NotFound from "./NotFound";

import logoUrl from "../sk_small.svg";
import style from "./Gort.scss";

import {} from "../preview.html.mustache";
import {} from "font-awesome-webpack";
import {} from "twixtykit/base.scss";

// Globally override our user widget if need be.
let UserWidget;

class Handler extends React.Component{

  userWidget() {
    if (UserWidget) {
      return UserWidget;
    }
  }

  render () {
    let userWidget = this.userWidget();
    if (userWidget) {
      userWidget = <div className={style.UserWidget}>{userWidget}</div>;
    }
    return (
      <div className={style.RootContainer}>
        <header className={style.Header}>
          <section>
            <Link to="/" className={style.backButton}>
              <img src={logoUrl} className={style.Logo} />
            </Link>
            {userWidget}
          </section>
        </header>
        {this.props.children}
      </div>
    );
  }
}

Handler.propTypes = {
  "children": React.PropTypes.object
};

export default class Gort extends React.Component {
  constructor() {
    super();
    this._handleClientError = ::this._handleClientError;
  }

  _handleClientError(err) {
    if (err.code === 401 || err.code === 403) {
      this.props.onLogout && this.props.onLogout();
    }
  }

  componentWillMount() {
    UserWidget = this.props.userWidget;
    SK.connect({
      token: this.props.token
    });
    SK.on("error", this._handleClientError);
  }

  render() {
    return (
      <Router history={browserHistory}>
        <Route path="" component={Handler}>
          <Route path="/" component={Home} />
          <Route path="/inputs/:inputId" component={InputDetail} />
          <Route path="/broadcasts/:broadcastId/" component={BroadcastDetail}>
            <Route path="scenes/" component={SceneEditor}>
              <Route path="" component={NotFound} />
              <Route path=":sceneId" component={SceneComposer}></Route>
            </Route>
            <Route path="graph" component={BroadcastGraphView} />
            <Route path="outputs" component={BroadcastOutputs} />
          </Route>
        </Route>
        <Redirect from="/*" to="/*/" />
      </Router>
    );
  }
}

Gort.propTypes = {
  "userWidget": React.PropTypes.object,
  "token": React.PropTypes.string,
  "onLogout": React.PropTypes.func
};

