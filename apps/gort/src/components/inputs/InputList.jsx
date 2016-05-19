
import React from "react";
import SK from "../../SK";
import { Link } from "react-router";

export default class InputList extends React.Component {
  constructor(params) {
    super(params);
    this.state = {inputs: []};
  }

  componentDidMount() {
    this.inputHandle = SK.inputs.watch({})
    .on("data", (inputs) => {
      this.setState({inputs});
    });
  }

  componentWillUnmount() {
    this.inputHandle.stop();
  }

  removeInput(id) {
    SK.inputs.delete(id);
  }

  render() {
    const inputNodes = this.state.inputs.map((input) => {
      return (
        <li key={input.id}>
          <Link to={`/inputs/${input.id}`}>{input.title}</Link>
          <button className="danger item-list-delete"
          onClick={this.removeInput.bind(null, input.id)}>
            delete
          </button>
        </li>
      );
    });
    return (
      <ul className="item-list">
        {inputNodes}
      </ul>
    );
  }
}
