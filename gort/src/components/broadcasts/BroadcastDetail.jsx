
import React from "react";
import { Link } from "react-router";
import twixty from "twixtykit";

import BroadcastGraph from "./BroadcastGraph";
import VertexCreate from "../vertices/VertexCreate";
import VertexDetail from "../vertices/VertexDetail";
import Loading from "../Loading";
import styles from "./BroadcastDetail.scss";
import SK from "../../SK";

export default React.createClass({
  displayName: "BroadcastDetail",
  getInitialState() {
    return {
      broadcast: {},
      showNewVertex: false,
      selected: null,
    };
  },

  propTypes: {
    "params": React.PropTypes.object.isRequired
  },

  componentDidMount() {
    const broadcastId = this.props.params.broadcastId;
    this.broadcastHandle = SK.broadcasts.watch({id: broadcastId})
    .on("data", (broadcasts) => {
      this.setState({broadcast: broadcasts[0]});
    })
    .catch((...args) => {
      twixty.error(...args);
    });
  },

  componentWillUnmount() {
    this.broadcastHandle.stop();
  },

  handleNewMoteClick() {
    this.setState({showNewVertex: true});
  },

  handleCloseBottomPanelClick() {
    this.setState({
      showNewVertex: false,
      selected: null,
    });
  },

  handlePick(type, id) {
    this.setState({selected: {type, id}});
  },

  render() {
    if (!this.state.broadcast) {
      return <Loading />;
    }

    let bottomPanel;
    if (this.state.showNewVertex) {
      bottomPanel = <VertexCreate />;
    }
    else if (this.state.selected && this.state.selected.type === "vertex") {
      bottomPanel = <VertexDetail vertexId={this.state.selected.id} />;
    }
    else {
      // bottomPanel = <MoteDetail mote={this.state.selectedMote} />
    }

    let closeBottomPanel;
    if (this.state.showNewVertex || this.state.selected !== null) {
      closeBottomPanel = (
        <a className={styles.closeBottomPanel} onClick={this.handleCloseBottomPanelClick}>
          <i className="fa fa-times" />
        </a>
      );
    }

    return (
      <section className={styles.verticalPanels}>
        <section className={styles.header}>
          <Link to="/" className={styles.backButton}>
            <i className="fa fa-chevron-left" />
          </Link>
          <h1>Broadcast <em>{this.state.broadcast.title}</em></h1>
          <button className={styles.newMoteButton} onClick={this.handleNewMoteClick}>
            <i className="fa fa-plus-square" />
          </button>
        </section>

        <section className="grow">
          <BroadcastGraph onPick={this.handlePick} broadcastId={this.props.params.broadcastId} />
        </section>

        <section className={styles.bottomPanel}>
          {closeBottomPanel}
          {bottomPanel}
        </section>
      </section>
    );
  }
});
