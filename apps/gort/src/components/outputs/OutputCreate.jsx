
import React from "react";
import SK from "../../SK";

import SKField from "../SKField";
import tk from "twixtykit/styles.scss";

export default class OutputCreate extends React.Component{
  constructor(params) {
    super(params);
    this.state = {};
    this.reset();
  }

  reset() {
    this.setState({
      newOutput: {
        title: "",
        url: ""
      }
    });
  }

  handleSubmit(e) {
    e.preventDefault();
    SK.outputs.create(this.state.newOutput);
    this.reset();
  }

  handleChange(newOutput) {
    this.setState({newOutput});
  }

  render() {
    return (
      <form className={tk.StackedForm} onSubmit={this.handleSubmit.bind(this)}>
        <fieldset>
          <em>Add an output</em>

          <SKField type="text" onChange={this.handleChange.bind(this)} data={this.state.newOutput} field="title" label="Title" placeholder="My Channel" />
          <SKField type="text" onChange={this.handleChange.bind(this)} data={this.state.newOutput} field="url" label="RTMP URL" placeholder="rtmp://example.com/app/streamkey" />

          <button type="submit" className={tk.Button}>Create</button>
        </fieldset>
      </form>
    );
  }
}
