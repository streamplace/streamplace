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
  const createEmptyChatProfile = useCreateEmptyChatProfile();

  return async () => {
    if (!did || !pdsAgent) {
      throw new Error("No DID or PDS agent");
    }
    let res;
    try {
      res = await pdsAgent.com.atproto.repo.getRecord({
        repo: did,
        collection: "place.stream.chat.profile",
        rkey: "self",
      });
    } catch (e) {
      console.error(
        "Failed to get chat profile record, attempting creation",
        e,
      );
    }
    if (!res || !res.success) {
      try {
        await createEmptyChatProfile();
        res = await pdsAgent.com.atproto.repo.getRecord({
          repo: did,
          collection: "place.stream.chat.profile",
          rkey: "self",
        });
      } catch (e) {
        console.error("Failed to create empty chat profile record", e);
      }
    }

    if (PlaceStreamChatProfile.isRecord(res.data.value)) {
      store.setState({ chatProfile: res.data.value });
    } else {
      console.log("not a record", res.data.value);
    }
  };
}

export function useCreateEmptyChatProfile() {
  const did = useDID();
  const pdsAgent = usePDSAgent();

  return async () => {
    if (!did || !pdsAgent) {
      throw new Error("No DID or PDS agent");
    }
    const res = await pdsAgent.com.atproto.repo.putRecord({
      repo: did,
      collection: "place.stream.chat.profile",
      rkey: "self",
      record: {
        $type: "place.stream.chat.profile",
        color:
          DEFAULT_COLORS[Math.floor(Math.random() * DEFAULT_COLORS.length)],
      },
    });
    if (!res.success) {
      throw new Error("Failed to create empty chat profile record");
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

const DEFAULT_COLORS: PlaceStreamChatProfile.Color[] = [
  { red: 244, green: 67, blue: 54 },
  { red: 233, green: 30, blue: 99 },
  { red: 156, green: 39, blue: 176 },
  { red: 103, green: 58, blue: 183 },
  { red: 63, green: 81, blue: 181 },
  { red: 33, green: 150, blue: 243 },
  { red: 3, green: 169, blue: 244 },
  { red: 0, green: 188, blue: 212 },
  { red: 0, green: 150, blue: 136 },
  { red: 76, green: 175, blue: 80 },
  { red: 139, green: 195, blue: 74 },
  { red: 205, green: 220, blue: 57 },
  { red: 255, green: 235, blue: 59 },
  { red: 255, green: 193, blue: 7 },
  { red: 255, green: 152, blue: 0 },
  { red: 255, green: 87, blue: 34 },
  { red: 121, green: 85, blue: 72 },
  { red: 158, green: 158, blue: 158 },
  { red: 96, green: 125, blue: 139 },
];
