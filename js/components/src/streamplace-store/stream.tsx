import { AppBskyFeedPost, BlobRef, RichText } from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { StreamplaceAgent } from "streamplace/src/agent";
import { PlaceStreamLivestream } from "streamplace/src/lexicons";
import { LivestreamViewHydrated } from "streamplace/src/useful-types";
import { useUrl } from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

import PackageJson from "../../package.json";

import { useEffect, useRef } from "react";
import { Platform } from "react-native";
import { getBrowserName } from "../lib/browser";

const useUploadThumbnail = () => {
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    return () => {
      // On unmount, abort any ongoing upload
      abortRef.current?.abort();
    };
  }, []);

  const uploadThumbnail = async (
    pdsAgent: StreamplaceAgent,
    customThumbnail?: Blob,
  ) => {
    if (!customThumbnail) return undefined;

    abortRef.current = new AbortController();
    const { signal } = abortRef.current;

    const maxTries = 3;
    let lastError: unknown = null;

    for (let tries = 0; tries < maxTries; tries++) {
      try {
        const thumbnail = await pdsAgent.uploadBlob(customThumbnail, {
          signal,
        });
        if (
          thumbnail.success &&
          thumbnail.data.blob.size === customThumbnail.size
        ) {
          console.log("Successfully uploaded thumbnail");
          return thumbnail.data.blob;
        } else {
          console.warn(
            `Blob size mismatch (attempt ${tries + 1}): received ${thumbnail.data.blob.size}, expected ${customThumbnail.size}`,
          );
        }
      } catch (e) {
        if (signal.aborted) {
          console.warn("Upload aborted");
          return undefined;
        }
        lastError = e;
        console.warn(`Error uploading thumbnail (attempt ${tries + 1}): ${e}`);
      }
    }

    throw new Error(
      `Could not successfully upload blob after ${maxTries} attempts. Last error: ${lastError}`,
    );
  };

  return uploadThumbnail;
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

async function buildGoLivePost(
  text: string,
  url: URL,
  profile: ProfileViewDetailed,
  params: URLSearchParams,
  thumbnail: BlobRef | undefined,
  agent: StreamplaceAgent,
): Promise<AppBskyFeedPost.Record> {
  const now = new Date();
  const linkUrl = `${url.protocol}//${url.host}/${profile.handle}?${params.toString()}`;
  const prefix = `🔴 LIVE `;
  const textUrl = `${url.protocol}//${url.host}/${profile.handle}`;
  const suffix = ` ${text}`;
  const content = prefix + textUrl + suffix;

  const rt = new RichText({ text: content });
  await rt.detectFacets(agent);
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
  const uploadThumbnail = useUploadThumbnail();

  return async ({
    title,
    customThumbnail,
    submitPost,
    customUrl,
  }: {
    title: string;
    customThumbnail?: Blob;
    submitPost?: boolean;
    customUrl?: string | null;
  }) => {
    if (!submitPost) {
      submitPost = true;
    }
    if (!customUrl) {
      customUrl = null;
    }
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    // Use customUrl if provided, otherwise fall back to the store URL
    const finalUrl = customUrl || url;
    const u = new URL(finalUrl);

    let thumbnail: BlobRef | undefined = undefined;

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

    const did = agent.did;
    const profile = await agent.getProfile({ actor: did });

    if (submitPost) {
      if (!profile) {
        throw new Error("No profile found for the user DID");
      }

      const params = new URLSearchParams({
        did: did,
        time: new Date().toISOString(),
      });

      let post = await buildGoLivePost(
        title,
        u,
        profile.data,
        params,
        thumbnail,
        agent,
      );

      newPost = await createNewPost(agent, post);

      if (!newPost.uri || !newPost.cid) {
        throw new Error(
          "Cannot read properties of undefined (reading 'uri' or 'cid')",
        );
      }
    }

    let platform: string = Platform.OS;
    let platVersion: string = Platform.Version
      ? Platform.Version.toString()
      : "";
    // no Platform.Version on web, so use browser name instead
    if (
      platform === "web" &&
      typeof window !== "undefined" &&
      window.navigator
    ) {
      platVersion = getBrowserName(window.navigator.userAgent);
    }

    const record: PlaceStreamLivestream.Record = {
      title: title,
      url: finalUrl,
      createdAt: new Date().toISOString(),
      // would match up with e.g. https://stream.place/iame.li
      canonicalUrl: `${finalUrl}/${profile.data.handle}`,
      // user agent style string
      // e.g. `@streamplace/components/0.1.0 (ios, 32.0)`
      agent: `@streamplace/components/${PackageJson.version} (${platform}, ${platVersion})`,
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

export function useUpdateStreamRecord(customUrl: string | null = null) {
  let agent = usePDSAgent();
  let url = useUrl();
  const uploadThumbnail = useUploadThumbnail();

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

    // Use customUrl if provided, otherwise fall back to the store URL
    const finalUrl = customUrl || url;

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
      url: finalUrl,
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
