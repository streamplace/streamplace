import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
} from "@playwright/test";
import { uploadFixture } from "./upload-fixture";

// A VOD's track record names a CID that is the BDASL of the canonical
// fragments alone, while the blob served over HTTP is
// [flat-header][fragments]. Transfer must verify the fragment stream, not the
// whole file, or it rejects every current VOD.
//
// The harness has two nodes. Upload real media through the app, then ask the
// peer (via its internal admin API) to pull the blob from the first node. The
// peer has no blob of its own, so this exercises the real network download,
// CID check, and sidecar rebuild. Only request-level coverage is possible:
// vod-transfer has no web or mobile UI.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const SERVER_URL = process.env.SERVER_URL;
const SERVER2_URL = process.env.SERVER2_URL;
const PEER_INTERNAL_URL = process.env.SERVER2_INTERNAL_URL;
const ACCOUNT_DID = process.env.ACCOUNT_DID;
const FIXTURE = process.env.E2E_FIXTURE_MP4;

test.skip(
  !HTTPS_URL ||
    !SERVER_URL ||
    !SERVER2_URL ||
    !PEER_INTERNAL_URL ||
    !ACCOUNT_DID ||
    !FIXTURE,
  "harness did not export the URLs and upload fixture this flow needs",
);

function transfer(
  request: APIRequestContext,
  uri: string,
): Promise<APIResponse> {
  return request.post(`${PEER_INTERNAL_URL}/vod-transfer`, {
    data: { source: SERVER_URL, uri },
  });
}

function delay(ms: number): Promise<void> {
  const { promise, resolve } = Promise.withResolvers<void>();
  setTimeout(resolve, ms);
  return promise;
}

test("vod-transfer: a peer pulls an uploaded VOD", async ({
  page,
  request,
}) => {
  // Upload + transcode can outlast the suite's default 90s on a loaded runner.
  test.setTimeout(15 * 60_000);

  const watchPath = await uploadFixture(page);
  const match = watchPath.match(/^\/([^/]+)\/video\/([^/]+)$/);
  expect(match).not.toBeNull();
  const uri = `at://${match![1]}/place.stream.video/${match![2]}`;

  // The peer indexes the record off the PDS firehose, so retry until it can
  // resolve the video. Failures before that point do not touch the blob store.
  const deadline = Date.now() + 2 * 60_000;
  let response = await transfer(request, uri);
  while (!response.ok() && Date.now() < deadline) {
    await delay(2_000);
    response = await transfer(request, uri);
  }
  const text = await response.text();
  expect(response.ok(), text).toBe(true);

  const body = JSON.parse(text) as {
    contentCid: string;
    downloaded: boolean;
    trackCount: number;
  };
  expect(body.contentCid).toMatch(/^bafkr4i/);
  expect(body.downloaded).toBe(true);
  expect(body.trackCount).toBeGreaterThan(0);

  // The fetched blob is now served by the peer, whole-file wrapper included.
  const blob = await request.get(
    `${SERVER2_URL}/xrpc/place.stream.playback.getVideoBlob`,
    { params: { cid: body.contentCid, did: ACCOUNT_DID as string } },
  );
  expect(blob.ok()).toBe(true);
  const bytes = await blob.body();
  expect(bytes.subarray(4, 8).toString()).toBe("ftyp");
});
