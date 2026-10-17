import { useCallback, useEffect, useState } from "react";
import { place } from "streamplace";
import { usePDSAgent } from "./xrpc";

/** One place.stream.chat.access record in the streamer's repo. */
export interface ChatAccessRuleRecord {
  uri: string;
  cid: string;
  rkey: string;
  value: place.stream.chat.access.Main;
}

export type ChatAccessSubjectInput =
  | { type: "verifier"; did: string }
  | { type: "label"; labeler: string; value: string };

/** Lists the current user's chat access rules, oldest first. */
export function useListChatAccessRules() {
  const agent = usePDSAgent();
  const [rules, setRules] = useState<ChatAccessRuleRecord[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [refreshTrigger, setRefreshTrigger] = useState(0);
  const refresh = useCallback(() => setRefreshTrigger((n) => n + 1), []);

  useEffect(() => {
    if (!agent?.did) {
      setRules([]);
      setError(null);
      return;
    }
    let cancelled = false;
    (async () => {
      setIsLoading(true);
      setError(null);
      try {
        const result = await agent.client.list(place.stream.chat.access, {
          repo: agent.did! as any,
        });
        if (cancelled) return;
        const records = result.records.map((record: any) => ({
          uri: record.uri,
          cid: record.cid || "",
          rkey: record.uri.split("/").pop() || "",
          value: record.value,
        }));
        records.sort((a, b) =>
          (a.value.createdAt || "").localeCompare(b.value.createdAt || ""),
        );
        setRules(records);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "Failed to load rules");
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [agent, refreshTrigger]);

  return { rules, isLoading, error, refresh };
}

/** Resolves a handle (with or without @) to a DID; DIDs pass through. */
async function resolveDID(
  agent: NonNullable<ReturnType<typeof usePDSAgent>>,
  input: string,
) {
  const v = input.trim().replace(/^@/, "");
  if (v.startsWith("did:")) return v;
  try {
    const resolved = await agent.com.atproto.identity.resolveHandle({
      handle: v,
    });
    return resolved.data.did;
  } catch {
    throw new Error(`Invalid DID or handle: ${input}`);
  }
}

/** Creates one rule in the current user's repo. */
export function useAddChatAccessRule() {
  const agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const addRule = async (
    action: "allow" | "deny",
    subject: ChatAccessSubjectInput,
  ) => {
    if (!agent?.did) throw new Error("Not logged in");
    setIsLoading(true);
    try {
      let subjectValue: any;
      if (subject.type === "verifier") {
        subjectValue = {
          $type: "place.stream.chat.access#verifier",
          did: await resolveDID(agent, subject.did),
        };
      } else {
        const value = subject.value.trim();
        if (!value) throw new Error("A label value is required");
        subjectValue = {
          $type: "place.stream.chat.access#label",
          labeler: await resolveDID(agent, subject.labeler),
          value,
        };
      }
      const record = {
        action,
        subject: subjectValue,
        createdAt: new Date().toISOString() as any,
      };
      return await agent.client.create(place.stream.chat.access, record, {
        repo: agent.did as any,
      });
    } finally {
      setIsLoading(false);
    }
  };

  return { addRule, isLoading };
}

/** Deletes one rule by rkey. */
export function useRemoveChatAccessRule() {
  const agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const removeRule = async (rkey: string) => {
    if (!agent?.did) throw new Error("Not logged in");
    setIsLoading(true);
    try {
      await agent.client.delete(place.stream.chat.access, {
        repo: agent.did as any,
        rkey,
      });
    } finally {
      setIsLoading(false);
    }
  };

  return { removeRule, isLoading };
}
