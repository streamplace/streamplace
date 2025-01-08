import { PlayerProps } from "components/player/props";

export const queryToProps = (query: URLSearchParams): Partial<PlayerProps> => {
  const entries = { ...Object.fromEntries(query) } as Record<string, any>;
  for (const [key, value] of Object.entries(entries)) {
    if (value === "true") {
      entries[key] = true;
    } else if (value === "false") {
      entries[key] = false;
    }
  }
  return entries as Partial<PlayerProps>;
};
