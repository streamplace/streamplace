// Pure VOD interaction functions. No React imports.
//
// Each function takes a PDS agent and the user's DID as explicit
// arguments. The React hook wrappers (useCreateVodComment, etc.) live
// in @streamplace/components.
import {
  PlaceStreamGetLikes,
  PlaceStreamLike,
  PlaceStreamVodComment,
  PlaceStreamVodDefs,
  PlaceStreamVodGetComments,
  StreamplaceAgent,
} from "streamplace";

export type VodCommentHydrated = PlaceStreamVodDefs.CommentView & {
  record: PlaceStreamVodComment.Record;
};

export async function createVodComment(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  params: { text: string; video: string },
) {
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
}

export async function deleteVodComment(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  uri: string,
) {
  const rkey = uri.split("/").pop();
  if (!rkey) throw new Error("No rkey found");
  return await pdsAgent.com.atproto.repo.deleteRecord({
    repo: userDID,
    collection: "place.stream.vod.comment",
    rkey,
  });
}

export async function createLike(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  subject: string,
) {
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
}

export async function deleteLike(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  uri: string,
) {
  const rkey = uri.split("/").pop();
  if (!rkey) throw new Error("No rkey found");
  return await pdsAgent.com.atproto.repo.deleteRecord({
    repo: userDID,
    collection: "place.stream.like",
    rkey,
  });
}

export async function getVodComments(
  pdsAgent: StreamplaceAgent,
  video: string,
  limit?: number,
  cursor?: string,
): Promise<PlaceStreamVodGetComments.OutputSchema> {
  const res = await pdsAgent.place.stream.vod.getComments({
    video,
    limit,
    cursor,
  });
  return res.data;
}

export async function getLikes(
  pdsAgent: StreamplaceAgent,
  subject: string,
  limit?: number,
  cursor?: string,
): Promise<PlaceStreamGetLikes.OutputSchema> {
  const res = await pdsAgent.place.stream.getLikes({
    subject,
    limit,
    cursor,
  });
  return res.data;
}

export async function getLikeCount(
  pdsAgent: StreamplaceAgent,
  subject: string,
): Promise<number> {
  const res = await pdsAgent.place.stream.getLikes({
    subject,
    limit: 1,
  });
  return res.data.count;
}
