
import React from "react";
import { Broadcast } from "bellamie";
import { Link } from 'react-router';

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
          <Link to={`/broadcasts/${broadcast._id}`}>{broadcast.slug}</Link>
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
