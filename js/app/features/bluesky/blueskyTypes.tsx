import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { OutputSchema } from "@atproto/api/dist/client/types/com/atproto/repo/listRecords";
import { OAuthSession } from "@atproto/oauth-client";
import { StreamKey } from "features/base/baseSlice";
import {
  PlaceStreamChatProfile,
  PlaceStreamLivestream,
  PlaceStreamServerSettings,
  StreamplaceAgent,
} from "streamplace";
import { StreamplaceOAuthClient } from "./oauthClient";

type NewLivestream = {
  loading: boolean;
  error: string | null;
  record: PlaceStreamLivestream.Record | null;
};

export interface BlueskyState {
  status: "start" | "loggedIn" | "loggedOut";
  oauthState: null | string;
  oauthSession: null | OAuthSession;
  pdsAgent: null | StreamplaceAgent;
  anonPDSAgent: null | StreamplaceAgent;
  profiles: { [key: string]: ProfileViewDetailed };
  // for e.g. others' avatars
  profileCache: { [key: string]: ProfileViewDetailed };
  client: null | StreamplaceOAuthClient;
  login: {
    loading: boolean;
    error: null | string;
  };
  pds: {
    url: string;
    loading: boolean;
    error: null | string;
  };
  newKey: null | StreamKey;
  storedKey: null | StreamKey;
  isDeletingKey: boolean;
  streamKeysResponse: {
    loading: boolean;
    error: null | string;
    records: null | OutputSchema;
  };
  newLivestream: null | NewLivestream;
  chatProfile: {
    loading: boolean;
    error: null | string;
    profile: null | PlaceStreamChatProfile.Record;
  };
  serverSettings: null | PlaceStreamServerSettings.Record;
}
