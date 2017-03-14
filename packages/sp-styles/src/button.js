
import styled from "styled-components";
import color from "color";
import {colorGold, colorGoldLight, colorBackground} from "./colors";

export const Button = styled.button`
  font-size: 1.3em;
  background-color: transparent;
  padding: 0.7em 1.2em;
  margin: 1em;
  border: 1px solid black;
  font-weight: 200;
  cursor: pointer;
  &:focus {
    outline: none;
  }
  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  };
`;

export const GoodButton = styled(Button)`
  color: black;
  border-color: ${colorBackground};
  color: ${colorBackground};
  &:hover, &:focus {
    background-color: ${colorGoldLight};
    &:disabled {
      background-color: transparent;
    }
  }
`;

// formerly #ff2d2d
export const BadButton = styled(Button)`
  color: ${colorGold};
  border-color: ${colorGold};
  &:hover, &:focus {
    background-color: #555;
    color: ${colorGoldLight};
    border-color: ${colorGoldLight};
  }
`;
