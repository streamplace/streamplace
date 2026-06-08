// Reads the streamplace slice's URL through the small context that
// StreamplaceProvider publishes. Mirrors js/app/hooks/useStreamplaceNode.
// The web's slice already exposes `url` via useStore too, so callers
// who don't need the context boundary can just use `useStreamplaceUrl`
// from `../lib/store/hooks` directly.
import { createContext } from "react";
import { useStreamplaceUrl } from "../lib/store/hooks";

export interface StreamplaceNode {
  url: string;
}

export const StreamplaceContext = createContext<StreamplaceNode>({
  url: "",
});

export default function useStreamplaceNode(): StreamplaceNode {
  const url = useStreamplaceUrl();
  return { url };
}
