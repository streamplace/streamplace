import { createContext } from "react";
import { DEFAULT_URL, selectAquareum } from "./aquareumSlice";
import { useAppSelector } from "store/hooks";

export const AquareumContext = createContext({
  url: DEFAULT_URL,
});

export default function AquareumProvider({
  children,
}: {
  children: React.ReactNode;
}): React.ReactElement {
  const aquareum = useAppSelector(selectAquareum);
  return (
    <AquareumContext.Provider value={{ url: aquareum.url }}>
      {children}
    </AquareumContext.Provider>
  );
}
