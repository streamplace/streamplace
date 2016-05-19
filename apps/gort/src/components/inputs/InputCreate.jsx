
import React from "react";
import SK from "../../SK";

import SKField from "../SKField";
import tk from "twixtykit/styles.scss";

export default class InputCreate extends React.Component{
  constructor(params) {
    super(params);
    this.state = {};
    this.reset();
  }

  reset() {
    this.setState({
      newInput: {
        title: ""
      }
    });
  }

  handleSubmit(e) {
    e.preventDefault();
    SK.inputs.create(this.state.newInput);
    this.reset();
  }

  handleChange(newInput) {
    this.setState({newInput});
  }

  render() {
    return (
      <form className={tk.StackedForm} onSubmit={this.handleSubmit.bind(this)}>
        <fieldset>
          <em>Add an input</em>

          <SKField type="text" onChange={this.handleChange.bind(this)} data={this.state.newInput} field="title" label="Title" placeholder="My Computer" />

          <button type="submit" className={tk.Button}>Create</button>
        </fieldset>
      </form>
    );
  }
}
