
import styled from "styled-components";

export const CanvasContainer = styled.div`
  flex-grow: 1;
  overflow: hidden;
  position: relative;
`;

// Channel contents is all position: none, 'cuz we manually grab stuff and render with WebGL
export const ChannelContents = styled.div`
  position: absolute;
  display: none;
`;

export const AutoShrink = styled.div`
  position: absolute;
  left: 50%;
  top: 50%;
`;

export const Canvas = styled.canvas`
  background-color: black;
  position: absolute;
`;
