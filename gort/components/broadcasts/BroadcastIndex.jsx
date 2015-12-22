
import React from "react";

import BroadcastList from "./BroadcastList";
import BroadcastCreate from "./BroadcastCreate";

export default React.createClass({
  displayName: 'BroadcastIndex',
  render () {
    return (
      <div className="center-column">
        <h1>streamprov studio</h1>
        <BroadcastList />
        <BroadcastCreate />
      </div>
    )
  }
})
