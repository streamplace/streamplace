
import React from "react";
import SK from "../../SK";

export default React.createClass({
  getInitialState() {
    return {slug: ""};
  },
  handleSubmit(e) {
    e.preventDefault();
    SK.broadcasts.create({slug: this.state.slug});
    this.setState({slug: ""});
  },
  handleChange(e) {
    this.setState({slug: e.target.value})
  },
  render() {
    return (
      <form className="pure-form" onSubmit={this.handleSubmit}>
        <fieldset>
          <legend>Add a broadcast</legend>

          <input type="text" value={this.state.slug} onChange={this.handleChange} placeholder="broadcast-slug" />

          <button type="submit" className="pure-button pure-button-primary">Create</button>
        </fieldset>
      </form>
    )
  },
});
