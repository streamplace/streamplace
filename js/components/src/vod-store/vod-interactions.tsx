import { useCallback } from "react";
import {
  PlaceStreamGetLikes,
  PlaceStreamLike,
  PlaceStreamVodComment,
  PlaceStreamVodDefs,
  PlaceStreamVodGetComments,
} from "streamplace";
import { useDID } from "../streamplace-store/streamplace-store";
import {
  usePDSAgent,
  usePossiblyUnauthedPDSAgent,
} from "../streamplace-store/xrpc";

export const useCreateVodComment = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();

  return useCallback(
    async (params: { text: string; video: string }) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }

      const record: PlaceStreamVodComment.Record = {
        $type: "place.stream.vod.comment",
        text: params.text,
        createdAt: new Date().toISOString(),
        video: params.video,
      };

      return await pdsAgent.com.atproto.repo.createRecord({
        repo: userDID,
        collection: "place.stream.vod.comment",
        record,
      });
    },
    [pdsAgent, userDID],
  );
};

export const useDeleteVodComment = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();

  return useCallback(
    async (uri: string) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }
      const rkey = uri.split("/").pop();
      if (!rkey) throw new Error("No rkey found");
      return await pdsAgent.com.atproto.repo.deleteRecord({
        repo: userDID,
        collection: "place.stream.vod.comment",
        rkey,
      });
    },
    [pdsAgent, userDID],
  );
};

export const useCreateLike = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();

  return useCallback(
    async (subject: string) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }

      const record: PlaceStreamLike.Record = {
        $type: "place.stream.like",
        subject,
        createdAt: new Date().toISOString(),
      };

      return await pdsAgent.com.atproto.repo.createRecord({
        repo: userDID,
        collection: "place.stream.like",
        record,
      });
    },
    [pdsAgent, userDID],
  );
};

export const useDeleteLike = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();

  return useCallback(
    async (uri: string) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }
      const rkey = uri.split("/").pop();
      if (!rkey) throw new Error("No rkey found");
      return await pdsAgent.com.atproto.repo.deleteRecord({
        repo: userDID,
        collection: "place.stream.like",
        rkey,
      });
    },
    [pdsAgent, userDID],
  );
};

export const useGetVodComments = () => {
  // Reads must hit the streamplace node (not Bluesky's appview, which is what
  // usePDSAgent falls back to when logged out and doesn't implement this
  // method) so comments are visible to logged-out viewers.
  const pdsAgent = usePossiblyUnauthedPDSAgent();

  return useCallback(
    async (
      video: string,
      limit?: number,
      cursor?: string,
    ): Promise<PlaceStreamVodGetComments.OutputSchema> => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      const res = await pdsAgent.place.stream.vod.getComments({
        video,
        limit,
        cursor,
      });
      return res.data;
    },
    [pdsAgent],
  );
};

export const useGetLikes = () => {
  // Read against the streamplace node so counts are visible when logged out.
  const pdsAgent = usePossiblyUnauthedPDSAgent();

  return useCallback(
    async (
      subject: string,
      limit?: number,
      cursor?: string,
    ): Promise<PlaceStreamGetLikes.OutputSchema> => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      const res = await pdsAgent.place.stream.getLikes({
        subject,
        limit,
        cursor,
      });
      return res.data;
    },
    [pdsAgent],
  );
};

export const useLikeCount = () => {
  const pdsAgent = usePossiblyUnauthedPDSAgent();

  return useCallback(
    async (subject: string): Promise<number> => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      const res = await pdsAgent.place.stream.getLikes({
        subject,
        limit: 1,
      });
      return res.data.count;
    },
    [pdsAgent],
  );
};

export type VodCommentHydrated = PlaceStreamVodDefs.CommentView & {
  record: PlaceStreamVodComment.Record;
};
