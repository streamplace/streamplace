import { AppBskyActorDefs } from "@atproto/api";
import {
  ChatMessageViewHydrated,
  LivestreamViewHydrated,
  PinnedRecordViewHydrated,
  PlaceStreamBadgeDefs,
  PlaceStreamDefs,
  PlaceStreamLiveTeleport,
  PlaceStreamModerationPermission,
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
  chatDraft: string;
  badgeSlots: {
    streamer: PlaceStreamBadgeDefs.BadgeSlot | null;
    user: PlaceStreamBadgeDefs.BadgeSlot | null;
  } | null;
  streamKey: string | null;
  setStreamKey: (key: string | null) => void;
  activeTeleport: PlaceStreamLiveTeleport.Record | null;
  activeTeleportUri: string | null;
  setActiveTeleportUri: (uri: string | null) => void;
  websocketConnected: boolean;
  hasReceivedSegment: boolean;
  pinnedComment: PinnedRecordViewHydrated | null;
  moderationPermissions: PlaceStreamModerationPermission.Record[];
  setModerationPermissions: (
    permissions: PlaceStreamModerationPermission.Record[],
  ) => void;
  localLivestreamURI: string | null;
  setLocalLivestreamURI: (uri: string | null) => void;
}

export interface LivestreamProblem {
  code: string;
  message: string;
  severity: "error" | "warning" | "info";
  link?: string;
}
