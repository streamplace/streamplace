
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./ArcCreate.scss";
import ArcDetail from "./ArcDetail";

export default class ArcEdit extends React.Component {
  constructor(props) {
    super(props);
    this.displayName = "ArcEdit";
    this.state = {
      deleting: false,
    };
  }

  handleChange(arc) {
    this.setState({arc});
  }

  handleSave() {
    const updateArc = {...this.state.arc};
    updateArc.broadcastId = this.props.broadcastId;
    SK.arcs.update(this.props.arcId, updateArc)
    .then((arc) => {
      twixty.info(`Updated arc ${arc.id}`);
    })
    .catch((err) => {
      twixty.error(err);
    });
  }

  handleDelete() {
    const arcId = this.props.arcId;
    this.setState({deleting: true});
    this.props.onDelete();
    SK.arcs.delete(arcId)
    .then(() => {
      twixty.info(`Deleted arc ${arcId}`);
    })
    .catch((err) => {
      twixty.error(err);
    });
  }

  render() {
    if (this.state.deleting === true) {
      return <br />;
    }
    return (
      <section className={style.ArcCreateContainer}>
        <ArcDetail arcId={this.props.arcId} broadcastId={this.props.broadcastId} onChange={this.handleChange.bind(this)} />
        <div className={style.ArcCreateButtonContainer}>
          <button onClick={this.handleSave.bind(this)}>Save</button>
          <button onClick={this.handleDelete.bind(this)}>Delete</button>
        </div>
      </section>
    );
  }
}

ArcEdit.propTypes = {
  arcId: React.PropTypes.string.isRequired,
  onDelete: React.PropTypes.func.isRequired,
  broadcastId: React.PropTypes.string.isRequired,
};
