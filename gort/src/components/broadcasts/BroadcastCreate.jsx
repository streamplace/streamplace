
import React from "react";
import SK from "../../SK";

export default React.createClass({
  getInitialState() {
    return {title: ""};
  },
  handleSubmit(e) {
    e.preventDefault();
    SK.broadcasts.create({title: this.state.title});
    this.setState({title: ""});
  },
  handleChange(e) {
    this.setState({title: e.target.value})
  },
  render() {
    return (
      <form className="pure-form" onSubmit={this.handleSubmit}>
        <fieldset>
          <legend>Add a broadcast</legend>

          <input type="text" value={this.state.title} onChange={this.handleChange} placeholder="Best Broadcast Ever" />

          <button type="submit" className="pure-button pure-button-primary">Create</button>
        </fieldset>
      </form>
    )
  },
});
