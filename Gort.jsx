
import React from "react";

import {} from "./gort.scss"
import BroadcastList from "./BroadcastList";
import BroadcastCreate from "./BroadcastCreate";

export default React.createClass({
  displayName: 'Gort',
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
