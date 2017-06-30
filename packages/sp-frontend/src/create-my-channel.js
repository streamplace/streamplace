import React, { Component } from "react";
import { bindComponent } from "sp-components";
import styled from "styled-components";
import { parse as parseUrl } from "url";
import { GoodButton } from "sp-styles";

const ServerEntry = styled.div`font-size: 1.6em;`;

const Hostname = styled.span``;

const SlugInput = styled.input`
  border: none;
  border-bottom: 2px solid #333;
  background-color: transparent;

  &:focus,
  &:active {
    border-bottom-color: #00b8ff;
    outline: none;
  }
`;

export class CreateMyChannel extends Component {
  static propTypes = {
    SP: React.PropTypes.object
  };

  constructor() {
    super();
    this.state = {
      slug: "",
      creating: false
    };
    this.handleChange = this.handleChange.bind(this);
    this.handleSubmit = this.handleSubmit.bind(this);
  }

  handleChange(e) {
    this.setState({ slug: e.target.value });
  }

  handleSubmit(e) {
    e.preventDefault();
    this.setState({ creating: true });
    // Set your handle to the slug and create the channel
    this.props.SP.users
      .update(this.props.SP.user.id, { handle: this.state.slug })
      .then(() => {
        return this.props.SP.channels.create({ slug: this.state.slug });
      })
      .catch(err => {
        this.props.SP.error(err);
        this.setState({ creating: false });
      });
  }

  componentDidMount() {
    this.slugInput.focus();
  }

  render() {
    const { hostname } = parseUrl(this.props.SP.server);
    return (
      <div>
        <form onSubmit={this.handleSubmit}>
          <p>Create your channel:</p>
          <ServerEntry>
            <Hostname>
              {hostname}/
            </Hostname>
            <SlugInput
              innerRef={elem => (this.slugInput = elem)}
              placeholder="my-channel"
              value={this.state.slug}
              onChange={this.handleChange}
            />
          </ServerEntry>
          <GoodButton disabled={this.state.creating} type="submit">
            {this.state.creating ? "Creating..." : "Create"}
          </GoodButton>
        </form>
      </div>
    );
  }
}

export default bindComponent(CreateMyChannel);
