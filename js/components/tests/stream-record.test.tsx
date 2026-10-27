import { act } from "react";
import { createRoot } from "react-dom/client";
import { LivestreamViewHydrated, StreamplaceAgent } from "streamplace";
import { afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { useUpdateStreamRecord } from "../src/streamplace-store/stream";

const session = vi.hoisted(() => ({
  agent: null as StreamplaceAgent | null,
  encodingWarnings: vi.spyOn(console, "warn").mockImplementation(() => {}),
}));

beforeAll(() => {
  for (const [message] of session.encodingWarnings.mock.calls) {
    expect(message).toBe(
      "[@atproto/lex-data]: Uint8Array.fromBase64 / Uint8Array.prototype.toBase64 not available in this environment. Falling back to ponyfill implementation.",
    );
  }
  session.encodingWarnings.mockRestore();
});

// Replace only the session inputs and native platform binding. Record validation,
// serialization, HTTP errors, thumbnail uploads, and the update action are real.
vi.mock("../src/streamplace-store/xrpc", () => ({
  usePDSAgent: () => session.agent,
}));
vi.mock("../src/streamplace-store/streamplace-store", () => ({
  useUrl: () => "https://node.example",
}));
vi.mock("react-native", () => ({ Platform: { OS: "web" } }));

const did = "did:plc:streamer";
const rkey = "3m5tjyvhd6k2a";
const uri = `at://${did}/place.stream.livestream/${rkey}`;
const cid = "bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku";
const nextCid = "bafyreif2pall7dybz7vecqka3zo24irdwabwdi4wc4qz5psw7rvtpe3ada";
const blobCid = "bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku";
const nextBlobCid =
  "bafkreif2pall7dybz7vecqka3zo24irdwabwdi4wc4qz5psw7rvtpe3ada";

function streamRecord() {
  return {
    $type: "place.stream.livestream" as const,
    title: "Original title",
    url: `https://node.example/${did}`,
    canonicalUrl: "https://creator.example/live",
    createdAt: "2026-10-01T12:00:00.000Z",
    lastSeenAt: "2026-10-01T12:30:00.000Z",
    agent: "creator-client/1.0",
    idleTimeoutSeconds: 0,
    notificationSettings: { pushNotification: false },
    post: { uri: `at://${did}/app.bsky.feed.post/${rkey}`, cid },
    thumb: {
      $type: "blob" as const,
      ref: { $link: blobCid },
      mimeType: "image/png",
      size: 3,
    },
    activity: {
      $type: "place.stream.defs#activityLabel" as const,
      label: "music",
    },
    tags: ["lang:en", "music"],
  };
}

let saved: ReturnType<typeof streamRecord> & { endedAt?: string };
let currentCid: string | undefined;
let writes: number;
let readFails: boolean;
let writeFails: boolean;
let uploadFails: boolean;
let afterRead: (() => void) | undefined;
let onUpload: ((init: RequestInit) => Promise<Response>) | undefined;
let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;

// A single-record HTTP repository enforces the same record-CID precondition as
// putRecord. The real client also validates its request and response schemas.
async function fetchHandler(path: string, init: RequestInit) {
  const url = new URL(path, "https://pds.example");
  if (url.pathname.endsWith("/com.atproto.repo.getRecord")) {
    expect(url.searchParams.get("repo")).toBe(did);
    expect(url.searchParams.get("collection")).toBe("place.stream.livestream");
    expect(url.searchParams.get("rkey")).toBe(rkey);
    if (readFails) {
      return Response.json({ error: "RecordNotFound" }, { status: 400 });
    }
    const response = Response.json({ uri, cid: currentCid, value: saved });
    afterRead?.();
    return response;
  }
  if (url.pathname.endsWith("/com.atproto.repo.putRecord")) {
    const body = JSON.parse(init.body as string);
    expect(body.repo).toBe(did);
    expect(body.collection).toBe("place.stream.livestream");
    expect(body.rkey).toBe(rkey);
    if (writeFails || (body.swapRecord && body.swapRecord !== currentCid)) {
      return Response.json({ error: "InvalidSwap" }, { status: 400 });
    }
    saved = body.record;
    currentCid = nextCid;
    writes++;
    return Response.json({ uri, cid: currentCid });
  }
  if (url.pathname.endsWith("/com.atproto.repo.uploadBlob")) {
    if (onUpload) return onUpload(init);
    if (uploadFails) {
      return Response.json({ error: "UploadFailed" }, { status: 500 });
    }
    return Response.json({
      blob: { ...streamRecord().thumb, ref: { $link: nextBlobCid } },
    });
  }
  throw new Error(`Unexpected request: ${path}`);
}

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  saved = streamRecord();
  currentCid = cid;
  writes = 0;
  readFails = writeFails = uploadFails = false;
  afterRead = onUpload = undefined;
  session.agent = new StreamplaceAgent({ did, fetchHandler });
  container = document.createElement("div");
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function view(record = streamRecord()): LivestreamViewHydrated {
  return {
    uri,
    cid,
    record,
    author: { did, handle: "creator.example" },
    indexedAt: record.createdAt,
  } as unknown as LivestreamViewHydrated;
}

async function updateAction(customUrl: string | null = null) {
  let update!: ReturnType<typeof useUpdateStreamRecord>;
  function Form() {
    update = useUpdateStreamRecord(customUrl);
    return null;
  }
  await act(async () => root.render(<Form />));
  return update;
}

it("preserves the latest record's untouched fields when the form saves", async () => {
  const staleView = view();
  saved.canonicalUrl = "https://creator.example/new-link";
  saved.endedAt = "2026-10-01T12:40:00.000Z";
  currentCid = nextCid;
  const latest = structuredClone(saved);
  const update = await updateAction();

  await update(
    "Edited title",
    staleView,
    undefined,
    latest.activity,
    latest.tags,
  );

  expect(saved).toEqual({ ...latest, title: "Edited title" });
  expect(staleView.record).toEqual(streamRecord());
  expect(writes).toBe(1);
});

it("still clears activity and tags when the form clears them", async () => {
  const update = await updateAction();
  await update("Edited title", view(), undefined, undefined, []);

  const { activity: _activity, tags: _tags, ...rest } = streamRecord();
  expect(saved).toEqual({ ...rest, title: "Edited title" });
});

it("applies an explicit URL and a newly uploaded thumbnail", async () => {
  const log = vi.spyOn(console, "log").mockImplementation(() => {});
  const update = await updateAction("https://creator.example/alternate");
  const original = streamRecord();
  const thumbnail = new Blob(["new"], { type: "image/png" });

  await update(
    "Edited title",
    view(),
    thumbnail,
    original.activity,
    original.tags,
  );

  expect(saved).toEqual({
    ...original,
    title: "Edited title",
    url: "https://creator.example/alternate",
    thumb: { ...original.thumb, ref: { $link: nextBlobCid } },
  });
  expect(log.mock.calls).toEqual([["Successfully uploaded thumbnail"]]);
});

it("rejects a write if the record changes after it was read", async () => {
  afterRead = () => {
    saved.endedAt = "2026-10-01T12:45:00.000Z";
    currentCid = nextCid;
  };
  const update = await updateAction();

  await expect(update("Edited title", view())).rejects.toMatchObject({
    error: "InvalidSwap",
  });

  expect(saved).toEqual({
    ...streamRecord(),
    endedAt: "2026-10-01T12:45:00.000Z",
  });
  expect(writes).toBe(0);
});

it("rejects an update when the PDS omits the record CID", async () => {
  currentCid = undefined;
  const update = await updateAction();

  await expect(update("Edited title", view())).rejects.toThrow("No record CID");

  expect(saved).toEqual(streamRecord());
  expect(writes).toBe(0);
});

it.each(["read", "write"])(
  "leaves the record unchanged on %s failure",
  async (failure) => {
    readFails = failure === "read";
    writeFails = failure === "write";
    const update = await updateAction();

    await expect(update("Edited title", view())).rejects.toMatchObject({
      error: failure === "read" ? "RecordNotFound" : "InvalidSwap",
    });

    expect(saved).toEqual(streamRecord());
    expect(writes).toBe(0);
  },
);

it("leaves the record unchanged when thumbnail upload retries fail", async () => {
  uploadFails = true;
  const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
  const update = await updateAction();

  await expect(
    update("Edited title", view(), new Blob(["new"], { type: "image/png" })),
  ).rejects.toThrow("Custom thumbnail upload failed");

  expect(saved).toEqual(streamRecord());
  expect(writes).toBe(0);
  expect(warn).toHaveBeenCalledTimes(3);
  for (const [message] of warn.mock.calls) {
    expect(message).toMatch(/^Error uploading thumbnail \(attempt [123]\):/);
  }
});

it("does not save when unmounting aborts the thumbnail upload", async () => {
  const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
  let started!: () => void;
  const uploading = new Promise<void>((resolve) => {
    started = resolve;
  });
  onUpload = (init) =>
    new Promise((_resolve, reject) => {
      init.signal!.addEventListener(
        "abort",
        () => reject(new DOMException("Aborted", "AbortError")),
        { once: true },
      );
      started();
    });
  const update = await updateAction();
  const result = update(
    "Edited title",
    view(),
    new Blob(["new"], { type: "image/png" }),
  ).then(
    () => null,
    (error: unknown) => error,
  );
  await uploading;
  await act(async () => root.unmount());
  expect(await result).toBeInstanceOf(Error);
  expect(String(await result)).toContain("Custom thumbnail upload failed");

  expect(saved).toEqual(streamRecord());
  expect(writes).toBe(0);
  expect(warn.mock.calls).toEqual([["Upload aborted"]]);
});
