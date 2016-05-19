
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";

export default class SceneEditor extends React.Component {
  constructor(params) {
    super(params);
    this.state = {};
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

  render() {
    return <div>hi</div>;
  }
}

SceneEditor.propTypes = {
  "broadcastId": React.PropTypes.string.isRequired,
  "params": React.PropTypes.object.isRequired
};
