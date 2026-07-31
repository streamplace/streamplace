import { useCallback } from "react";
import { place } from "streamplace";
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

      const record = {
        text: params.text,
        createdAt: new Date().toISOString() as any,
        video: params.video as any,
      };

      return await pdsAgent.client.create(place.stream.vod.comment, record, {
        repo: userDID as any,
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
      return await pdsAgent.client.delete(place.stream.vod.comment, {
        repo: userDID as any,
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

      const record = {
        subject: subject as any,
        createdAt: new Date().toISOString() as any,
      };

      return await pdsAgent.client.create(place.stream.like, record, {
        repo: userDID as any,
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
      return await pdsAgent.client.delete(place.stream.like, {
        repo: userDID as any,
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
    ): Promise<place.stream.vod.getComments.$OutputBody> => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      const res = await pdsAgent.client.call(place.stream.vod.getComments, {
        video: video as any,
        limit,
        cursor,
      });
      return res;
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
    ): Promise<place.stream.getLikes.$OutputBody> => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      const res = await pdsAgent.client.call(place.stream.getLikes, {
        subject: subject as any,
        limit,
        cursor,
      });
      return res;
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
      const res = await pdsAgent.client.call(place.stream.getLikes, {
        subject: subject as any,
        limit: 1,
      });
      return res.count;
    },
    [pdsAgent],
  );
};

export type VodCommentHydrated = place.stream.vod.defs.CommentView & {
  record: place.stream.vod.comment.Main;
};
