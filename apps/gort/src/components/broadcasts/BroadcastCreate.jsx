
import React from "react";

import SK from "../../SK";
import SKField from "../SKField";

export default class BroadcastCreate extends React.Component{
  constructor(params) {
    super(params);
    this.state = {};
  }

  reset() {
    this.setState({
      newBroadcast: {
        title: "",
        url: "",
        enabled: false
      }
    });
  }

  handleSubmit(e) {
    e.preventDefault();
    SK.broadcasts.create(this.state.newBroadcast);
    this.reset();
  }

  handleChange(newBroadcast) {
    this.setState({newBroadcast});
  }

  render() {
    return (
      <form className="pure-form" onSubmit={this.handleSubmit.bind(this)}>
        <fieldset>
          <em>Add a broadcast</em>

          <SKField type="text" data={this.state.newBroadcast} onChange={this.handleChange.bind(this)} placeholder="Best Broadcast Ever" label="Title" field="title" />

          <button type="submit" className="pure-button pure-button-primary">Create</button>
        </fieldset>
      </form>
    );
  }
}
