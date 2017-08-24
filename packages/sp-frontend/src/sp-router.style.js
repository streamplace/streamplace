import styled from "styled-components";
import { NavLink } from "react-router-dom";
import ColorHash from "color-hash";

const colorHash = new ColorHash();

const getColor = string => {
  const [r, g, b] = colorHash.rgb(string);
  return `rgb(${r}, ${g}, ${b})`;
};

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
  flex-shrink: 0;
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

const optionsStyles = () => `
  margin-top: auto;
  background-color: #444;
`;

export const ChannelIcon = styled(NavLink)`
  width: 3em;
  height: 3em;
  display: block;
  cursor: pointer;
  border-radius: 0.4em;
  margin: 1.2em 0;
  background-color: ${props =>
    props.icon ? "transparent" : getColor(props.id)};
  color: white;
  overflow: hidden;
  display: flex;
  user-select: none;
  align-items: center;
  justify-content: center;
  background-image: ${props => (props.icon ? `url("${props.icon}")` : "none")};
  background-size: contain;
  opacity: 0.5;

  ${props => props.id === "channel-options" && optionsStyles()}

  &:hover {
    box-shadow: ${oSize} ${oSize} 0px ${oColor}, -${oSize} -${oSize} 0px ${oColor}, ${oSize} -${oSize} 0px ${oColor}, -${oSize} ${oSize} 0px ${oColor};
  }

  &.active {
    opacity: 1;
  }
`;

export const ChannelIconText = styled.span`font-size: 2em;`;
