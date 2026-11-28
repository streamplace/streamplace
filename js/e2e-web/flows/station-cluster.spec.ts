import {
  expect,
  test,
  type APIResponse,
  type Response,
} from "@playwright/test";
import { constants } from "node:fs";
import { open, readFile, stat } from "node:fs/promises";
import { isAbsolute } from "node:path";
import { pointAppAtNode } from "../server-setup";
import { loginThroughPds } from "./login";
import { uploadFixture } from "./upload-fixture";

const ORIGIN = "place.stream.media.origin";
const BLOB = "/xrpc/place.stream.playback.getVideoBlob";
const PLAYLIST = "/xrpc/place.stream.playback.getVideoPlaylist";
const GET_RECORD = "/xrpc/com.atproto.repo.getRecord";
const WAIT = 120_000;

test.skip(
  !process.env.E2E_STATION_CLUSTER,
  "requires the station cluster harness",
);
test.describe.configure({ retries: 0 });
// Keep OAuth proofs out of browser traces and ensure the draft route is interceptable.
test.use({ trace: "off", serviceWorkers: "block" });

function required(name: string): string {
  const value = process.env[name];
  if (!value?.trim()) throw new Error(`E2E_STATION_CLUSTER requires ${name}`);
  return value;
}

function httpsOrigin(name: string): string {
  const url = new URL(required(name));
  expect(url.protocol, name).toBe("https:");
  return url.origin;
}

async function json<T>(response: APIResponse): Promise<T> {
  expect(
    response.status(),
    `${response.url()}${response.ok() ? "" : `: ${await response.text()}`}`,
  ).toBe(200);
  return (await response.json()) as T;
}

function credentialedFetch(url: string | URL, init: RequestInit) {
  // Keep credentials out of Playwright API calls and never follow redirects.
  return fetch(url, {
    ...init,
    redirect: "error",
    signal: AbortSignal.timeout(10_000),
  });
}

async function credentialedJson<T>(
  url: string | URL,
  init: RequestInit,
  status = 200,
): Promise<T> {
  const response = await credentialedFetch(url, init);
  expect(response.status, response.url).toBe(status);
  return (await response.json()) as T;
}

type StrongRef = { uri: string; cid: string };
type MuxlTrack = { $type: string; blob: string; size: number; trackId: string };
type VideoView = {
  uri: string;
  author: { did: string };
  record: { source: { $type: string; tracks: StrongRef[] } };
  tracks?: (StrongRef & { record: { track: MuxlTrack } })[];
};

function primaryTrack(
  view: VideoView,
  uri: string,
  creator: string,
): MuxlTrack {
  expect(view.uri).toBe(uri);
  expect(view.author.did).toBe(creator);
  expect(view.record.source.$type).toBe("place.stream.media.defs#sourceTracks");
  const ref = view.record.source.tracks[0];
  expect(ref?.uri).toMatch(
    new RegExp(`^at://${creator}/place\\.stream\\.media\\.track/`),
  );
  const track = view.tracks?.find(
    (t) => t.uri === ref.uri && t.cid === ref.cid,
  );
  if (!track) throw new Error(`${uri}: missing hydrated track for ${ref.uri}`);
  expect(track.record.track.$type).toBe("place.stream.media.defs#muxlTrack");
  expect(track.record.track.blob).toMatch(/^bafkr4i[a-z2-7]+$/);
  expect(track.record.track.size).toBeGreaterThan(0);
  expect(track.record.track.trackId).toMatch(/\S/);
  return track.record.track;
}

async function restartAck(path: string) {
  const text = await readFile(path, "utf8").catch(
    (error: NodeJS.ErrnoException) => {
      if (error.code !== "ENOENT") throw error;
      return '{"generation":0}';
    },
  );
  const ack = JSON.parse(text) as { generation: number; error?: string };
  expect(Number.isSafeInteger(ack.generation), path).toBe(true);
  expect(ack.error, `${path}: edge restart failed`).toBeFalsy();
  return ack.generation;
}

async function restartEdge(fifo: string, ack: string, generation: number) {
  expect(await restartAck(ack), "restart generation before command").toBe(
    generation - 1,
  );
  // A missing controller must fail rather than block waiting for a reader.
  const control = await open(fifo, constants.O_WRONLY | constants.O_NONBLOCK);
  try {
    await control.write("restart-station-edge\n");
  } finally {
    await control.close();
  }
  await expect
    .poll(() => restartAck(ack), {
      message: `edge restart ${generation}`,
      timeout: WAIT,
    })
    .toBe(generation);
}

function mp4Boxes(bytes: Buffer): string[] {
  const types: string[] = [];
  for (let offset = 0; offset < bytes.length;) {
    expect(bytes.length - offset, "MP4 box header").toBeGreaterThanOrEqual(8);
    const size = bytes.readUInt32BE(offset);
    expect(size, "MP4 box size").toBeGreaterThanOrEqual(8);
    expect(offset + size, "MP4 box fits response").toBeLessThanOrEqual(
      bytes.length,
    );
    types.push(bytes.toString("ascii", offset + 4, offset + 8));
    offset += size;
  }
  return types;
}

test("station cluster serves published media from a cold edge and honors withdrawal", async ({
  browser,
  page,
  request,
}) => {
  test.setTimeout(20 * 60_000);
  expect(required("PDS_KIND"), "private draft PDS").toBe("spaces");
  const a = httpsOrigin("SERVER_HTTPS_URL");
  const b = httpsOrigin("SERVER2_HTTPS_URL");
  const pdsHttps = httpsOrigin("PDS_HTTPS_URL");
  const aHttp = new URL(required("SERVER_URL")).origin;
  const bHttp = new URL(required("SERVER2_URL")).origin;
  const aInternal = new URL(required("SERVER_INTERNAL_URL")).origin;
  const bInternal = new URL(required("SERVER2_INTERNAL_URL")).origin;
  const pds = new URL(required("PDS_HTTP_URL")).origin;
  const creator = required("ACCOUNT_DID");
  const creatorHandle = required("ACCOUNT_HANDLE");
  const creatorPassword = required("ACCOUNT_PASSWORD");
  const station = required("E2E_STATION_DID");
  const handle = required("E2E_STATION_HANDLE");
  const password = required("E2E_STATION_PASSWORD");
  const fixture = required("E2E_FIXTURE_MP4");
  const fifo = required("E2E_CONTROL_FIFO");
  const ack = required("E2E_STATION_EDGE_RESTART");
  expect(
    new Set([aHttp, bHttp, aInternal, bInternal]).size,
    "distinct node listeners",
  ).toBe(4);
  expect(new URL(b).hostname, "edge server hostname").toBe(
    `station-edge.${new URL(pdsHttps).hostname}`,
  );
  expect(isAbsolute(ack), "E2E_STATION_EDGE_RESTART must be absolute").toBe(
    true,
  );
  expect((await stat(fifo)).isFIFO(), "E2E_CONTROL_FIFO must be a FIFO").toBe(
    true,
  );
  expect((await stat(fixture)).isFile(), "E2E_FIXTURE_MP4 must be a file").toBe(
    true,
  );
  expect(await restartAck(ack), "fresh cluster restart generation").toBe(0);

  const nodes = [a, b];
  const get = (base: string, path: string, params?: Record<string, string>) =>
    request.get(`${base}${path}`, { params, timeout: 10_000, maxRedirects: 0 });
  const serverDids = await Promise.all(
    nodes.map(async (node) => {
      const { id } = await json<{ id: string }>(
        await get(node, "/.well-known/did.json"),
      );
      expect(id, `${node}: server DID`).toBe(
        `did:web:${new URL(node).hostname}`,
      );
      const discovery = await get(node, "/.well-known/atproto-did");
      expect(discovery.status(), discovery.url()).toBe(200);
      expect((await discovery.text()).trim()).toBe(station);
      return id;
    }),
  );
  expect(
    new Set([...serverDids, station, creator]).size,
    "local, station and creator DIDs",
  ).toBe(4);

  type OriginRecord = StrongRef & {
    value: { $type: string; blob: string; size: number; mimeType: string };
  };
  const listOrigins = async (base: string, repo: string) => {
    const origins: OriginRecord[] = [];
    const seen = new Set<string>();
    let cursor: string | undefined;
    do {
      const page = await json<{ records: OriginRecord[]; cursor?: string }>(
        await get(base, "/xrpc/com.atproto.repo.listRecords", {
          repo,
          collection: ORIGIN,
          limit: "100",
          ...(cursor ? { cursor } : {}),
        }),
      );
      origins.push(...page.records);
      cursor = page.cursor;
      if (cursor) {
        expect(seen.has(cursor), `${base}: repeated origin cursor`).toBe(false);
        seen.add(cursor);
      }
    } while (cursor);
    return origins;
  };
  const videos = async (node: string) => {
    const { videos, cursor } = await json<{
      videos: { uri: string }[];
      cursor?: string;
    }>(
      await get(node, "/xrpc/place.stream.media.getVideoList", {
        repo: creator,
        limit: "100",
      }),
    );
    expect(
      cursor,
      `${node}: unexpected second page of creator videos`,
    ).toBeFalsy();
    return videos.map((video) => video.uri);
  };
  const expectNoVideos = async () => {
    await Promise.all(
      nodes.map(async (node) => {
        expect(await videos(node), `${node}: listed creator videos`).toEqual(
          [],
        );
      }),
    );
  };
  const expectNoLocalOrigins = async () => {
    await Promise.all(
      nodes.map(async (node, i) => {
        expect(
          await listOrigins(node, serverDids[i]),
          `${node}: local repo origins`,
        ).toEqual([]);
      }),
    );
  };

  // The test runner applies the project's proxy and device options to new
  // contexts; this browser already has the project's TLS launch flags.
  const stationContext = await browser.newContext({
    storageState: { cookies: [], origins: [] },
  });
  try {
    await loginThroughPds(await stationContext.newPage(), a, {
      handle,
      password,
    });
  } finally {
    await stationContext.close();
  }

  const watchPath = await uploadFixture(page, async (editor) => {
    const editorUrl = new URL(editor.url());
    expect(editorUrl.origin).toBe(a);
    const draftRkey = /^\/upload\/video\/([234567a-z]{13})\/?$/.exec(
      editorUrl.pathname,
    )?.[1];
    if (!draftRkey)
      throw new Error(`Missing draft rkey in ${editorUrl.pathname}`);
    const titleInput = editor.getByPlaceholder("Give your video a title");
    const title = await titleInput.inputValue();
    expect(title.trim(), "saved draft title").not.toBe("");
    const draftUri = `ats://${creator}/place.stream.vod.drafts/self/${creator}/place.stream.vod.draftVideo/${draftRkey}`;
    const lxm = "place.stream.vod.getDraft";
    const draftPath = `/xrpc/${lxm}`;
    const anonymous = await get(b, draftPath, { uri: draftUri });
    expect(anonymous.status(), "anonymous edge draft GET").toBe(401);

    const { accessJwt, did } = await credentialedJson<{
      accessJwt: string;
      did: string;
    }>(`${pdsHttps}/xrpc/com.atproto.server.createSession`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        identifier: creatorHandle,
        password: creatorPassword,
      }),
    });
    expect(did, "PDS session creator").toBe(creator);
    // The PDS access JWT stays on the PDS; the owner verifies the ingress audience.
    const authUrl = new URL(
      "/xrpc/com.atproto.server.getServiceAuth",
      pdsHttps,
    );
    authUrl.search = new URLSearchParams({
      aud: serverDids[1],
      lxm,
    }).toString();
    const { token } = await credentialedJson<{ token: string }>(authUrl, {
      headers: { Authorization: `Bearer ${accessJwt}` },
    });
    const draftUrl = new URL(draftPath, b);
    draftUrl.searchParams.set("uri", draftUri);
    const { error, message } = await credentialedJson<{
      error?: string;
      message?: string;
    }>(draftUrl, { headers: { Authorization: `Bearer ${token}` } }, 403);
    expect(
      /re.?authoriz/i.test(`${error} ${message}`),
      "edge draft GET with PDS service auth requires reauthorization",
    ).toBe(true);

    const matchesDraft = (url: URL) =>
      url.origin === a &&
      url.pathname === draftPath &&
      url.searchParams.size === 1 &&
      url.searchParams.get("uri") === draftUri;
    let edgeBody: Buffer | undefined;
    await editor.route(matchesDraft, async (route) => {
      const appRequest = route.request();
      if (appRequest.method() !== "GET") return route.fallback();
      const headers = { ...appRequest.headers() };
      expect(
        headers.authorization?.startsWith("DPoP ") === true && !!headers.dpop,
        "editor issued an OAuth DPoP draft GET",
      ).toBe(true);
      delete headers.host;
      // The app signs for A. Forward this attempt once over verified TLS to B
      // and relay its real response, including any nonce challenge from A.
      const upstream = await credentialedFetch(
        `${b}${draftPath}${new URL(appRequest.url()).search}`,
        { headers },
      );
      const body = Buffer.from(await upstream.arrayBuffer());
      const responseHeaders: Record<string, string> = {};
      upstream.headers.forEach((value, name) => {
        responseHeaders[name] = value;
      });
      // fetch decoded the body; Playwright will supply its new length.
      delete responseHeaders["content-encoding"];
      delete responseHeaders["content-length"];
      delete responseHeaders["transfer-encoding"];
      if (upstream.status === 200) edgeBody = body;
      await route.fulfill({
        status: upstream.status,
        headers: responseHeaders,
        body,
      });
    });
    try {
      await editor.reload();
      await expect(titleInput).toBeVisible();
      await expect(titleInput).toHaveValue(title);
      if (!edgeBody) throw new Error("No successful OAuth draft GET through B");
      const { draft } = JSON.parse(edgeBody.toString()) as {
        draft: { uri: string; record: { title: string } };
      };
      expect(draft.uri).toBe(draftUri);
      expect(draft.record.title).toBe(title);
    } finally {
      await editor.unroute(matchesDraft);
    }

    expect(
      await listOrigins(pds, station),
      "processed draft must not attest",
    ).toEqual([]);
    await expectNoVideos();
  });
  const [, author, , rkey] = watchPath.split("/");
  expect(author).toBe(creator);
  const uri = `at://${creator}/place.stream.video/${rkey}`;
  const video = async (node: string) =>
    json<VideoView>(
      await get(node, "/xrpc/place.stream.media.getVideo", { uri }),
    );
  const waitForListing = (node: string) =>
    expect
      .poll(() => videos(node), {
        message: `${node}: published ${uri}`,
        timeout: WAIT,
      })
      .toContain(uri);
  await Promise.all(nodes.map(waitForListing));
  const {
    blob: cid,
    size,
    trackId,
  } = primaryTrack(await video(a), uri, creator);
  expect(primaryTrack(await video(b), uri, creator)).toMatchObject({
    blob: cid,
    size,
    trackId,
  });

  const originParams = { repo: station, collection: ORIGIN, rkey: cid };
  const expectedOrigin = {
    uri: `at://${station}/${ORIGIN}/${cid}`,
    cid: expect.stringMatching(/^b[a-z2-7]+$/),
    value: { $type: ORIGIN, blob: cid, size, mimeType: "video/mp4" },
  };
  await expect
    .poll(() => listOrigins(pds, station), {
      message: "station PDS attestation",
      timeout: WAIT,
    })
    .toEqual([expect.objectContaining(expectedOrigin)]);
  expect(
    await json<OriginRecord>(await get(pds, GET_RECORD, originParams)),
  ).toMatchObject(expectedOrigin);
  await expectNoLocalOrigins();

  await restartEdge(fifo, ack, 1);
  await waitForListing(b);
  await pointAppAtNode(page, b);
  const isEdgeBlob = (response: Response, segment: boolean) => {
    const url = new URL(response.url());
    const blob = url.searchParams.get("cid")?.replace(/\.(m4s|mp4)$/, "");
    return (
      response.request().method() === "GET" &&
      url.origin === b &&
      url.pathname === BLOB &&
      url.searchParams.get("did") === creator &&
      !!blob &&
      (blob === cid) === segment
    );
  };
  const [init, segment] = await Promise.all([
    page.waitForResponse((response) => isEdgeBlob(response, false), {
      timeout: WAIT,
    }),
    page.waitForResponse((response) => isEdgeBlob(response, true), {
      timeout: WAIT,
    }),
    page.goto(`${b}${watchPath}`),
  ]);
  expect(init.ok(), init.url()).toBe(true);
  expect(init.headers()["content-type"]).toBe("video/mp4");
  expect(mp4Boxes(await init.body())).toEqual(
    expect.arrayContaining(["ftyp", "moov"]),
  );
  expect(segment.status(), segment.url()).toBe(206);
  expect(segment.headers()["content-type"]).toBe("video/mp4");
  expect(segment.headers()["cache-control"]).toContain("no-store");
  const range = /^bytes (\d+)-(\d+)\/(\d+)$/.exec(
    segment.headers()["content-range"] ?? "",
  );
  if (!range)
    throw new Error(`${segment.url()}: missing segment Content-Range`);
  const [, start, end, total] = range.map(Number);
  const bytes = await segment.body();
  expect(total).toBe(size);
  expect(bytes.length).toBe(end - start + 1);
  expect(mp4Boxes(bytes)).toEqual(expect.arrayContaining(["moof", "mdat"]));

  const player = page.locator("video").first();
  await expect(player).toBeVisible();
  const baseline = await player.evaluate(
    (v: HTMLVideoElement) => v.currentTime,
  );
  await expect
    .poll(
      () =>
        player.evaluate(
          (v: HTMLVideoElement, time) =>
            !v.paused && !v.seeking && v.currentTime > time + 0.5,
          baseline,
        ),
      { message: "video playback from the restarted edge", timeout: 60_000 },
    )
    .toBe(true);

  const expectWithdrawn = async () => {
    await expectNoVideos();
    await Promise.all(
      nodes.map(async (node) => {
        const playlist = await get(node, PLAYLIST, { uri });
        expect(playlist.status(), playlist.url()).toBe(404);
        expect(await playlist.json()).toMatchObject({
          message: "VideoUnavailable",
        });

        let media = await get(node, PLAYLIST, { uri, track: trackId });
        if (media.status() >= 300 && media.status() < 400) {
          // A sid-less media request can mint a session before checking withdrawal.
          expect(media.status(), media.url()).toBe(302);
          expect(await media.text(), media.url()).toBe("");
          const location = media.headers().location;
          if (!location)
            throw new Error(
              `${media.url()}: missing session redirect Location`,
            );
          const target = new URL(location, media.url());
          expect(target.origin).toBe(node);
          expect(target.pathname).toBe(PLAYLIST);
          expect(target.searchParams.size).toBe(3);
          expect(target.searchParams.get("uri")).toBe(uri);
          expect(target.searchParams.get("track")).toBe(trackId);
          expect(
            target.searchParams.get("sid"),
            "fresh playback session",
          ).toBeTruthy();
          media = await get(node, `${target.pathname}${target.search}`);
        }
        expect(media.status(), media.url()).toBe(404);
        expect(await media.json()).toMatchObject({
          message: "VideoUnavailable",
        });

        const blob = await get(node, BLOB, { cid, did: creator });
        expect(blob.status(), blob.url()).toBe(404);
        expect(await blob.json()).toMatchObject({ message: "BlobNotFound" });
      }),
    );
    const record = await get(pds, GET_RECORD, originParams);
    expect([400, 404], record.url()).toContain(record.status());
    expect(await record.json()).toMatchObject({ error: "RecordNotFound" });
    expect(
      await listOrigins(pds, station),
      "withdrawn station origins",
    ).toEqual([]);
  };

  const withdraw = await request.post(`${aInternal}/withdraw-origin`, {
    data: { cid },
    timeout: WAIT,
    maxRedirects: 0,
  });
  expect(withdraw.ok(), `${withdraw.url()}: ${await withdraw.text()}`).toBe(
    true,
  );
  await expect(expectWithdrawn).toPass({ timeout: WAIT });

  for (const internal of [aInternal, bInternal]) {
    const result = await json<{
      serverDid: string;
      scanned: number;
      indexed: number;
      errors?: string[];
    }>(
      await request.post(`${internal}/reindex-origins`, {
        timeout: WAIT,
        maxRedirects: 0,
      }),
    );
    expect(result.serverDid, internal).toBe(station);
    expect(result.errors ?? [], internal).toEqual([]);
    expect(result.scanned, internal).toBeGreaterThanOrEqual(1);
    expect(result.indexed, internal).toBe(0);
  }
  await expectWithdrawn();
  await expectNoLocalOrigins();

  await restartEdge(fifo, ack, 2);
  // Establish that the creator video and its track are indexed before
  // treating a cold edge's negative playback response as withdrawal.
  await expect(async () => {
    expect(primaryTrack(await video(b), uri, creator)).toMatchObject({
      blob: cid,
      size,
      trackId,
    });
  }).toPass({ timeout: WAIT });
  await expectWithdrawn();
  await expectNoLocalOrigins();
});
