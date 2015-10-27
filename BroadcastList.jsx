
import React from "react";
import { Broadcast } from "./model"

window.Broadcast = Broadcast;

export default React.createClass({
  displayName: 'BroadcastList',
  getInitialState() {
    return {broadcasts: []}
  },
  componentDidMount() {
   Broadcast.get({}, (broadcasts) => {
      this.setState({broadcasts});
    });
  },
  removeBroadcast(id) {
    Broadcast.remove(id);
  },
  render() {
    const broadcastNodes = this.state.broadcasts.map((broadcast) => {
      return (
        <li key={broadcast._id}>
          {broadcast.slug} {broadcast._id}
          <button className="pure-button button-error"
          onClick={this.removeBroadcast.bind(null, broadcast._id)}>
            delete
          </button>
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
