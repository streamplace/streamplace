
import React, { Component } from "react";

/**
 * Think of this like Redux's <Provider>. It represents a connection to a Streamplace API server,
 * and provides an API client in context that other stuff can use for subscriptions and data
 * binding later.
 */
export default class Streamplace extends Component {
  static propTypes = {
    SP: React.PropTypes.object,
    "children": React.PropTypes.oneOfType([
      React.PropTypes.arrayOf(React.PropTypes.node),
      React.PropTypes.node
    ]),
  };

  static childContextTypes = {
    SP: React.PropTypes.object,
  };

  getChildContext() {
    return {
      SP: this.props.SP,
    };
  }

  render () {
    return this.props.children;
  }
}
