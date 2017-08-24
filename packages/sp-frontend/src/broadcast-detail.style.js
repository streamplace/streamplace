import styled from "styled-components";
import { activeColor } from "./shared.style";

export const Column = styled.div`
  flex-basis: 0;
  flex-grow: 1;
  padding: 0 1em 1em 1em;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
`;

export const Stack = styled.div`
  flex-grow: 2;
  flex-basis: 0;
  background-color: #ccc;
`;

export const StackTitle = styled.h4`
  text-align: center;
  position: relative;
`;

export const StackDragWrapper = styled.div`padding: 1em;`;

const streamKinds = {
  Input: `
    background-color: #ffbfbf;
  `,
  File: `
    background-color: #c3c3ff;
  `
};

export const StackItem = styled.div`
  background-color: white;
  border: 1px solid #333;
  padding: 1em;

  ${props => streamKinds[props.kind]};
`;

export const OutputTitle = styled.strong``;

export const Output = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5em;
`;

export const OutputButton = styled.button`
  border: none;
  font-size: 1.5em;
  position: relative;
  cursor: pointer;

  background-color: ${props => (props.active ? activeColor : "#aaa")};
  margin-left: 0.2em;

  border: 1px solid transparent;

  &:focus,
  &:active {
    border: 1px solid ${activeColor};
    outline: none;
  }
`;
