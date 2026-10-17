// React layer for VOD interactions. Re-exports the pure types from
// @streamplace/core and the React hooks from ./hooks.
export type { VodCommentHydrated } from "@streamplace/core";
export {
  useCreateLike,
  useCreateVodComment,
  useDeleteLike,
  useDeleteVodComment,
  useGetLikes,
  useGetVodComments,
  useLikeCount,
} from "./hooks";
