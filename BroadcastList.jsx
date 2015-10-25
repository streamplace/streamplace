
import React from "react";
import { Broadcast } from "./model"

window.Broadcast = Broadcast;

export default React.createClass({
  displayName: 'BroadcastList',
  getInitialState: function() {
    return {broadcasts: []}
  },
  componentDidMount: function() {
   Broadcast.get({}, (broadcasts) => {
      this.setState({broadcasts});
    });
  },
  removeBroadcast: function(id) {
    Broadcast.remove(id);
  },
  render: function() {
    const broadcastNodes = this.state.broadcasts.map((broadcast) => {
      return (
        <li key={broadcast._id}>
          {broadcast.slug} (<a onClick={this.removeBroadcast.bind(null, broadcast._id)}>{broadcast._id}</a>)
        </li>
      )
    });
    return (
      <ul>
        {broadcastNodes}
      </ul>
    )
  }
})
