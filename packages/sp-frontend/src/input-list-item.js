import PropTypes from "prop-types";
import React, { Component } from "react";
import { bindComponent } from "sp-components";

export class InputListItem extends Component {
  constructor() {
    super();
    this.state = {
      showKey: false
    };
  }
  static propTypes = {
    input: PropTypes.object.isRequired,
    SP: PropTypes.object
  };

  toggleKey() {
    this.setState({ showKey: !this.state.showKey });
  }

  renderKey() {
    if (this.state.showKey) {
      return (
        <span>
          {this.props.input.streamKey}
          &nbsp; &nbsp;
          <a onClick={() => this.toggleKey()}>Hide</a>
        </span>
      );
    }
    return <a onClick={() => this.toggleKey()}>Show</a>;
  }

  render() {
    const { input, SP } = this.props;
    return (
      <div key={input.id}>
        <h4>{input.title}</h4>
        <p>
          <strong>URL: </strong>
          rtmp://{SP.schema.host}/stream
        </p>
        <p>
          <strong>Stream Key:</strong> {this.renderKey()}
        </p>
      </div>
    );
  }
}

export default bindComponent(InputListItem);
