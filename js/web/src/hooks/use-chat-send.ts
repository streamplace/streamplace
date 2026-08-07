// Optimistic chat send: adds a local message immediately, POSTs to PDS, rolls back on failure.
import { RichText } from "@atproto/api";
import { reduceChat, type LivestreamStore } from "@streamplace/core";
import { useCallback } from "react";
import { type ChatMessageViewHydrated } from "streamplace";
import { useSession } from "../lib/session";

const ALLOWED_FACET_TYPES = new Set([
  "app.bsky.richtext.facet#link",
  "app.bsky.richtext.facet#mention",
]);

export function useChatSend(store: LivestreamStore) {
  const { pdsAgent, did } = useSession();

  return useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      if (!trimmed) return;
      if (!pdsAgent || !did) {
        throw new Error("Sign in to send chat messages");
      }

      const streamerDid = store.getState().livestream?.author.did;
      if (!streamerDid) {
        throw new Error("No active livestream to chat on");
      }

      // Grab and clear reply context before composing the message.
      const replyTo = store.getState().replyToMessage;
      if (replyTo) {
        store.setState((s) => ({ ...s, replyToMessage: null }));
      }

      // Detect links and mentions so the message renders with rich
      // formatting for ourselves (optimistic) and for anyone who
      // receives the record afterwards.
      let facets: unknown[] | undefined;
      try {
        const rt = new RichText({ text: trimmed });
        await rt.detectFacets(pdsAgent);
        facets = rt.facets?.filter((f) =>
          f.features.every((ft: { $type: string }) =>
            ALLOWED_FACET_TYPES.has(ft.$type),
          ),
        );
      } catch {
        // Non-fatal: ship the message without facets.
        facets = undefined;
      }

      const localUri = `local-${Date.now()}`;
      const createdAt = new Date().toISOString();

      const replyRef = replyTo
        ? {
            root: { uri: replyTo.uri, cid: replyTo.cid },
            parent: { uri: replyTo.uri, cid: replyTo.cid },
          }
        : undefined;

      const localMessage: ChatMessageViewHydrated = {
        uri: localUri as any,
        cid: "",
        author: { did: did as any, handle: did as any },
        record: {
          $type: "place.stream.chat.message",
          text: trimmed,
          createdAt: createdAt as any,
          streamer: streamerDid,
          ...(replyRef ? { reply: replyRef } : {}),
          ...(facets
            ? { facets: facets as ChatMessageViewHydrated["record"]["facets"] }
            : {}),
        },
        indexedAt: createdAt as any,
        ...(replyTo
          ? {
              replyTo: {
                $type: "place.stream.chat.defs#messageView",
                uri: replyTo.uri,
                cid: replyTo.cid,
                author: replyTo.author,
                record: replyTo.record,
                indexedAt: replyTo.indexedAt,
                chatProfile: replyTo.chatProfile,
              } as ChatMessageViewHydrated["replyTo"],
            }
          : {}),
      };

      store.setState((s) => reduceChat(s, [localMessage], [], []));

      try {
        await pdsAgent.com.atproto.repo.createRecord({
          repo: did,
          collection: "place.stream.chat.message",
          record: localMessage.record,
        });
      } catch (err) {
        store.setState((s) => {
          const newIndex = { ...s.chatIndex };
          for (const [key, msg] of Object.entries(newIndex)) {
            if (msg.uri === localUri) delete newIndex[key];
          }
          return {
            ...s,
            chatIndex: newIndex,
            chat: s.chat.filter((m) => m.uri !== localUri),
          };
        });
        throw err;
      }
    },
    [pdsAgent, did, store],
  );
}
