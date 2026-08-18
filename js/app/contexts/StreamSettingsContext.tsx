import { createContext, ReactNode, useContext, useState } from "react";
import type { place } from "streamplace";

export type StreamSettingsMode = "create" | "metadata" | "moderation";

export interface StreamSettingsContextValue {
  mode: StreamSettingsMode;
  setMode: (mode: StreamSettingsMode) => void;
  metadata: place.stream.metadata.configuration.Main | null;
  setMetadata: (
    metadata: place.stream.metadata.configuration.Main | null,
  ) => void;
}

const StreamSettingsContext = createContext<
  StreamSettingsContextValue | undefined
>(undefined);

export function StreamSettingsProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<StreamSettingsMode>("create");
  const [metadata, setMetadata] =
    useState<place.stream.metadata.configuration.Main | null>(null);

  return (
    <StreamSettingsContext.Provider
      value={{ mode, setMode, metadata, setMetadata }}
    >
      {children}
    </StreamSettingsContext.Provider>
  );
}

export function useStreamSettings() {
  const ctx = useContext(StreamSettingsContext);
  if (ctx === undefined) {
    throw new Error(
      "useStreamSettings must be used within a StreamSettingsProvider",
    );
  }
  return ctx;
}
