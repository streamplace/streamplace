
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
  render: function() {
    const broadcastNodes = this.state.broadcasts.map((broadcast) => {
      return (
        <li key={broadcast._id}>
          {broadcast.slug} ({broadcast._id})
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
