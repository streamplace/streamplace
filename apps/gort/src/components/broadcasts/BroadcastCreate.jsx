
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import SKField from "../SKField";

export default class BroadcastCreate extends React.Component{
  constructor(params) {
    super(params);
    this.state = {newBroadcast: this.initialBroadcast()};
  }

  componentWillMount() {
    this.reset();
  }

  initialBroadcast() {
    return {
      title: "",
      enabled: false,
      outputIds: [],
    };
  }

  reset() {
    this.setState({
      newBroadcast: this.initialBroadcast()
    });
  }

  handleSubmit(e) {
    e.preventDefault();
    SK.broadcasts.create(this.state.newBroadcast).catch(::twixty.error);
    this.reset();
  }

  handleChange(newFields) {
    const newBroadcast = {...this.state.newBroadcast, ...newFields};
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
