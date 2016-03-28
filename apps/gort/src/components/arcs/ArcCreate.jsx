
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./ArcCreate.scss";
import ArcDetail from "./ArcDetail";

export default class ArcCreate extends React.Component {
  constructor(props) {
    super(props);
    this.displayName = "ArcCreate";
    this.state = {};
  }

  handleChange(arc) {
    this.setState({arc});
  }

  handleCreate() {
    const createArc = {...this.state.arc};
    createArc.broadcastId = this.props.broadcastId;
    SK.arcs.create(createArc)
    .then((arc) => {
      twixty.info(`Created arc ${arc.id}`);
    })
    .catch((err) => {
      twixty.error(err);
    });
  }

  render() {
    return (
      <section className={style.ArcCreateContainer}>
        <ArcDetail broadcastId={this.props.broadcastId} create onChange={this.handleChange.bind(this)} />
        <div className={style.ArcCreateButtonContainer}>
          <button onClick={this.handleCreate.bind(this)}>Create</button>
        </div>
      </section>
    );
  }
}

ArcCreate.propTypes = {
  broadcastId: React.PropTypes.string.isRequired
};
