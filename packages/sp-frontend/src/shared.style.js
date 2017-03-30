
import styled from "styled-components";

export const FlexContainer = styled.div`
  width: 100%;
  height: 100%;
  flex-grow: 1;
  display: flex;
  position: relative;

  flex-direction: ${props => props.column ? "column" : "row"};
`;
