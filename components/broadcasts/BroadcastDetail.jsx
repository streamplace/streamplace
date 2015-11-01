
import React from "react";
import { Broadcast } from "bellamie"
import { Link } from 'react-router';
import Loading from '../Loading';

export default React.createClass({
  displayName: 'BroadcastList',
  getInitialState() {
    return {broadcast: {}}
  },
  componentDidMount() {
   Broadcast.get({}, (broadcasts) => {
      this.setState({broadcast: broadcasts[0]});
    });
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
