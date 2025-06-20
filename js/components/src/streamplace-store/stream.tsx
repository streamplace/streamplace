import { AppBskyFeedPost, BlobRef, RichText } from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { StreamplaceAgent } from "streamplace/src/agent";
import { PlaceStreamLivestream } from "streamplace/src/lexicons";
import { LivestreamViewHydrated } from "streamplace/src/useful-types";
import { useUrl } from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

const uploadThumbnail = async (
  pdsAgent: StreamplaceAgent,
  customThumbnail?: Blob,
) => {
  if (customThumbnail) {
    let tries = 0;
    try {
      let thumbnail = await pdsAgent.uploadBlob(customThumbnail);

      while (
        thumbnail.data.blob.size === 0 &&
        customThumbnail.size !== 0 &&
        tries < 3
      ) {
        console.warn(
          "Reuploading blob as blob sizes don't match! Blob size recieved is",
          thumbnail.data.blob.size,
          "and sent blob size is",
          customThumbnail.size,
        );
        thumbnail = await pdsAgent.uploadBlob(customThumbnail);
      }

      if (tries === 3) {
        throw new Error("Could not successfully upload blob (tried thrice)");
      }

      if (thumbnail.success) {
        console.log("Successfully uploaded thumbnail");
        return thumbnail.data.blob;
      }
    } catch (e) {
      throw new Error("Error uploading thumbnail: " + e);
    }
  }
};

async function createNewPost(
  agent: StreamplaceAgent,
  record: AppBskyFeedPost.Record,
): Promise<{ uri: string; cid: string }> {
  try {
    const post = await agent.post(record);

    return { uri: post.uri, cid: post.cid };
  } catch (error) {
    console.error("Error creating new post:", error);
    throw error;
  }
}

function buildGoLivePost(
  text: string,
  url: URL,
  profile: ProfileViewDetailed,
  params: URLSearchParams,
  thumbnail: BlobRef | undefined,
): AppBskyFeedPost.Record {
  const now = new Date();
  const linkUrl = `${url.protocol}//${url.host}/${profile.handle}?${params.toString()}`;
  const prefix = `🔴 LIVE `;
  const textUrl = `${url.protocol}//${url.host}/${profile.handle}`;
  const suffix = ` ${text}`;
  const content = prefix + textUrl + suffix;

  const rt = new RichText({ text: content });
  rt.detectFacetsWithoutResolution();
  const record: AppBskyFeedPost.Record = {
    $type: "app.bsky.feed.post",
    text: content,
    "place.stream.livestream": {
      url: linkUrl,
      title: text,
    },
    facets: rt.facets,
    createdAt: now.toISOString(),
  };
  record.embed = {
    $type: "app.bsky.embed.external",
    external: {
      description: text,
      thumb: thumbnail,
      title: `@${profile.handle} is 🔴LIVE on ${url.host}!`,
      uri: linkUrl,
    },
  };

  return record;
}

export function useCreateStreamRecord() {
  let agent = usePDSAgent();
  let url = useUrl();

  return async (
    title: string,
    customThumbnail?: Blob,
    submitPost: boolean = true,
  ) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    let thumbnail: BlobRef | undefined = undefined;

    const u = new URL(url);

    if (customThumbnail) {
      try {
        thumbnail = await uploadThumbnail(agent, customThumbnail);
      } catch (e) {
        throw new Error(`Custom thumbnail upload failed ${e}`);
      }
    } else {
      // No custom thumbnail: fetch the server-side image and upload it
      // try thrice lel
      let tries = 0;
      try {
        for (; tries < 3; tries++) {
          try {
            console.log(
              `Fetching thumbnail from ${u.protocol}//${u.host}/api/playback/${agent.did}/stream.png`,
            );
            const thumbnailRes = await fetch(
              `${u.protocol}//${u.host}/api/playback/${agent.did}/stream.png`,
            );
            if (!thumbnailRes.ok) {
              throw new Error(
                `Failed to fetch thumbnail: ${thumbnailRes.status})`,
              );
            }
            const thumbnailBlob = await thumbnailRes.blob();
            console.log(thumbnailBlob);
            thumbnail = await uploadThumbnail(agent, thumbnailBlob);
          } catch (e) {
            console.warn(
              `Failed to fetch thumbnail, retrying (${tries + 1}/3): ${e}`,
            );
            // Wait 1 second before retrying
            await new Promise((resolve) => setTimeout(resolve, 2000));
            if (tries === 2) {
              throw new Error(`Failed to fetch thumbnail after 3 tries: ${e}`);
            }
          }
        }
      } catch (e) {
        throw new Error(`Thumbnail upload failed ${e}`);
      }
    }

    let newPost: undefined | { uri: string; cid: string } = undefined;

    if (submitPost) {
      const did = agent.did;
      const profile = await agent.getProfile({ actor: did });

      if (!profile) {
        throw new Error("No profile found for the user DID");
      }

      const params = new URLSearchParams({
        did: did,
        time: new Date().toISOString(),
      });

      let post = buildGoLivePost(title, u, profile.data, params, thumbnail);

      newPost = await createNewPost(agent, post);

      if (!newPost.uri || !newPost.cid) {
        throw new Error(
          "Cannot read properties of undefined (reading 'uri' or 'cid')",
        );
      }
    }

    const record: PlaceStreamLivestream.Record = {
      title: title,
      url: url,
      createdAt: new Date().toISOString(),
      post: newPost,
      thumb: thumbnail,
    };

    await agent.com.atproto.repo.createRecord({
      repo: agent.did,
      collection: "place.stream.livestream",
      record,
    });
    return record;
  };
}

export function useUpdateStreamRecord() {
  let agent = usePDSAgent();
  let url = useUrl();

  return async (
    title: string,
    livestream: LivestreamViewHydrated | null,
    customThumbnail?: Blob,
  ) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    if (!livestream) {
      throw new Error("No latest record");
    }

    let rkey = livestream.uri.split("/").pop();
    let oldRecordValue: PlaceStreamLivestream.Record = livestream.record;

    if (!rkey) {
      throw new Error("No rkey?");
    }

    let thumbnail: BlobRef | undefined = oldRecordValue.thumb;

    // update thumbnail if a new one is provided
    if (customThumbnail) {
      try {
        thumbnail = await uploadThumbnail(agent, customThumbnail);
      } catch (e) {
        throw new Error(`Custom thumbnail upload failed ${e}`);
      }
    }

    const record: PlaceStreamLivestream.Record = {
      title: title,
      url: url,
      createdAt: new Date().toISOString(),
      post: oldRecordValue.post,
      thumb: thumbnail,
    };

    await agent.com.atproto.repo.putRecord({
      repo: agent.did,
      collection: "place.stream.livestream",
      rkey,
      record,
    });

    return record;
  };
}
