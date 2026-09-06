// The lexicon-typed client brands DID/AT-URI/datetime strings; values passed
// around the UI are plain strings sourced from server data, so delegated
// procedure calls cast those fields (same approach as js/components).
import { useSession } from "@/lib/session";
import { place } from "streamplace";

/**
 * Moderation writes for a stream the viewer owns or moderates. Owner calls
 * write to their own repo directly; delegated moderators go through the
 * place.stream.moderation.* procedures, which the server authorizes against
 * the streamer's delegation records.
 *
 * All functions reject on failure; callers own error surfacing.
 */
export function useModerationActions() {
  const { pdsAgent } = useSession();

  const requireAgent = () => {
    if (!pdsAgent?.did) throw new Error("Not logged in");
    return pdsAgent;
  };

  const blockUser = async (subjectDid: string, streamerDid: string) => {
    const agent = requireAgent();
    if (agent.did === streamerDid) {
      return agent.com.atproto.repo.createRecord({
        repo: agent.did,
        collection: "app.bsky.graph.block",
        record: {
          $type: "app.bsky.graph.block",
          subject: subjectDid,
          createdAt: new Date().toISOString(),
        },
      });
    }
    return agent.client.call(place.stream.moderation.createBlock, {
      streamer: streamerDid as any,
      subject: subjectDid as any,
    });
  };

  const hideMessage = async (messageUri: string, streamerDid: string) => {
    const agent = requireAgent();
    if (agent.did === streamerDid) {
      return agent.client.create(
        place.stream.chat.gate,
        { hiddenMessage: messageUri as any },
        { repo: agent.did as any },
      );
    }
    return agent.client.call(place.stream.moderation.createGate, {
      streamer: streamerDid as any,
      messageUri: messageUri as any,
    });
  };

  const pinMessage = async (
    messageUri: string,
    streamerDid: string,
    expiresAt?: string,
  ) => {
    const agent = requireAgent();
    if (agent.did === streamerDid) {
      return agent.com.atproto.repo.createRecord({
        repo: streamerDid,
        collection: "place.stream.chat.pinnedRecord",
        record: {
          $type: "place.stream.chat.pinnedRecord",
          pinnedMessage: messageUri,
          createdAt: new Date().toISOString(),
          ...(expiresAt ? { expiresAt } : {}),
        },
      });
    }
    return agent.client.call(place.stream.moderation.createPin, {
      streamer: streamerDid as any,
      messageUri: messageUri as any,
      ...(expiresAt ? { expiresAt: expiresAt as any } : {}),
    });
  };

  const unpinMessage = async (pinUri: string, streamerDid: string) => {
    const agent = requireAgent();
    if (agent.did === streamerDid) {
      const rkey = pinUri.split("/").pop();
      if (!rkey) throw new Error("Invalid pin URI");
      return agent.com.atproto.repo.deleteRecord({
        repo: streamerDid,
        collection: "place.stream.chat.pinnedRecord",
        rkey,
      });
    }
    return agent.client.call(place.stream.moderation.deletePin, {
      streamer: streamerDid as any,
      pinUri: pinUri as any,
    });
  };

  const updateStreamTitle = async (
    livestreamUri: string,
    title: string,
    streamerDid: string,
  ) => {
    const agent = requireAgent();
    if (agent.did === streamerDid) {
      const rkey = livestreamUri.split("/").pop();
      if (!rkey) throw new Error("Invalid livestream URI");
      // Livestream records are append-only: publishing a new record with a
      // fresh createdAt acts as a chapter marker, so the old record is kept.
      const { value } = await agent.client.get(place.stream.livestream, {
        repo: streamerDid as any,
        rkey,
      });
      return agent.client.create(
        place.stream.livestream,
        {
          ...(value as object),
          title,
          createdAt: new Date().toISOString() as any,
        },
        { repo: streamerDid as any },
      );
    }
    return agent.client.call(place.stream.moderation.updateLivestream, {
      streamer: streamerDid as any,
      livestreamUri: livestreamUri as any,
      title,
    });
  };

  return {
    blockUser,
    hideMessage,
    pinMessage,
    unpinMessage,
    updateStreamTitle,
  };
}
