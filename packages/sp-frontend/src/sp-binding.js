
/**
 * Think of this like Redux's "connect". It exports some functions that make it really easy for SP
 * react components to bind themselves to SP API objects.
 *
 * If I were doing everything from scratch, I'd probably just have sp-client use Redux internally;
 * it already uses some really similar methods. That may still be in the cards. But hopefully
 * using these methods here means we won't have to change any of our components when that internal
 * refactor happens.
 */

import React, { Component } from "react";

const noop = () => {
  return {};
};

export function subscribe(BoundComponent, subscriptionFunc = noop) {
  class Binding extends Component {
    static contextTypes = {
      SP: React.PropTypes.object.isRequired,
    };

    constructor() {
      super();
      this.state = {};
    }

    componentWillMount() {
      const handles = subscriptionFunc(this.props, this.context.SP);
      const allPromises = Object.keys(handles).map((key) => {
        this.setState({[key]: []});
        handles[key].on("data", (newData) => {
          this.setState({[key]: newData});
        });
        return handles[key];
      });
      Promise.all(allPromises).then(() => {
        this.setState({ready: true});
      });
      this.setState({handles});
    }

    componentWillUnmount() {
      Object.keys(this.state.handles).forEach((name) => {
        this.state.handles[name].stop();
      });
    }

    render () {
      const combined = {SP: this.context.SP, ...this.state, ...this.props};
      return (
        <BoundComponent {...combined} />
      );
    }
  }
  return Binding;
}
