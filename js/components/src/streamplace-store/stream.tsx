import { AppBskyFeedPost, BlobRef, RichText } from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { LivestreamViewHydrated, place, StreamplaceAgent } from "streamplace";
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
          // Convert @atproto/api's BlobRef class to its plain lex-JSON form
          // ({$type: "blob", ref: {$link}, ...}). The @atproto/lex client's
          // lexStringify only recognizes its OWN BlobRef class — a foreign
          // BlobRef gets serialized field-by-field, losing $type, which the
          // server then rejects as a malformed blob.
          return JSON.parse(JSON.stringify(thumbnail.data.blob));
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
    langs: ["en"],
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
    canonicalUrl,
    notificationSettings,
    idleTimeoutSeconds,
    activity,
    tags,
  }: {
    title: string;
    customThumbnail?: Blob;
    submitPost?: boolean;
    canonicalUrl?: string;
    notificationSettings?: place.stream.livestream.NotificationSettings;
    idleTimeoutSeconds?: number;
    activity?: place.stream.livestream.Main["activity"];
    tags?: string[];
  }) => {
    if (!agent) {
      throw new Error("No PDS agent found");
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
    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    const thisUrl = `${url}/${agent.did}`;

    const record: place.stream.livestream.Main = {
      $type: "place.stream.livestream",
      title: title,
      url: thisUrl as any,
      createdAt: new Date().toISOString() as any,
      lastSeenAt: new Date().toISOString() as any,
      // would match up with e.g. https://stream.place/iame.li
      canonicalUrl: canonicalUrl as any,
      // user agent style string
      // e.g. `@streamplace/components/0.1.0 (ios, 32.0)`
      agent: `@streamplace/components/${PackageJson.version} (${platform}, ${platVersion})`,
      idleTimeoutSeconds: idleTimeoutSeconds,
      activity: activity,
      tags: tags?.length ? tags : undefined,
    };

    if (notificationSettings) {
      record.notificationSettings = notificationSettings;
    }

    if (customThumbnail) {
      try {
        const thumbnail = await uploadThumbnail(agent, customThumbnail);
        record.thumb = thumbnail as any;
      } catch (e) {
        throw new Error(`Custom thumbnail upload failed ${e}`);
      }
    }

    const output = await agent.client.call(place.stream.live.startLivestream, {
      livestream: record,
      streamer: agent.did as any,
      createBlueskyPost: submitPost,
    });

    return output;
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
    activity?: place.stream.livestream.Main["activity"],
    tags?: string[],
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
    let oldRecordValue: place.stream.livestream.Main = livestream.record;

    if (!rkey) {
      throw new Error("No rkey?");
    }

    let thumbnail: BlobRef | undefined = oldRecordValue.thumb as any;

    // update thumbnail if a new one is provided
    if (customThumbnail) {
      try {
        thumbnail = await uploadThumbnail(agent, customThumbnail);
      } catch (e) {
        throw new Error(`Custom thumbnail upload failed ${e}`);
      }
    }

    const record: place.stream.livestream.Main = {
      $type: "place.stream.livestream",
      title: title,
      url: finalUrl as any,
      createdAt: new Date().toISOString() as any,
      post: oldRecordValue.post,
      thumb: thumbnail as any,
      activity: activity,
      tags: tags?.length ? tags : undefined,
    };

    await agent.client.put(place.stream.livestream, record, {
      repo: agent.did as any,
      rkey,
    });

    return record;
  };
}

export function useEndLivestream() {
  let agent = usePDSAgent();
  return async () => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    return await agent.client.call(place.stream.live.stopLivestream, {});
  };
}
