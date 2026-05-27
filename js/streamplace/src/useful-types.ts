import {
  PlaceStreamChatDefs,
  PlaceStreamChatMessage,
  PlaceStreamChatPinnedRecord,
  PlaceStreamLivestream,
  PlaceStreamMediaGetVideo,
} from "./lexicons";

export interface LivestreamViewHydrated
  extends PlaceStreamLivestream.LivestreamView {
  record: PlaceStreamLivestream.Record;
}

export interface ChatMessageViewHydrated
  extends PlaceStreamChatDefs.MessageView {
  record: PlaceStreamChatMessage.Record;
}

export interface PinnedRecordViewHydrated
  extends PlaceStreamChatDefs.PinnedRecordView {
  record: PlaceStreamChatPinnedRecord.Record;
}

export interface VideoViewHydrated extends PlaceStreamMediaGetVideo.VideoView {
  record: PlaceStreamLivestream.Record;
}
