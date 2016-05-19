
import React from "react";

import BroadcastList from "./BroadcastList";
import BroadcastCreate from "./BroadcastCreate";

export default class BroadcastIndex extends React.Component{
  render () {
    return (
      <div className="center-column">
        <h1>stream kitchen</h1>
        <BroadcastList />
              </div>
    );
  }
}
