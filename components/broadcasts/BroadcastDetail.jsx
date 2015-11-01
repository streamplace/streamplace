
import React from "react";
import { Broadcast } from "bellamie"
import { Link } from 'react-router';
import Loading from '../Loading';

export default React.createClass({
  displayName: 'BroadcastDetail',
  getInitialState() {
    return {broadcast: {}}
  },
  componentDidMount() {
    this.broadcastHandle = Broadcast.get({_id: this.props.params.broadcastId}, (broadcasts) => {
      this.setState({broadcast: broadcasts[0]});
    });
  },
  componentWillUnmount() {
    this.broadcastHandle.stop();
  },
  render() {
    if (!this.state.broadcast) {
      return <Loading />
    }
    return (
      <div>
        <Link to="/">Back to list of broadcasts</Link>
        <h1>Broadcast <em>{this.state.broadcast.slug}</em></h1>
      </div>
    )
  }
})
