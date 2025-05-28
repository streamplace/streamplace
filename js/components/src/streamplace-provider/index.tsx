import React, { useRef } from "react";
import { makeStreamplaceStore } from "../streamplace-store/streamplace-store";
import { StreamplaceContext } from "./context";
import Poller from "./poller";

export function StreamplaceProvider({
  children,
  url,
}: {
  children: React.ReactNode;
  url: string;
}) {
  // todo: handle url changes?
  const store = useRef(makeStreamplaceStore({ url })).current;

  return (
    <StreamplaceContext.Provider value={{ store: store }}>
      <Poller>{children}</Poller>
    </StreamplaceContext.Provider>
  );
}
