import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { PlaceStreamModerationGetAuditLog } from "streamplace";
import { useAuditLogEvents } from "../livestream-store/livestream-store";
import { usePDSAgent } from "./xrpc";

export type AuditLogEntry = PlaceStreamModerationGetAuditLog.AuditLogEntry;
export type AuditLogEvent = PlaceStreamModerationGetAuditLog.AuditLogEvent;

interface UseAuditLogOptions {
  limit?: number;
  action?: string;
  moderator?: string;
}

interface UseAuditLogResult {
  logs: AuditLogEntry[];
  isLoading: boolean;
  error: string | null;
  hasMore: boolean;
  loadMore: () => void;
  refresh: () => void;
}

/**
 * Hook to fetch and paginate audit logs for the current user's stream.
 * Also subscribes to real-time WebSocket events to update the list.
 */
export function useAuditLog(
  options: UseAuditLogOptions = {},
): UseAuditLogResult {
  const agent = usePDSAgent();
  const [apiLogs, setApiLogs] = useState<AuditLogEntry[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(true);
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  // Track IDs of entries we've already seen to avoid duplicates
  const seenIdsRef = useRef<Set<number>>(new Set());

  // Get real-time WebSocket events
  let wsEvents: AuditLogEvent[] = [];
  try {
    wsEvents = useAuditLogEvents();
  } catch {
    // Not in LivestreamProvider context, that's OK - just use API data
  }

  const { limit = 50, action, moderator } = options;

  const refresh = useCallback(() => {
    setApiLogs([]);
    setCursor(undefined);
    setHasMore(true);
    setError(null);
    seenIdsRef.current.clear();
    setRefreshTrigger((prev) => prev + 1);
  }, []);

  const loadMore = useCallback(() => {
    if (!isLoading && hasMore) {
      setRefreshTrigger((prev) => prev + 1);
    }
  }, [isLoading, hasMore]);

  useEffect(() => {
    if (!agent?.did) {
      setApiLogs([]);
      setError(null);
      setHasMore(false);
      return;
    }

    const fetchLogs = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const result = await agent.place.stream.moderation.getAuditLog({
          limit,
          cursor,
          action:
            action as PlaceStreamModerationGetAuditLog.QueryParams["action"],
          moderator,
        });

        // Track seen IDs to avoid duplicates with WebSocket events
        result.data.logs.forEach((log) => seenIdsRef.current.add(log.id));

        if (cursor) {
          // Appending to existing logs
          setApiLogs((prev) => [...prev, ...result.data.logs]);
        } else {
          // Initial load or refresh
          setApiLogs(result.data.logs);
        }

        setCursor(result.data.cursor);
        setHasMore(!!result.data.cursor);
      } catch (err) {
        setError(
          `Failed to fetch audit logs: ${err instanceof Error ? err.message : "Unknown error"}`,
        );
      } finally {
        setIsLoading(false);
      }
    };

    fetchLogs();
  }, [agent?.did, limit, action, moderator, refreshTrigger]);

  // Reset when filters change
  useEffect(() => {
    setApiLogs([]);
    setCursor(undefined);
    setHasMore(true);
    seenIdsRef.current.clear();
    setRefreshTrigger((prev) => prev + 1);
  }, [action, moderator]);

  // Merge WebSocket events with API logs
  const logs = useMemo(() => {
    // Get new entries from WebSocket events that we haven't seen yet
    const newWsEntries = wsEvents
      .filter((event) => event.entry && !seenIdsRef.current.has(event.entry.id))
      .map((event) => event.entry!);

    // Merge all entries: new WebSocket entries first, then API logs
    const allEntries = [...newWsEntries, ...apiLogs];

    // Build set of deleted URIs from ALL entries (API + WebSocket)
    // A createBlock/createGate can be undone if there's no corresponding deleteBlock/deleteGate
    const deletedUris = new Set<string>();
    allEntries.forEach((entry) => {
      if (
        (entry.action === "deleteBlock" || entry.action === "deleteGate") &&
        entry.targetUri &&
        entry.success
      ) {
        deletedUris.add(entry.targetUri);
      }
    });

    // Compute canUndo for each entry based on the deletedUris set
    return allEntries.map((entry) => {
      const shouldBeUndoable =
        entry.success &&
        (entry.action === "createBlock" || entry.action === "createGate") &&
        !!entry.resultUri &&
        !deletedUris.has(entry.resultUri);

      // Only update if different from current value
      if (entry.canUndo !== shouldBeUndoable) {
        return { ...entry, canUndo: shouldBeUndoable };
      }
      return entry;
    });
  }, [wsEvents, apiLogs]);

  return {
    logs,
    isLoading,
    error,
    hasMore,
    loadMore,
    refresh,
  };
}

interface UndoModerationActionResult {
  undoBlock: (blockUri: string, streamerDid: string) => Promise<void>;
  undoGate: (gateUri: string, streamerDid: string) => Promise<void>;
  isLoading: boolean;
}

/**
 * Hook to undo moderation actions (delete blocks and gates).
 */
export function useUndoModerationAction(): UndoModerationActionResult {
  const agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const undoBlock = useCallback(
    async (blockUri: string, streamerDid: string) => {
      if (!agent?.did) {
        throw new Error("Not logged in");
      }

      setIsLoading(true);
      try {
        await agent.place.stream.moderation.deleteBlock({
          streamer: streamerDid,
          blockUri,
        });
      } finally {
        setIsLoading(false);
      }
    },
    [agent],
  );

  const undoGate = useCallback(
    async (gateUri: string, streamerDid: string) => {
      if (!agent?.did) {
        throw new Error("Not logged in");
      }

      setIsLoading(true);
      try {
        await agent.place.stream.moderation.deleteGate({
          streamer: streamerDid,
          gateUri,
        });
      } finally {
        setIsLoading(false);
      }
    },
    [agent],
  );

  return {
    undoBlock,
    undoGate,
    isLoading,
  };
}
