
import styled from "styled-components";

export const UserFrame = styled.div`
  height: 210px;
  display: flex;
  flex-direction: column;
  user-select: none;
  margin-bottom: 0.5em;
  padding: 0.7em;
  position: relative;
`;

export const UserTitle = styled.strong`
  text-align: center;
  display: block;
  cursor: default;
`;

export const RemoveButton = styled.a`
  cursor: pointer;
  position: absolute;
  right: 1em;
  bottom: 0.8em;
`;

export const SceneToggleButton = styled.a`
  cursor: pointer;
  position: absolute;
  left: 1em;
  bottom: 0.8em;
`;

