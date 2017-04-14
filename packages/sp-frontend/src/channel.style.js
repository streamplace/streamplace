
import styled from "styled-components";
import { FlexContainer } from "./shared.style";
import { Button } from "sp-styles";

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
`;

export const GoLiveButton = styled(Button)`
  margin: 0 0 0 auto;
  cursor: pointer;
  height: 80%;
  font-size: 0.7em;
  -webkit-app-region: no-drag;
  padding: 0.2em 0.5em;
  background-color: rgba(225, 0, 0, 1);
  border-color: rgba(225, 0, 0, 1);
  color: white;
  border-radius: 5px;

  &:hover {
    background-color: rgba(255, 0, 0, 1);
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
