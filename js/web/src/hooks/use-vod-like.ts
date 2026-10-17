import { useToast } from "@/hooks/use-toast";
import { useSession } from "@/lib/session";
import { useStore } from "@/lib/store";
import { createLike, deleteLike } from "@streamplace/core";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getOptimisticLikeState } from "../components/video/vod-watch";

export type LikeRecordPage = {
  cursor?: string;
  records: Array<{ uri: string; value: unknown }>;
};

export async function findViewerLike(
  fetchPage: (cursor?: string) => Promise<LikeRecordPage>,
  subjectUri: string,
): Promise<string | null> {
  let cursor: string | undefined;
  const visited = new Set<string>();

  do {
    const page = await fetchPage(cursor);
    const match = page.records.find((record) => {
      const value = record.value as { subject?: unknown };
      return value?.subject === subjectUri;
    });
    if (match) return match.uri;

    cursor = page.cursor;
    if (!cursor || visited.has(cursor)) return null;
    visited.add(cursor);
  } while (cursor);

  return null;
}

export function useVodLike({
  subjectUri,
  initialCount,
}: {
  subjectUri: string;
  initialCount: number;
}) {
  const { pdsAgent, did } = useSession();
  const { t } = useTranslation("common");
  const openLoginModal = useStore((state) => state.openLoginModal);
  const { show: showToast } = useToast();
  const [count, setCount] = useState(initialCount);
  const [viewerLikeUri, setViewerLikeUri] = useState<string | null | undefined>(
    undefined,
  );
  const [liked, setLiked] = useState(false);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    setCount(initialCount);
  }, [initialCount, subjectUri]);

  useEffect(() => {
    let cancelled = false;

    if (!pdsAgent || !did) {
      setViewerLikeUri(null);
      setLiked(false);
      return;
    }

    setViewerLikeUri(undefined);
    setLiked(false);
    findViewerLike(async (cursor) => {
      const response = await pdsAgent.com.atproto.repo.listRecords({
        repo: did,
        collection: "place.stream.like",
        limit: 100,
        ...(cursor ? { cursor } : {}),
      });
      return {
        cursor: response.data.cursor,
        records: response.data.records,
      };
    }, subjectUri)
      .then((uri) => {
        if (!cancelled) {
          setViewerLikeUri(uri);
          setLiked(uri !== null);
        }
      })
      .catch((error) => {
        console.error("Failed to load viewer like", error);
        if (!cancelled) {
          setViewerLikeUri(null);
          setLiked(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [did, pdsAgent, subjectUri]);

  const toggle = useCallback(async () => {
    if (!pdsAgent || !did) {
      openLoginModal();
      return;
    }
    if (pending || viewerLikeUri === undefined) return;

    const previous = {
      liked,
      count,
      uri: viewerLikeUri,
    };
    const optimistic = getOptimisticLikeState(previous);
    setCount(optimistic.count);
    setLiked(optimistic.liked);
    setPending(true);

    try {
      if (previous.liked && previous.uri) {
        await deleteLike(pdsAgent, did, previous.uri);
        setViewerLikeUri(null);
      } else {
        const response = await createLike(pdsAgent, did, subjectUri);
        setViewerLikeUri(response.data.uri);
      }
    } catch (error) {
      setCount(previous.count);
      setViewerLikeUri(previous.uri);
      setLiked(previous.liked);
      showToast(t("like-update-failed"), t("like-state-restored"), {
        variant: "error",
      });
    } finally {
      setPending(false);
    }
  }, [
    count,
    did,
    liked,
    openLoginModal,
    pdsAgent,
    pending,
    showToast,
    subjectUri,
    t,
    viewerLikeUri,
  ]);

  return {
    count,
    liked,
    loading: viewerLikeUri === undefined || pending,
    toggle,
  };
}
