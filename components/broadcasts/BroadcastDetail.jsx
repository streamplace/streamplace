
import React from "react";
import { Broadcast, Mote } from "bellamie"
import { Link } from 'react-router';
import MoteGraph from "../motes/MoteGraph";
import MoteDetail from "../motes/MoteDetail";
import Loading from '../Loading';
import styles from "./BroadcastDetail.scss";

export default React.createClass({
  displayName: 'BroadcastDetail',
  getInitialState() {
    return {broadcast: {}}
  },
  componentDidMount() {
    this.broadcastHandle = Broadcast.get({_id: this.props.params.broadcastId}, (broadcasts) => {
      this.setState({broadcast: broadcasts[0]});
    });
    this.moteHandle = Mote.get({broadcastId: this.props.params.broadcastId}, (motes) => {
      this.setState({motes});
    });
  },
  componentWillUnmount() {
    this.broadcastHandle.stop();
  },
  render() {
    if (!this.state.broadcast) {
      return <Loading />
    }
    let selectedMote = null;
    return (
      <section className={styles.verticalPanels}>
        <section className={styles.header}>
          <Link to="/" className={styles.backButton}>
            <i className="fa fa-chevron-left" />
          </Link>
          <h1>Broadcast <em>{this.state.broadcast.slug}</em></h1>
        </section>

        <section className="grow">
          <MoteGraph motes={this.state.motes} />
        </section>

        <section className="grow">
          <MoteDetail mote={selectedMote} />
        </section>
      </section>
    )
  }
})
