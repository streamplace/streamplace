import { AppBskyActorDefs } from "@atproto/api";
import {
  ChatMessageViewHydrated,
  LivestreamViewHydrated,
  PlaceStreamDefs,
  PlaceStreamSegment,
} from "streamplace";

export interface LivestreamState {
  profile: AppBskyActorDefs.ProfileViewBasic | null;
  chatIndex: { [key: string]: ChatMessageViewHydrated };
  chat: ChatMessageViewHydrated[];
  authors: { [key: string]: ChatMessageViewHydrated["chatProfile"] };
  livestream: LivestreamViewHydrated | null;
  viewers: number | null;
  pendingHides: string[];
  segment: PlaceStreamSegment.Record | null;
  recentSegments: PlaceStreamSegment.Record[];
  problems: LivestreamProblem[];
  renditions: PlaceStreamDefs.Rendition[];
  replyToMessage: ChatMessageViewHydrated | null;
  streamKey: string | null;
  setStreamKey: (key: string | null) => void;
  websocketConnected: boolean;
  hasReceivedSegment: boolean;
}

export interface LivestreamProblem {
  code: string;
  message: string;
  severity: "error" | "warning" | "info";
  link?: string;
}
