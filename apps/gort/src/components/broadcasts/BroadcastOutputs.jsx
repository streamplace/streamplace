
import React from "react";
import twixty from "twixtykit";
import _ from "underscore";

import style from "./BroadcastOutputs.scss";
import SK from "../../SK";

export default class BroadcastOutputs extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  componentDidMount() {
    this.outputHandle = SK.outputs.watch({})
    .on("data", (outputs) => {
      this.setState({outputs});
    })
    .catch(::twixty.error);

    this.broadcastHandle = SK.broadcasts.watch({id: this.props.params.broadcastId})
    .on("data", ([broadcast]) => {
      this.setState({broadcast});
    })
    .catch(::twixty.error);
  }

  componentWillUnmount() {
    this.outputHandle.stop();
    this.broadcastHandle.stop();
  }

  handleToggleActivated(outputId, isActive) {
    let outputIds = [...this.state.broadcast.outputIds];
    if (isActive) {
      if (outputIds.indexOf(outputId) !== -1) {
        // We already have this one! No-op.
        return;
      }
      outputIds.push(outputId);
    }
    else {
      outputIds = _(outputIds).without(outputId);
    }
    SK.broadcasts.update(this.props.params.broadcastId, {outputIds}).catch(::twixty.error);
  }

  renderRow(output) {
    const active = _(this.state.broadcast.outputIds).contains(output.id);
    let button;
    if (active) {
      let onClick = this.handleToggleActivated.bind(this, output.id, false);
      button = <button onClick={onClick} className={style.DeactivateButton}>Deactivate</button>;
    }
    else {
      let onClick = this.handleToggleActivated.bind(this, output.id, true);
      button = <button onClick={onClick} className={style.ActivateButton}>Activate</button>;
    }
    return (
      <section className={style.Row}>
        <span className={style.Title}>{output.title}</span>
        {button}
      </section>
    );
  }

  render () {
    if (!this.state.outputs || !this.state.broadcast) {
      return <div />;
    }
    const rows = this.state.outputs.map(::this.renderRow);
    return (
      <section className={style.Container}>
      <div>{rows}</div>
      </section>
    );
  }
}

BroadcastOutputs.propTypes = {
  "params": React.PropTypes.object.isRequired,
};
