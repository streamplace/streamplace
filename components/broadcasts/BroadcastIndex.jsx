
import React from "react";

import BroadcastList from "./BroadcastList";
import BroadcastCreate from "./BroadcastCreate";

export default React.createClass({
  displayName: 'BroadcastIndex',
  render () {
    return (
      <div>
        <h1>streamprov studio</h1>
        <BroadcastList />
        <BroadcastCreate />
      </div>
    )
  }
})
