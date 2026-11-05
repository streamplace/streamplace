import { expect, test } from "@playwright/test";

// Protocol-only flow for third-party websocket consumers: the nested author on
// livestream, chat, reply and pinned views carries the account's indexed
// Bluesky displayName, pronouns and avatar, live and in the reconnect
// snapshot. There is no UI, so no native counterpart. The PDS, firehose index
// and /api/websocket are all real; nothing is substituted.
const SERVER_URL = process.env.SERVER_URL!;
const PDS = process.env.PDS_HTTP_URL!;
const DID = process.env.ACCOUNT_DID!;
const HANDLE = process.env.ACCOUNT_HANDLE!;
const WS_URL = `${SERVER_URL.replace(/^http/, "ws")}/api/websocket/${DID}`;

const PROFILE = "app.bsky.actor.profile";
const MESSAGE = "place.stream.chat.message";
const PIN = "place.stream.chat.pinnedRecord";

// A valid 1x1 PNG.
const AVATAR_PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
  "base64",
);

type Frame = Record<string, any>;

type ProfileFields = {
  displayName?: string;
  pronouns?: string;
  avatar?: string;
};

const avatarUrl = (cid: string) =>
  `https://cdn.bsky.app/img/avatar/plain/${DID}/${cid}@jpeg`;

// The author fields a profile record should surface; the node omits an empty
// displayName and any missing pronouns or avatar.
const profileFields = (record: Frame): ProfileFields => ({
  displayName: record.displayName || undefined,
  pronouns: record.pronouns,
  avatar: record.avatar ? avatarUrl(record.avatar.ref.$link) : undefined,
});

const isProfile = (frame: Frame) =>
  frame.$type === "app.bsky.actor.defs#profileViewBasic" && frame.did === DID;
const isLivestream = (frame: Frame) =>
  frame.$type === "place.stream.livestream#livestreamView";
const isMessage = (text: string) => (frame: Frame) =>
  frame.$type === "place.stream.chat.defs#messageView" &&
  frame.record?.text === text;
const isPin = (text: string) => (frame: Frame) =>
  frame.$type === "place.stream.chat.defs#pinnedRecordView" &&
  frame.message?.record?.text === text;

async function openSocket() {
  const socket = new WebSocket(WS_URL);
  const frames: Frame[] = [];
  socket.addEventListener("message", (event) => {
    const parsed = JSON.parse(String(event.data));
    frames.push(...(Array.isArray(parsed) ? parsed : [parsed]));
  });
  await new Promise<void>((resolve, reject) => {
    socket.addEventListener("open", () => resolve(), { once: true });
    socket.addEventListener(
      "error",
      () => reject(new Error(`could not open ${WS_URL}`)),
      { once: true },
    );
  });
  return {
    close: () => socket.close(),
    // Frames only accumulate, so a match stays a match.
    async frame(
      description: string,
      predicate: (frame: Frame) => boolean,
      timeout = 30_000,
    ) {
      await expect
        .poll(() => frames.find(predicate), { message: description, timeout })
        .toBeDefined();
      return frames.find(predicate)!;
    },
  };
}

// The standalone profile is sent once per connection, so reconnect until the
// index has caught up with the PDS.
async function waitForIndexedProfile(want: ProfileFields) {
  await expect(async () => {
    const socket = await openSocket();
    try {
      const profile = await socket.frame("standalone profile", isProfile, 5000);
      const { displayName, pronouns, avatar } = profile;
      expect({ displayName, pronouns, avatar }).toEqual(want);
    } finally {
      socket.close();
    }
  }).toPass({ timeout: 45_000 });
}

function expectAuthor(author: Frame, want: ProfileFields) {
  expect(author).toMatchObject({ did: DID, handle: HANDLE, ...want });
}

test("websocket-authors: author views carry the indexed Bluesky profile live and on reconnect", async ({
  request,
}) => {
  test.setTimeout(180_000);

  const session = await request.post(
    `${PDS}/xrpc/com.atproto.server.createSession`,
    { data: { identifier: DID, password: process.env.ACCOUNT_PASSWORD! } },
  );
  expect(session.ok(), await session.text()).toBe(true);
  const headers = {
    Authorization: `Bearer ${(await session.json()).accessJwt}`,
  };
  const post = async (
    method: string,
    data: Frame | Buffer,
    extraHeaders: Record<string, string> = {},
  ) => {
    const response = await request.post(`${PDS}/xrpc/${method}`, {
      headers: { ...headers, ...extraHeaders },
      data,
    });
    expect(response.ok(), `${method}: ${await response.text()}`).toBe(true);
    return response.json();
  };
  const putProfile = (record: Frame) =>
    post("com.atproto.repo.putRecord", {
      repo: DID,
      collection: PROFILE,
      rkey: "self",
      record,
    });

  const original = await request.get(
    `${PDS}/xrpc/com.atproto.repo.getRecord?repo=${DID}&collection=${PROFILE}&rkey=self`,
  );
  const saved = await original.json();
  if (!original.ok()) expect(saved.error).toBe("RecordNotFound");
  const base: Frame = original.ok() ? saved.value : { $type: PROFILE };

  const created: { collection: string; uri: string }[] = [];
  const create = async (collection: string, record: Frame) => {
    const result = await post("com.atproto.repo.createRecord", {
      repo: DID,
      collection,
      record: { $type: collection, ...record },
    });
    created.push({ collection, uri: result.uri });
    return result as { uri: string; cid: string };
  };

  try {
    try {
      const { blob } = await post("com.atproto.repo.uploadBlob", AVATAR_PNG, {
        "Content-Type": "image/png",
      });
      const stamp = Date.now();
      const want: ProfileFields = {
        displayName: `Socket Author ${stamp}`,
        pronouns: "they/them",
        avatar: avatarUrl(blob.ref.$link),
      };
      await putProfile({
        ...base,
        displayName: want.displayName,
        pronouns: want.pronouns,
        avatar: blob,
      });
      await waitForIndexedProfile(want);

      const rootText = `socket authors root ${stamp}`;
      const replyText = `socket authors reply ${stamp}`;

      // Live socket: the initial livestream snapshot, then chat, reply and pin
      // as the node indexes them from the PDS.
      const live = await openSocket();
      try {
        const livestream = await live.frame("livestream", isLivestream);
        expectAuthor(livestream.author, want);

        const root = await create(MESSAGE, {
          text: rootText,
          createdAt: new Date().toISOString(),
          streamer: DID,
        });
        const liveRoot = await live.frame("live chat", isMessage(rootText));
        expectAuthor(liveRoot.author, want);

        const reply = await create(MESSAGE, {
          text: replyText,
          createdAt: new Date().toISOString(),
          streamer: DID,
          reply: { root, parent: root },
        });
        const liveReply = await live.frame("live reply", isMessage(replyText));
        expectAuthor(liveReply.author, want);
        expect(liveReply.replyTo.uri).toBe(root.uri);
        expectAuthor(liveReply.replyTo.author, want);

        await create(PIN, {
          pinnedMessage: reply.uri,
          pinnedBy: DID,
          createdAt: new Date().toISOString(),
        });
        const livePin = await live.frame("live pin", isPin(replyText));
        expectAuthor(livePin.message.author, want);
        expectAuthor(livePin.message.replyTo.author, want);
      } finally {
        live.close();
      }

      // A fresh connection gets the same authors from the stored snapshot.
      const snapshot = await openSocket();
      try {
        const livestream = await snapshot.frame(
          "backfilled livestream",
          isLivestream,
        );
        expectAuthor(livestream.author, want);

        const root = await snapshot.frame(
          "backfilled chat",
          isMessage(rootText),
        );
        expectAuthor(root.author, want);

        const reply = await snapshot.frame(
          "backfilled reply",
          isMessage(replyText),
        );
        expectAuthor(reply.author, want);
        expectAuthor(reply.replyTo.author, want);

        const pin = await snapshot.frame("backfilled pin", isPin(replyText));
        expectAuthor(pin.message.author, want);
        expectAuthor(pin.message.replyTo.author, want);
      } finally {
        snapshot.close();
      }
    } finally {
      // Newest first: the pin before the messages it points at.
      for (const { collection, uri } of created.reverse()) {
        await post("com.atproto.repo.deleteRecord", {
          repo: DID,
          collection,
          rkey: uri.split("/").pop(),
        });
      }
    }
  } finally {
    // Profile deletions are not indexed by the node, so put the original
    // record back (an empty one when none existed) and wait for the index, so
    // later specs never see the fixture.
    await putProfile(base);
    await waitForIndexedProfile(profileFields(base));
  }
});
