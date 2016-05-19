
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";

export default class InputDetail extends React.Component{
  constructor() {
    super();
    this.state = {
      input: {}
    };
  }

  componentDidMount() {
    const inputId = this.props.params.inputId;
    this.inputHandle = SK.inputs.watch({id: inputId})
    .on("data", ([input]) => {
      this.setState({input});
    })
    .catch((...args) => {
      twixty.error(...args);
    });
  }

  componentWillUnmount() {
    this.inputHandle.stop();
  }

  render () {
    if (!this.state.input.id) {
      return <div />;
    }
    return (
      <div>
        <h4>Input {this.state.input.title}</h4>
        <p>Stream Key: {this.state.input.streamKey}</p>
      </div>
    );
  }
}

InputDetail.propTypes = {
  "params": React.PropTypes.object.isRequired
};
