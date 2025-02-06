import { OAuthSession } from "@aquareum/atproto-oauth-client-react-native";
import { Agent } from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { StreamKey } from "features/base/baseSlice";
import { StreamplaceOAuthClient } from "./oauthClient";
import { PlaceStreamLivestream } from "lexicons";

type NewLivestream = {
  loading: boolean;
  error: string | null;
  record: PlaceStreamLivestream.Record | null;
};

export interface BlueskyState {
  status: "start" | "loggedIn" | "loggedOut";
  oauthState: null | string;
  oauthSession: null | OAuthSession;
  pdsAgent: null | Agent;
  profiles: { [key: string]: ProfileViewDetailed };
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
  newLivestream: null | NewLivestream;
}
