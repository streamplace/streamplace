// React hooks for chat/moderation state. The pure reducer and types live
// in @streamplace/core; these hooks bind them to the LivestreamStore and
// to React lifecycle.
import { ComAtprotoModerationCreateReport, RichText } from "@atproto/api";
import {
  type LivestreamState,
  type NewChatMessage,
  reduceChat,
} from "@streamplace/core";
import { useCallback } from "react";
import { ChatMessageViewHydrated, place } from "streamplace";
import { useChatProfile, useDID, useHandle } from "../streamplace-store";
import { usePDSAgent } from "../streamplace-store/xrpc";
import { getStoreFromContext, useLivestreamStore } from "./use-store";

export { NewChatMessage };

export const useReplyToMessage = () =>
  useLivestreamStore((state) => state.replyToMessage);

export const useSetReplyToMessage = () => {
  const store = getStoreFromContext();
  return useCallback(
    (message: ChatMessageViewHydrated | null) => {
      store.setState({ replyToMessage: message });
    },
    [store],
  );
};

export const useChatDraft = () =>
  useLivestreamStore((state) => state.chatDraft);

export const useSetChatDraft = () => {
  const store = getStoreFromContext();
  return useCallback(
    (draft: string) => {
      store.setState({ chatDraft: draft });
    },
    [store],
  );
};

export const useBadgeSlots = () =>
  useLivestreamStore((state) => state.badgeSlots);

export const useSetBadgeSlots = () => {
  const store = getStoreFromContext();
  return useCallback(
    (slots: NonNullable<LivestreamState["badgeSlots"]> | null) => {
      store.setState({ badgeSlots: slots });
    },
    [store],
  );
};

export const usePendingHides = () =>
  useLivestreamStore((state) => state.pendingHides);

export const useAddPendingHide = () => {
  const store = getStoreFromContext();
  return useCallback(
    (messageUri: string) => {
      const state = store.getState();
      if (!state.pendingHides.includes(messageUri)) {
        const newPendingHides = [...state.pendingHides, messageUri];
        const newState = reduceChat(state, [], [], [messageUri]);
        store.setState({
          ...newState,
          pendingHides: newPendingHides,
        });
      }
    },
    [store],
  );
};

export const useCreateChatMessage = () => {
  const pdsAgent = usePDSAgent();
  const store = getStoreFromContext();
  const userDID = useDID();
  const userHandle = useHandle();
  const chatProfile = useChatProfile();

  return async (msg: NewChatMessage) => {
    if (!pdsAgent || !userDID) {
      throw new Error("No PDS agent or user DID found");
    }

    let state = store.getState();

    const streamerProfile = state.profile;

    if (!streamerProfile) {
      throw new Error("Profile not found");
    }

    const rt = new RichText({ text: msg.text });
    await rt.detectFacets(pdsAgent);

    // filter out any facets that aren't in the allowed list
    rt.facets = rt.facets?.filter((facet) => {
      return (
        // if all features are in the allowed list
        facet.features.every((feature) =>
          [
            "app.bsky.richtext.facet#link",
            "app.bsky.richtext.facet#mention",
          ].includes(feature.$type),
        )
      );
    });

    const record = {
      $type: "place.stream.chat.message",
      text: msg.text,
      createdAt: new Date().toISOString(),
      streamer: streamerProfile.did,
      facets: rt.facets,
      ...(msg.reply
        ? {
            reply: {
              root: {
                cid: msg.reply.cid,
                uri: msg.reply.uri,
              },
              parent: {
                cid: msg.reply.cid,
                uri: msg.reply.uri,
              },
            },
          }
        : {}),
    } as unknown as place.stream.chat.message.Main;

    const localChat: ChatMessageViewHydrated = {
      uri: `local-${Date.now()}` as any,
      cid: "",
      author: {
        did: userDID as any,
        handle: (userHandle || userDID) as any,
      },
      record: record,
      indexedAt: new Date().toISOString() as any,
      chatProfile: chatProfile || undefined,
    };

    state = reduceChat(state, [localChat], [], []);
    store.setState(state);

    try {
      await pdsAgent.com.atproto.repo.createRecord({
        repo: userDID,
        collection: "place.stream.chat.message",
        record,
      });
    } catch (err) {
      // Remove the optimistic message if the server call fails
      const currentState = store.getState();
      const updatedIndex = { ...currentState.chatIndex };
      for (const [key, existingMsg] of Object.entries(updatedIndex)) {
        if (existingMsg.uri === localChat.uri) {
          delete updatedIndex[key];
          break;
        }
      }
      store.setState({
        ...currentState,
        chatIndex: updatedIndex,
        chat: Object.keys(updatedIndex)
          .sort((a, b) => {
            const aTime = parseInt(a.split("-")[0], 10);
            const bTime = parseInt(b.split("-")[0], 10);
            return bTime - aTime;
          })
          .map((key) => updatedIndex[key]),
      });
      throw err;
    }
  };
};

export const useDeleteChatMessage = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();
  return async (uri: string) => {
    if (!pdsAgent) {
      throw new Error("No PDS agent found");
    }
    if (!userDID) {
      throw new Error("No user DID found");
    }
    const rkey = uri.split("/").pop();
    if (!rkey) {
      throw new Error("No rkey found");
    }
    return await pdsAgent.com.atproto.repo.deleteRecord({
      repo: userDID,
      collection: "place.stream.chat.message",
      rkey: rkey,
    });
  };
};

export const useAddSystemMessage = () => {
  const store = getStoreFromContext();
  return useCallback(
    (message: ChatMessageViewHydrated) => {
      const state = store.getState();
      const newState = reduceChat(state, [message], []);
      store.setState(newState);
    },
    [store],
  );
};

export const useSubmitReport = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();

  return useCallback(
    async (
      subject: ComAtprotoModerationCreateReport.InputSchema["subject"],
      reasonType: string,
      reason?: string,
    ) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }

      try {
        const response = await pdsAgent.com.atproto.moderation.createReport(
          {
            reasonType,
            reason,
            subject: subject,
          },
          {
            headers: {
              // "atproto-proxy": `${userDID}#atproto_labeler`,
            },
          },
        );

        return response;
      } catch (error) {
        console.error("Failed to submit report:", error);
        throw error;
      }
    },
    [pdsAgent, userDID],
  );
};

export const useReportChatMessage = () => {
  const submitReport = useSubmitReport();

  return useCallback(
    async (
      message: ChatMessageViewHydrated,
      reasonType: string,
      reason?: string,
    ) => {
      const reportSubject = {
        $type: "com.atproto.repo.strongRef",
        uri: message.uri,
        cid: message.cid,
      };

      return await submitReport(reportSubject, reasonType, reason);
    },
    [submitReport],
  );
};

export const usePinChatMessage = () => {
  const agent = usePDSAgent();

  return async (
    messageUri: string,
    streamerDID: string,
    expiresAt?: string,
  ) => {
    if (!agent || !agent.did) {
      throw new Error("No PDS agent or user DID found");
    }

    // If streamer, create directly
    if (agent.did === streamerDID) {
      const record = {
        $type: "place.stream.chat.pinnedRecord",
        pinnedMessage: messageUri,
        createdAt: new Date().toISOString(),
        ...(expiresAt ? { expiresAt } : {}),
      };

      const result = await agent.com.atproto.repo.createRecord({
        repo: streamerDID,
        collection: "place.stream.chat.pinnedRecord",
        record,
      });
      return result;
    }

    // Otherwise, use delegated moderation endpoint
    const result = await agent.client.call(place.stream.moderation.createPin, {
      streamer: streamerDID,
      messageUri,
      ...(expiresAt ? { expiresAt } : {}),
    } as any);
    return result;
  };
};

export const useUnpinChatMessage = () => {
  const agent = usePDSAgent();
  const store = getStoreFromContext();

  return async (pinUri: string, streamerDID: string) => {
    if (!agent || !agent.did) {
      throw new Error("No PDS agent or user DID found");
    }

    // If streamer, delete directly
    if (agent.did === streamerDID) {
      const rkey = pinUri.split("/").pop();
      if (!rkey) {
        throw new Error("Invalid pin URI");
      }

      await agent.com.atproto.repo.deleteRecord({
        repo: streamerDID,
        collection: "place.stream.chat.pinnedRecord",
        rkey,
      });
      // Optimistically clear the pinned comment
      store.setState({ pinnedComment: null });
      return;
    }

    // Otherwise, use delegated moderation endpoint
    await agent.client.call(place.stream.moderation.deletePin, {
      streamer: streamerDID,
      pinUri,
    } as any);
    // Optimistically clear the pinned comment
    store.setState({ pinnedComment: null });
  };
};
