
import React from "react";

import {} from "./gort.scss"
import BroadcastList from "./BroadcastList";

export default React.createClass({
  displayName: 'Gort',
  render: function() {
    return (
      <div>
        <h1>streamprov studio</h1>
        <BroadcastList />
      </div>
    )
  }
})
