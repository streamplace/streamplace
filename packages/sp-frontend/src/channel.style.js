
import styled from "styled-components";
import { FlexContainer } from "./shared.style";
import { Button, colorLive, colorLiveLight } from "sp-styles";

export const UserBar = styled.div`
  background-color: #eee;
  width: 175px;
  flex-grow: 0;
  flex-shrink: 0;
`;

/**
 * Hacky. Yuck. But it gets the title of the channel up there on the title bar, so I'm fine with
 * it.
 */
export const TitleBar = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  -webkit-user-select: none;
  -webkit-app-region: drag;
  margin-top: -48px;
  height: 47px;
  display: flex;
  font-size: 2em;
  font-weight: 200;
  padding: 0 0.3em;
  align-items: center;
  background-color: ${props => props.active ? colorLiveLight : "white"};
  color: ${props => props.active ? "white" : "#333"};
  justify-content: space-between;
`;

export const GoLiveButton = styled(Button)`
  margin: 0;
  cursor: pointer;
  height: 80%;
  width: 115px;
  font-size: 0.7em;
  -webkit-app-region: no-drag;
  padding: 0.2em 0.5em;
  background-color: ${props => props.active ? "white" : colorLive};
  border: none;
  color: ${props => props.active ? "#333" : "white"};
  border-radius: 5px;

  &:hover {
    background-color: ${props => props.active ? "#ddd" : colorLiveLight};
  }
`;

export const ChannelName = styled.strong`
  font-weight: 600;
  font-size: 0.7em;
  position: relative;
`;

export const CanvasWrapper = styled(FlexContainer)`
  padding: 1em;
`;
