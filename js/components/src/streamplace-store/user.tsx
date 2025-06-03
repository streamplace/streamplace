import { PlaceStreamChatProfile } from "streamplace";
import {
  getStreamplaceStoreFromContext,
  useDID,
  useStreamplaceStore,
} from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

export function useGetChatProfile() {
  const did = useDID();
  const pdsAgent = usePDSAgent();
  const store = getStreamplaceStoreFromContext();

  return async () => {
    if (!did || !pdsAgent) {
      throw new Error("No DID or PDS agent");
    }
    const res = await pdsAgent.com.atproto.repo.getRecord({
      repo: did,
      collection: "place.stream.chat.profile",
      rkey: "self",
    });
    if (!res.success) {
      throw new Error("Failed to get chat profile record");
    }

    if (PlaceStreamChatProfile.isRecord(res.data.value)) {
      store.setState({ chatProfile: res.data.value });
    } else {
      console.log("not a record", res.data.value);
    }
  };
}

export function useGetBskyProfile() {
  const did = useDID();
  const pdsAgent = usePDSAgent();
  const store = getStreamplaceStoreFromContext();

  return async () => {
    if (!did || !pdsAgent) {
      throw new Error("No DID or PDS agent");
    }
    const res = await pdsAgent.app.bsky.actor.getProfile({
      actor: did,
    });
    if (!res.success) {
      throw new Error("Failed to get chat profile record");
    }

    store.setState({ handle: res.data.handle });
  };
}

export function useChatProfile() {
  return useStreamplaceStore((x) => x.chatProfile);
}
