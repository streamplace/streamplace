
import React, { Component } from "react";
import styled from "styled-components";

const Bar = styled.header`
  background-color: white;
  border-bottom: 1px solid #ccc;
  height: 50px;
  display: flex;
  position: relative;
  -webkit-user-select: none;
  -webkit-app-region: drag;
`;

const UserMenuLink = styled.a`
  display: block;
  margin-left: auto;
  height: 100%;
  display: flex;
  -webkit-app-region: no-drag;
  width: 3em;
`;

const UserMenuIcon = styled.i`
  margin: auto;
  margin: auto 0.8em;
  cursor: pointer;
  -webkit-app-region: no-drag;
  &::before {
    cursor: pointer;
    font-size: 1.5em;
    color: #999;
    -webkit-app-region: no-drag;
    a:hover & {
      color: #ccc;
    }
  }
`;

const UserMenu = styled.ul`
  position: absolute;
  display: ${(props) => props.open ? "block" : "none"};
  right: 1em;
  margin: 0;
  margin-top: 0.5em;
  top: 100%;
  background-color: white;
  list-style-type: none;
  border-radius: 3px;
  border: 1px solid #ccc;
  z-index: 100;
  user-select: none;
  padding: 0;
  min-width: 150px;
`;

const UserMenuItem = styled.li`
  padding: 0.5em 1em;
  cursor: pointer;
  font-size: 0.9em;
  color: #333;

  &:hover {
    background-color: #eee;
  }
`;

export default class TopBar extends Component {
  static propTypes = {
    onLogout: React.PropTypes.func.isRequired,
  };

  componentWillMount() {
    // "clickoutside" handler. if we got here, make sure our menu is closed.
    this.listener = (e) => {
      if (this.state.menuOpen) {
        this.setState({menuOpen: false});
      }
    };
    window.addEventListener("click", this.listener);
  }

  componentWillUnmount() {
    window.removeEventListener("click", this.listener);
  }

  constructor() {
    super();
    this.state = {menuOpen: false};
  }

  toggleMenu(e) {
    e.stopPropagation();
    this.setState({menuOpen: !this.state.menuOpen});
  }

  handleLogout() {
    this.setState({menuOpen: false});
    this.props.onLogout();
  }

  render () {
    return (
      <Bar>
        <UserMenuLink onClick={(e) => this.toggleMenu(e)}>
          <UserMenuIcon className="fa fa-gear" />
        </UserMenuLink>
        <UserMenu onClick={(e) => e.stopPropagation()} open={this.state.menuOpen}>
          <UserMenuItem onClick={() => this.handleLogout()}>Log Out</UserMenuItem>
        </UserMenu>
      </Bar>
    );
  }
}
