import { AppBskyGraphBlock } from "@atproto/api";
import { useState } from "react";
import { usePDSAgent } from "./xrpc";

/**
 * Hook to create a block record (ban user from chat).
 *
 * When the caller is the stream owner (agent.did === streamerDID), creates the
 * block record directly via ATProto writes to their repo.
 *
 * When the caller is a delegated moderator, uses the place.stream.moderation.createBlock
 * XRPC endpoint which validates permissions and creates the record using the
 * streamer's OAuth session.
 */
export function useCreateBlockRecord() {
  let agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const createBlock = async (subjectDID: string, streamerDID?: string) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    setIsLoading(true);
    try {
      // If no streamerDID provided or caller is the streamer, use direct ATProto write
      if (!streamerDID || agent.did === streamerDID) {
        const record: AppBskyGraphBlock.Record = {
          $type: "app.bsky.graph.block",
          subject: subjectDID,
          createdAt: new Date().toISOString(),
        };
        const result = await agent.com.atproto.repo.createRecord({
          repo: agent.did,
          collection: "app.bsky.graph.block",
          record,
        });
        return result;
      }

      // Otherwise, use delegated moderation endpoint
      const result = await agent.place.stream.moderation.createBlock({
        streamer: streamerDID,
        subject: subjectDID,
      });
      return result;
    } finally {
      setIsLoading(false);
    }
  };

  return { createBlock, isLoading };
}

/**
 * Hook to create a gate record (hide a chat message).
 *
 * When the caller is the stream owner (agent.did === streamerDID), creates the
 * gate record directly via ATProto writes to their repo.
 *
 * When the caller is a delegated moderator, uses the place.stream.moderation.createGate
 * XRPC endpoint which validates permissions and creates the record using the
 * streamer's OAuth session.
 */
export function useCreateHideChatRecord() {
  let agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const createHideChat = async (
    chatMessageUri: string,
    streamerDID?: string,
  ) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    setIsLoading(true);
    try {
      // If no streamerDID provided or caller is the streamer, use direct ATProto write
      if (!streamerDID || agent.did === streamerDID) {
        const record = {
          $type: "place.stream.chat.gate",
          hiddenMessage: chatMessageUri,
        };

        const result = await agent.com.atproto.repo.createRecord({
          repo: agent.did,
          collection: "place.stream.chat.gate",
          record,
        });
        return result;
      }

      // Otherwise, use delegated moderation endpoint
      const result = await agent.place.stream.moderation.createGate({
        streamer: streamerDID,
        messageUri: chatMessageUri,
      });
      return result;
    } finally {
      setIsLoading(false);
    }
  };

  return { createHideChat, isLoading };
}

/**
 * Hook to update a livestream record (update stream title).
 *
 * When the caller is the stream owner (agent.did === streamerDID), updates the
 * livestream record directly via ATProto writes to their repo.
 *
 * When the caller is a delegated moderator, uses the place.stream.moderation.updateLivestream
 * XRPC endpoint which validates permissions and updates the record using the
 * streamer's OAuth session.
 */
export function useUpdateLivestreamRecord() {
  let agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const updateLivestream = async (
    livestreamUri: string,
    title: string,
    streamerDID?: string,
  ) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    setIsLoading(true);
    try {
      // If no streamerDID provided or caller is the streamer, use direct ATProto write
      if (!streamerDID || agent.did === streamerDID) {
        // Extract rkey from URI
        const rkey = livestreamUri.split("/").pop();
        if (!rkey) {
          throw new Error("Invalid livestream URI");
        }

        // Get existing record first
        const getResult = await agent.com.atproto.repo.getRecord({
          repo: agent.did,
          collection: "place.stream.livestream",
          rkey,
        });

        const oldRecord = getResult.data.value as any;

        // Update the record
        const record = {
          $type: "place.stream.livestream",
          title: title,
          url: oldRecord.url,
          createdAt: oldRecord.createdAt,
          post: oldRecord.post,
          thumb: oldRecord.thumb,
        };

        const result = await agent.com.atproto.repo.putRecord({
          repo: agent.did,
          collection: "place.stream.livestream",
          rkey,
          record,
          swapRecord: getResult.data.cid,
        });
        return result;
      }

      // Otherwise, use delegated moderation endpoint
      const result = await agent.place.stream.moderation.updateLivestream({
        streamer: streamerDID,
        livestreamUri: livestreamUri,
        title: title,
      });
      return result;
    } finally {
      setIsLoading(false);
    }
  };

  return { updateLivestream, isLoading };
}
