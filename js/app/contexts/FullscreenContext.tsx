import React, { createContext, useContext, useState, ReactNode } from "react";

interface FullscreenContextValue {
  fullscreen: boolean;
  setFullscreen: (value: boolean) => void;
}

const FullscreenContext = createContext<FullscreenContextValue | undefined>(
  undefined,
);

export const FullscreenProvider = ({ children }: { children: ReactNode }) => {
  const [fullscreen, setFullscreen] = useState(false);
  return (
    <FullscreenContext.Provider value={{ fullscreen, setFullscreen }}>
      {children}
    </FullscreenContext.Provider>
  );
};

export function useFullscreen() {
  const ctx = useContext(FullscreenContext);
  if (!ctx) {
    throw new Error("useFullscreen must be used within a FullscreenProvider");
  }
  return ctx;
}
