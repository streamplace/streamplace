import styled from "styled-components";

export const UserFrame = styled.div`
  display: flex;
  flex-direction: column;
  user-select: none;
  margin-bottom: 0.5em;
  padding: 0.7em;
  position: relative;
`;

export const CanvasWrapper = styled.div`
  height: 160px;
  display: flex;
`;

export const ActionBar = styled.div`
  display: flex;
  justify-content: space-between;
  background-color: #ccc;
  border-radius: 5px;
  margin: 0.5em 0;
`;

export const ActionButton = styled.a`
  display: block;
  flex-basis: 0px;
  padding: 0.2em 0.5em;
  cursor: pointer;
  flex-grow: 1;
  text-align: center;
  color: #333;
  border-right: 1px solid #333;

  &:last-child {
    border-right: none;
  }
`;

export const UserTitle = styled.strong`
  text-align: center;
  display: block;
  cursor: default;
`;
