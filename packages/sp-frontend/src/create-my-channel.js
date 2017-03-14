
import React, { Component } from "react";
import {subscribe} from "./sp-binding";
import styled from "styled-components";
import {parse as parseUrl} from "url";

const ServerEntry = styled.div`
  font-size: 1.6em;
`;

const Hostname = styled.span`

`;

const SlugInput = styled.input`
  border: none;
  border-bottom: 2px solid #333;
  background-color: transparent;

  &:focus, &:active {
    border-bottom-color: #00b8ff;
    outline: none;
  }
`;

export class CreateMyChannel extends Component {
  constructor() {
    super();
    this.state = {
      slug: "",
    };
    this.handleChange = this.handleChange.bind(this);
  }

  handleChange(e) {
    this.setState({slug: e.target.value});
  }

  componentDidMount() {
    this.slugInput.focus();
  }

  render () {
    const {hostname} = parseUrl(this.props.SP.server);
    return (
      <div>
        <p>Create your channel:</p>
        <ServerEntry>
          <Hostname>{hostname}/</Hostname><SlugInput innerRef={elem => this.slugInput = elem} placeholder="my-channel" value={this.state.slug} onChange={this.handleChange} />
        </ServerEntry>
      </div>
    );
  }
}

export default subscribe(CreateMyChannel);
