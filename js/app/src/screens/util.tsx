import { parseTimeHash, parseTimestamp } from "@streamplace/components";
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

/**
 * Temporal reference (YouTube-style #t=…) from the current URL, as a player
 * prop. The fragment wins (#t=5m5s); ?t=5m5s and ?startTime=305 work too for
 * embeds and deep links where fragments are awkward.
 */
export const startTimeFromLocation = (): Partial<PlayerProps> => {
  if (typeof window === "undefined") {
    return {};
  }
  const fromHash = parseTimeHash(window.location.hash);
  if (fromHash != null) {
    return { startTime: fromHash };
  }
  const query = new URLSearchParams(window.location.search);
  const parsed = parseTimestamp(
    query.get("t") ?? query.get("startTime") ?? undefined,
  );
  if (parsed == null) {
    return {};
  }
  return { startTime: parsed };
};
