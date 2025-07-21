import { AppBskyGraphBlock } from "@atproto/api";
import { useState } from "react";
import { usePDSAgent } from "./xrpc";

export function useCreateBlockRecord() {
  let agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const createBlock = async (subjectDID: string) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    setIsLoading(true);
    try {
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
    } finally {
      setIsLoading(false);
    }
  };

  return { createBlock, isLoading };
}

export function useCreateHideChatRecord() {
  let agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const createHideChat = async (chatMessageUri: string) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    setIsLoading(true);
    try {
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
    } finally {
      setIsLoading(false);
    }
  };

  return { createHideChat, isLoading };
}
