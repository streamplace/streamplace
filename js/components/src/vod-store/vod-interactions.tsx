import { useCallback } from "react";
import {
  PlaceStreamLike,
  PlaceStreamVodComment,
  PlaceStreamVodDefs,
  PlaceStreamVodGetComments,
  PlaceStreamVodGetLikes,
} from "streamplace";
import { useDID } from "../streamplace-store/streamplace-store";
import { usePDSAgent } from "../streamplace-store/xrpc";

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

export const useCreateVodLike = () => {
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

export const useDeleteVodLike = () => {
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
  const pdsAgent = usePDSAgent();

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

export const useGetVodLikes = () => {
  const pdsAgent = usePDSAgent();

  return useCallback(
    async (
      subject: string,
      limit?: number,
      cursor?: string,
    ): Promise<PlaceStreamVodGetLikes.OutputSchema> => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      const res = await pdsAgent.place.stream.vod.getLikes({
        subject,
        limit,
        cursor,
      });
      return res.data;
    },
    [pdsAgent],
  );
};

export const useVodLikeCount = () => {
  const pdsAgent = usePDSAgent();

  return useCallback(
    async (subject: string): Promise<number> => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      const res = await pdsAgent.place.stream.vod.getLikes({
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
