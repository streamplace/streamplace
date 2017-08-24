import React, { Component } from "react";
import { bindComponent, watch } from "sp-components";
import { StackItem } from "./broadcast-detail.style";

export class BroadcastStackItem extends Component {
  static propTypes = {
    children: React.PropTypes.any
  };
  // static subscribe(props) {
  //   return {
  //     broadcast: watch.one("broadcasts", { id: props.broadcastId }),
  //     inputs: watch("inputs", { userId: props.SP.user.id }),
  //     outputs: watch("outputs", { userId: props.SP.user.id })
  //   };
  // }

  constructor() {
    super();
    this.state = {};
  }

  render() {
    return (
      <StackItem>
        {this.props.children}
      </StackItem>
    );
  }
}

export default bindComponent(BroadcastStackItem);
