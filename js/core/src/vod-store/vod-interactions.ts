// Pure VOD interaction functions. No React imports.
//
// Each function takes a PDS agent and the user's DID as explicit
// arguments. The React hook wrappers (useCreateVodComment, etc.) live
// in @streamplace/components.
import { place, StreamplaceAgent } from "streamplace";

export type VodCommentHydrated = place.stream.vod.defs.CommentView & {
  record: place.stream.vod.comment.Main;
};

export async function createVodComment(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  params: { text: string; video: string },
) {
  const record = {
    text: params.text,
    createdAt: new Date().toISOString() as any,
    video: params.video as any,
  };

  return await pdsAgent.client.create(place.stream.vod.comment, record, {
    repo: userDID as any,
  });
}

export async function deleteVodComment(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  uri: string,
) {
  const rkey = uri.split("/").pop();
  if (!rkey) throw new Error("No rkey found");
  return await pdsAgent.client.delete(place.stream.vod.comment, {
    repo: userDID as any,
    rkey,
  });
}

export async function createLike(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  subject: string,
) {
  const record = {
    subject: subject as any,
    createdAt: new Date().toISOString() as any,
  };

  return await pdsAgent.client.create(place.stream.like, record, {
    repo: userDID as any,
  });
}

export async function deleteLike(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  uri: string,
) {
  const rkey = uri.split("/").pop();
  if (!rkey) throw new Error("No rkey found");
  return await pdsAgent.client.delete(place.stream.like, {
    repo: userDID as any,
    rkey,
  });
}

export async function getVodComments(
  pdsAgent: StreamplaceAgent,
  video: string,
  limit?: number,
  cursor?: string,
): Promise<place.stream.vod.getComments.$OutputBody> {
  const res = await pdsAgent.client.call(place.stream.vod.getComments, {
    video: video as any,
    limit,
    cursor,
  });
  return res;
}

export async function getLikes(
  pdsAgent: StreamplaceAgent,
  subject: string,
  limit?: number,
  cursor?: string,
): Promise<place.stream.getLikes.$OutputBody> {
  const res = await pdsAgent.client.call(place.stream.getLikes, {
    subject: subject as any,
    limit,
    cursor,
  });
  return res;
}

export async function getLikeCount(
  pdsAgent: StreamplaceAgent,
  subject: string,
): Promise<number> {
  const res = await pdsAgent.client.call(place.stream.getLikes, {
    subject: subject as any,
    limit: 1,
  });
  return res.count;
}
