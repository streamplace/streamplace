import { place } from "./lexicons/index.js";

export interface LivestreamViewHydrated
  extends place.stream.livestream.LivestreamView {
  record: place.stream.livestream.Main;
}

export interface ChatMessageViewHydrated
  extends place.stream.chat.defs.MessageView {
  record: place.stream.chat.message.Main;
}

export interface PinnedRecordViewHydrated
  extends place.stream.chat.defs.PinnedRecordView {
  record: place.stream.chat.pinnedRecord.Main;
}

export interface VideoViewHydrated
  extends place.stream.media.getVideo.VideoView {
  record: place.stream.livestream.Main;
  tracks: TrackViewHydrated[];
}

export interface TrackViewHydrated extends place.stream.media.track.TrackView {
  record: place.stream.media.track.Main;
}
