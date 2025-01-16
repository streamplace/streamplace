import { StreamplaceContext } from "features/streamplace/streamplaceProvider";
import { useContext } from "react";

export default function useStreamplaceNode() {
  return useContext(StreamplaceContext);
}
