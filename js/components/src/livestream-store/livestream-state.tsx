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
  renditions: PlaceStreamDefs.Rendition[];
  replyToMessage: ChatMessageViewHydrated | null;
  streamKey: string | null;
  setStreamKey: (key: string | null) => void;
}
