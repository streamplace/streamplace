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
  handleWebSocketMessages: (messages: any[]) => void;
  livestream: LivestreamViewHydrated | null;
  viewers: number | null;
  segment: PlaceStreamSegment.Record | null;
  renditions: PlaceStreamDefs.Rendition[];
}
