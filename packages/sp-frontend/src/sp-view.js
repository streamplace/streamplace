
import React, { Component } from "react";

export default class SPView extends Component {
  constructor() {
    super();
    this.state = {};
  }

  render () {
    return (
      <div>{this.props.children}</div>
    );
  }
}

SPView.propTypes = {
  "children": React.PropTypes.object,
};
