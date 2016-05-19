
import React from "react";
import SK from "../../SK";
import { Link } from "react-router";

export default class BroadcastList extends React.Component {
  constructor(params) {
    super(params);
    this.state = {broadcasts: []};
  }

  componentDidMount() {
    this.broadcastHandle = SK.broadcasts.watch({})
    .on("data", (broadcasts) => {
      this.setState({broadcasts});
    });
  }

  componentWillUnmount() {
    this.broadcastHandle.stop();
  }

  removeBroadcast(id) {
    SK.broadcasts.delete(id);
  }

  render() {
    const broadcastNodes = this.state.broadcasts.map((broadcast) => {
      return (
        <li key={broadcast.id}>
          <Link to={`/broadcasts/${broadcast.id}`}>{broadcast.title}</Link>
          <button className="danger item-list-delete"
          onClick={this.removeBroadcast.bind(null, broadcast.id)}>
            delete
          </button>
        </li>
      );
    });
    return (
      <ul className="item-list">
        {broadcastNodes}
      </ul>
    );
  }
}
