
import styled from "styled-components";
import {FlexContainer} from "./shared.style";

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
  pointer-events: none;
  margin-top: -42px;
  font-size: 2em;
  font-weight: 200;
  padding-left: 0.3em;
`;

export const ChannelName = styled.strong`
  font-weight: 600;
  font-size: 0.7em;
  position: relative;
  top: -3px;
`;

export const CanvasWrapper = styled(FlexContainer)`
  padding: 1em;
`;
