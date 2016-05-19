
import React from "react";
import SK from "../../SK";
import { Link } from "react-router";

export default class OutputList extends React.Component {
  constructor(params) {
    super(params);
    this.state = {outputs: []};
  }

  componentDidMount() {
    this.outputHandle = SK.outputs.watch({})
    .on("data", (outputs) => {
      this.setState({outputs});
    });
  }

  componentWillUnmount() {
    this.outputHandle.stop();
  }

  removeOutput(id) {
    SK.outputs.delete(id);
  }

  render() {
    const outputNodes = this.state.outputs.map((output) => {
      return (
        <li key={output.id}>
          <Link to={`/outputs/${output.id}`}>{output.title}</Link>
          <button className="danger item-list-delete"
          onClick={this.removeOutput.bind(null, output.id)}>
            delete
          </button>
        </li>
      );
    });
    return (
      <ul className="item-list">
        {outputNodes}
      </ul>
    );
  }
}
