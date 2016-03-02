
import React from "react";
import SK from "../../SK";
import { Link } from 'react-router';

export default React.createClass({
  displayName: 'BroadcastList',
  getInitialState() {
    return {broadcasts: []}
  },
  componentDidMount() {
    this.broadcastHandle = SK.broadcasts.watch({})
    .then((broadcasts) => {
      this.setState({broadcasts});
    })
    .catch((err) => {
      console.error(err);
    })
    .on("created", (broadcasts) => {
      this.setState({broadcasts});
    })
    .on("updated", (broadcasts) => {
      this.setState({broadcasts});
    })
    .on("deleted", (broadcasts) => {
      this.setState({broadcasts});
    });
  },
  componentWillUnmount() {
    this.broadcastHandle.stop();
  },
  removeBroadcast(id) {
    SK.broadcasts.delete(id);
  },
  render() {
    const broadcastNodes = this.state.broadcasts.map((broadcast) => {
      return (
        <li key={broadcast.id}>
          <Link to={`/broadcasts/${broadcast.id}`}>{broadcast.id}</Link>
          <button className="pure-button button-error"
          onClick={this.removeBroadcast.bind(null, broadcast.id)}>
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
