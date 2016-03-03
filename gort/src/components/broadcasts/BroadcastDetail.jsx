
import React from "react";
// import { Broadcast, Mote } from "bellamie"
import { Link } from 'react-router';

// import MoteGraph from "../motes/MoteGraph";
// import MoteDetail from "../motes/MoteDetail";
// import MoteCreate from "../motes/MoteCreate";
import VertexCreate from "../vertices/VertexCreate";
import Loading from '../Loading';
import styles from "./BroadcastDetail.scss";
import SK from "../../SK";

export default React.createClass({
  displayName: 'BroadcastDetail',
  getInitialState() {
    return {
      broadcast: {},
      showNewVertex: false,
      // selectedMote: null,
    }
  },

  componentDidMount() {
    const broadcastId = this.props.params.broadcastId;
    this.broadcastHandle = SK.broadcasts.watch({id: broadcastId})
    .on("data", (broadcasts) => {
      this.setState({broadcast: broadcasts[0]});
    })
    .catch((...args) => {
      console.error(args);
    });

    this.vertexHandle = SK.vertices.watch({broadcastId})
    .on("data", (vertices) => {
      this.setState({vertices});
    })
    .catch((...args) => {
      console.error(args);
    });

    this.arcHandle = SK.arcs.watch({broadcastId})
    .on("data", (arcs) => {
      this.setState({arcs});
    })
    .catch((...args) => {
      console.error(args);
    });
  },

  componentWillUnmount() {
    this.broadcastHandle.stop();
    this.arcHandle.stop();
    this.vertexHandle.stop();
  },

  handleNewMoteClick() {
    this.setState({showNewVertex: true});
  },

  handleCloseBottomPanelClick() {
    this.setState({
      showNewVertex: false,
      selectedMote: null,
    });
  },

  render() {
    if (!this.state.broadcast) {
      return <Loading />
    }

    let bottomPanel;
    if (this.state.showNewVertex) {
      bottomPanel = <VertexCreate />
    }
    else {
      // bottomPanel = <MoteDetail mote={this.state.selectedMote} />
    }

    let closeBottomPanel;
    if (this.state.showNewVertex || this.state.selectedMote !== null) {
      closeBottomPanel = (
        <a className={styles.closeBottomPanel} onClick={this.handleCloseBottomPanelClick}>
          <i className="fa fa-times" />
        </a>
      )
    }

        // <section className="grow">
        //   <MoteGraph motes={this.state.motes} />
        // </section>
        //

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

        <section className="grow" />

        <section className={styles.bottomPanel}>
          {closeBottomPanel}
          {bottomPanel}
        </section>
      </section>
    )
  }
})
