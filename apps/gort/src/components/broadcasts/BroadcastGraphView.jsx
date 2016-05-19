
import React from "react";
import twixty from "twixtykit";

import BroadcastGraph from "./BroadcastGraph";
import VertexCreate from "../vertices/VertexCreate";
import VertexDetail from "../vertices/VertexDetail";
import ArcEdit from "../arcs/ArcEdit";
import Loading from "../Loading";
import style from "./BroadcastDetail.scss";
import SK from "../../SK";

export default class BroadcastGraphView extends React.Component {
  constructor(params) {
    super(params);
    this.state = {
      broadcast: {},
      showNewVertex: false,
      selected: null
    };
  }

  componentDidMount() {
    const broadcastId = this.props.params.broadcastId;
    this.broadcastHandle = SK.broadcasts.watch({id: broadcastId})
    .on("data", (broadcasts) => {
      this.setState({broadcast: broadcasts[0]});
    })
    .catch((...args) => {
      twixty.error(...args);
    });
  }

  componentWillUnmount() {
    this.broadcastHandle.stop();
  }

  handleNewVertexClick() {
    this.setState({showNewVertex: true});
  }

  handlePick(type, id) {
    this.setState({
      showNewVertex: false,
      selected: {type, id},
    });
  }

  clearSelection() {
    this.setState({
      selected: null,
    });
  }

  render() {
    if (!this.state.broadcast.id) {
      return <Loading />;
    }
    let bottomPanel = null;
    if (this.state.showNewVertex) {
      bottomPanel = <VertexCreate broadcastId={this.props.params.broadcastId} />;
    }
    else if (this.state.selected && this.state.selected.type === "vertex") {
      bottomPanel = <VertexDetail vertexId={this.state.selected.id} />;
    }
    else if (this.state.selected && this.state.selected.type === "arc") {
      bottomPanel = <ArcEdit onDelete={this.clearSelection.bind(this)} broadcastId={this.props.params.broadcastId} arcId={this.state.selected.id} />;
    }

    // If we are to be rendering a bottom panel, add its wrapper to it.
    if (bottomPanel !== null) {
      bottomPanel = (
        <section className={style.BottomPanel}>
          {bottomPanel}
        </section>
      );
    }

    return (
      <section className={style.verticalPanels}>
        <section className={style.GraphPanel}>
          <button className={style.newVertexButton} onClick={this.handleNewVertexClick.bind(this)}>
            <i className="fa fa-plus-square" />
          </button>
          <BroadcastGraph onPick={this.handlePick.bind(this)} broadcastId={this.props.params.broadcastId} />
        </section>
        {bottomPanel}
      </section>
    );
  }
}

BroadcastGraphView.propTypes = {
  "params": React.PropTypes.object.isRequired,
};
