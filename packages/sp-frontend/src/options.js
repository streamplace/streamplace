import PropTypes from "prop-types";
import React, { Component } from "react";
import { bindComponent } from "sp-components";
import { Button } from "sp-styles";
import { OptionsContainer } from "./options.style";

export class Options extends Component {
  static propTypes = {
    onLogout: PropTypes.func.isRequired
  };

  render() {
    return (
      <OptionsContainer>
        <Button onClick={this.props.onLogout}>Log out</Button>
      </OptionsContainer>
    );
  }
}

export default bindComponent(Options);
