
import React from "react";
import { Broadcast, Mote } from "bellamie"
import { Link } from 'react-router';
import MoteGraph from "../motes/MoteGraph";
import MoteDetail from "../motes/MoteDetail";
import MoteCreate from "../motes/MoteCreate";
import Loading from '../Loading';
import styles from "./BroadcastDetail.scss";

export default React.createClass({
  displayName: 'BroadcastDetail',
  getInitialState() {
    return {
      broadcast: {},
      showNewMote: false,
      selectedMote: null,
    }
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

  handleNewMoteClick() {
    this.setState({showNewMote: true});
  },

  handleCloseBottomPanelClick() {
    this.setState({
      showNewMote: false,
      selectedMote: null,
    });
  },

  render() {
    if (!this.state.broadcast) {
      return <Loading />
    }
    let bottomPanel;
    if (this.state.showNewMote) {
      bottomPanel = <MoteCreate />
    }
    else {
      bottomPanel = <MoteDetail mote={this.state.selectedMote} />
    }
    let closeBottomPanel;
    if (this.state.showNewMote || this.state.selectedMote !== null) {
      closeBottomPanel = (
        <a className={styles.closeBottomPanel} onClick={this.handleCloseBottomPanelClick}>
          <i className="fa fa-times" />
        </a>
      )
    }
    return (
      <section className={styles.verticalPanels}>
        <section className={styles.header}>
          <Link to="/" className={styles.backButton}>
            <i className="fa fa-chevron-left" />
          </Link>
          <h1>Broadcast <em>{this.state.broadcast.slug}</em></h1>
          <button className={styles.newMoteButton} onClick={this.handleNewMoteClick}>
            <i className="fa fa-plus-square" />
          </button>
        </section>

        <section className="grow">
          <MoteGraph motes={this.state.motes} />
        </section>

        <section className={styles.bottomPanel}>
          {closeBottomPanel}
          {bottomPanel}
        </section>
      </section>
    )
  }
})
