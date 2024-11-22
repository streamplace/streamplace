import { AquareumContext } from "features/aquareum/aquareumProvider";
import { useContext } from "react";

export default function useAquareumNode() {
  return useContext(AquareumContext);
}
