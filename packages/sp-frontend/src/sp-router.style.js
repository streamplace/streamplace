
import styled from "styled-components";
import {NavLink} from "react-router-dom";

export const AppContainer = styled.div`
  height: 100%;
  display: flex;
  flex-direction: row;
`;

// Ordinarily for a sidebar I'd use `em`s and the size of the icons for the width, but we want
// this sidebar to line up exactly with Mac OS control icons. So here we are.
export const Sidebar = styled.header`
  background-color: #333;
  padding-top: 1em;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  width: 82px;
  -webkit-user-select: none;
  -webkit-app-region: drag;
`;

export const PageContainer = styled.div`
  display: flex;
  flex-direction: column;
  flex-grow: 1;
`;

const oColor = "#cccccc";
const oSize = "3px";

export const ChannelIcon = styled(NavLink)`
  width: 3em;
  height: 3em;
  display: block;
  cursor: pointer;
  border-radius: 0.4em;
  margin: 1.2em 0;
  background-color: ${props => props.icon ? "transparent" : "white"};
  color: black;
  overflow: hidden;
  display: flex;
  user-select: none;
  align-items: center;
  background-image: ${props => props.icon ? `url("${props.icon}")` : "none"};
  background-size: contain;
  opacity: 0.5;

  &:hover {
    box-shadow: ${oSize} ${oSize} 0px ${oColor}, -${oSize} -${oSize} 0px ${oColor}, ${oSize} -${oSize} 0px ${oColor}, -${oSize} ${oSize} 0px ${oColor};
  }

  &.active {
    opacity: 1;
  }
`;

export const ChannelIconText = styled.span`
  font-size: 2em;
`;
