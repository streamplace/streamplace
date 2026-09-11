import { AppBskyActorDefs } from "@atproto/api";
import {
  ChatMessageViewHydrated,
  LivestreamViewHydrated,
  PinnedRecordViewHydrated,
  place,
} from "streamplace";

/**
 * A permission record plus its AT-URI, which is needed to apply deletion
 * events to records loaded before the websocket connection was established.
 */
export type LivestreamModerationPermission =
  place.stream.moderation.permission.Main & {
    uri?: string;
  };

export interface LivestreamState {
  profile: AppBskyActorDefs.ProfileViewBasic | null;
  chatIndex: { [key: string]: ChatMessageViewHydrated };
  chat: ChatMessageViewHydrated[];
  authors: { [key: string]: ChatMessageViewHydrated["chatProfile"] };
  livestream: LivestreamViewHydrated | null;
  viewers: number | null;
  /** Running number of playback sessions this livestream has had. */
  viewTotal: number | null;
  pendingHides: string[];
  segment: place.stream.segment.Main | null;
  recentSegments: place.stream.segment.Main[];
  problems: LivestreamProblem[];
  renditions: place.stream.defs.Rendition[];
  replyToMessage: ChatMessageViewHydrated | null;
  chatDraft: string;
  badgeSlots: {
    streamer: place.stream.badge.defs.BadgeSlot | null;
    user: place.stream.badge.defs.BadgeSlot | null;
  } | null;
  streamKey: string | null;
  setStreamKey: (key: string | null) => void;
  activeTeleport: place.stream.live.teleport.Main | null;
  activeTeleportUri: string | null;
  setActiveTeleportUri: (uri: string | null) => void;
  websocketConnected: boolean;
  hasReceivedSegment: boolean;
  pinnedComment: PinnedRecordViewHydrated | null;
  moderationPermissions: LivestreamModerationPermission[];
  setModerationPermissions: (
    permissions: LivestreamModerationPermission[],
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
