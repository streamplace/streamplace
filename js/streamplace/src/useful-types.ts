import {
  PlaceStreamChatDefs,
  PlaceStreamChatMessage,
  PlaceStreamChatPinnedRecord,
  PlaceStreamLivestream,
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
