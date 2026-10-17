// React hook wrappers for the pure VOD interaction functions in
// @streamplace/core. The hooks pull pdsAgent and userDID from the
// streamplace store and call the pure functions.
import {
  createLike as createLikeCore,
  createVodComment as createVodCommentCore,
  deleteLike as deleteLikeCore,
  deleteVodComment as deleteVodCommentCore,
  getLikeCount as getLikeCountCore,
  getLikes as getLikesCore,
  getVodComments as getVodCommentsCore,
} from "@streamplace/core";
import { useCallback } from "react";
import { useDID } from "../streamplace-store";
import { usePDSAgent } from "../streamplace-store/xrpc";

export const useCreateVodComment = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();
  return useCallback(
    (params: { text: string; video: string }) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }
      return createVodCommentCore(pdsAgent, userDID, params);
    },
    [pdsAgent, userDID],
  );
};

export const useDeleteVodComment = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();
  return useCallback(
    (uri: string) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }
      return deleteVodCommentCore(pdsAgent, userDID, uri);
    },
    [pdsAgent, userDID],
  );
};

export const useCreateLike = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();
  return useCallback(
    (subject: string) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }
      return createLikeCore(pdsAgent, userDID, subject);
    },
    [pdsAgent, userDID],
  );
};

export const useDeleteLike = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();
  return useCallback(
    (uri: string) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }
      return deleteLikeCore(pdsAgent, userDID, uri);
    },
    [pdsAgent, userDID],
  );
};

export const useGetVodComments = () => {
  const pdsAgent = usePDSAgent();
  return useCallback(
    (video: string, limit?: number, cursor?: string) => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      return getVodCommentsCore(pdsAgent, video, limit, cursor);
    },
    [pdsAgent],
  );
};

export const useGetLikes = () => {
  const pdsAgent = usePDSAgent();
  return useCallback(
    (subject: string, limit?: number, cursor?: string) => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      return getLikesCore(pdsAgent, subject, limit, cursor);
    },
    [pdsAgent],
  );
};

export const useLikeCount = () => {
  const pdsAgent = usePDSAgent();
  return useCallback(
    (subject: string) => {
      if (!pdsAgent) {
        throw new Error("No PDS agent found");
      }
      return getLikeCountCore(pdsAgent, subject);
    },
    [pdsAgent],
  );
};
