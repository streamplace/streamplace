import {
  PlaceStreamChatDefs,
  PlaceStreamChatMessage,
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
