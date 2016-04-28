
import React from "react";

import SK from "../../SK";

export default class VertexPositionEditor extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      positions: props.positions,
      vertexId: props.vertexId,
    };
  }

  handleChange(inputName, field, e) {
    const positions = {...this.state.positions};
    positions[inputName][field] = parseInt(e.target.value);
    this.setState({positions});
  }

  handleSave() {
    const newVertex = {
      params: {
        inputCount: Object.keys(this.state.positions).length,
        positions: this.state.positions
      }
    };
    SK.vertices.update(this.state.vertexId, newVertex)
    .then(() => {
      // console.log("Success!");
    })
    .catch((err) => {
      // console.error(err);
    });
  }

  render() {
    const inputs = Object.keys(this.state.positions).map((inputName) => {
      const input = this.state.positions[inputName];
      return (
        <div key={inputName}>
          <strong>{inputName}</strong>
          <p>X: <input onChange={this.handleChange.bind(this, inputName, "x")} type="text" length="4" value={input.x}/></p>
          <p>Y: <input onChange={this.handleChange.bind(this, inputName, "y")} type="text" length="4" value={input.y}/></p>
          <p>Width: <input onChange={this.handleChange.bind(this, inputName, "width")} type="text" length="4" value={input.width}/></p>
          <p>Height: <input onChange={this.handleChange.bind(this, inputName, "height")} type="text" length="4" value={input.height}/></p>
        </div>
      );
    });
    return <div>
      {inputs}
      <div>
        <button type="button" onClick={this.handleSave.bind(this)}>Save Positions</button>
      </div>
    </div>;
  }
}

VertexPositionEditor.propTypes = {
  vertexId: React.PropTypes.string.isRequired,
  positions: React.PropTypes.object.isRequired,
};
