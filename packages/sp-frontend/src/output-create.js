import React, { Component } from "react";
import {
  NiceLabel,
  BigInput,
  NiceForm,
  NiceSubmit,
  NiceTitle
} from "./shared.style";
import Loading from "./loading";
import { bindComponent } from "sp-components";

export class OutputCreate extends Component {
  static propTypes = {
    SP: React.PropTypes.object
  };

  constructor() {
    super();
    this.state = {
      title: "",
      url: "",
      creating: false
    };
  }

  submit(e) {
    e.preventDefault();
    this.setState({
      creating: true
    });
    this.props.SP.outputs
      .create({
        title: this.state.title,
        url: this.state.url
      })
      .then(() => {
        this.setState({
          title: "",
          url: "",
          creating: false
        });
      });
  }

  renderInner() {
    if (this.state.creating) {
      return <Loading />;
    }
    return (
      <div>
        <NiceLabel>
          Output Name
          <BigInput onChange={e => this.setState({ title: e.target.value })} />
        </NiceLabel>
        <NiceLabel>
          RTMP URL
          <BigInput onChange={e => this.setState({ url: e.target.value })} />
        </NiceLabel>
        <NiceSubmit>Create</NiceSubmit>
      </div>
    );
  }

  render() {
    return (
      <NiceForm onSubmit={e => this.submit(e)}>
        <NiceTitle>Create Output</NiceTitle>
        {this.renderInner()}
      </NiceForm>
    );
  }
}

export default bindComponent(OutputCreate);
