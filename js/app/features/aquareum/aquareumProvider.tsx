import { createContext, useEffect } from "react";
import { DEFAULT_URL, initialize, selectAquareum } from "./aquareumSlice";
import { useAppDispatch, useAppSelector } from "store/hooks";

export const AquareumContext = createContext({
  url: DEFAULT_URL,
});

export default function AquareumProvider({
  children,
}: {
  children: React.ReactNode;
}): React.ReactElement {
  const aquareum = useAppSelector(selectAquareum);
  const dispatch = useAppDispatch();
  useEffect(() => {
    if (!aquareum.initialized) {
      dispatch(initialize());
    }
  }, [aquareum.initialized]);
  if (!aquareum.initialized) {
    return <></>;
  }
  return (
    <AquareumContext.Provider value={{ url: aquareum.url }}>
      {children}
    </AquareumContext.Provider>
  );
}
