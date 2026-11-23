import { expect, test } from "@playwright/test";
import {
  createDecipheriv,
  createECDH,
  hkdfSync,
  randomBytes,
} from "node:crypto";
import { once } from "node:events";
import { createServer } from "node:http";
import { setTimeout } from "node:timers/promises";
import { loginThroughPds } from "./login";

const station = process.env.SERVER_URL!;
const origin = process.env.SERVER_HTTPS_URL!;
const pds = process.env.PDS_HTTP_URL!;
const collection = "place.stream.livestream";
const notificationPath = "/api/notification";

test.skip(!origin || !pds, "needs the HTTPS station harness");

function subscription(endpoint: string) {
  const receiver = createECDH("prime256v1");
  const publicKey = receiver.generateKeys();
  const auth = randomBytes(16);
  return {
    token: {
      endpoint,
      keys: {
        auth: auth.toString("base64url"),
        p256dh: publicKey.toString("base64url"),
      },
    },
    // RFC 8291 + RFC 8188: decrypt the real Web Push record at the test gateway.
    decrypt(body: Buffer): unknown {
      const salt = body.subarray(0, 16);
      const offset = 21 + body.readUInt8(20);
      const sender = body.subarray(21, offset);
      const ikm = hkdfSync(
        "sha256",
        receiver.computeSecret(sender),
        auth,
        Buffer.concat([Buffer.from("WebPush: info\0"), publicKey, sender]),
        32,
      );
      const key = Buffer.from(
        hkdfSync("sha256", ikm, salt, "Content-Encoding: aes128gcm\0", 16),
      );
      const nonce = Buffer.from(
        hkdfSync("sha256", ikm, salt, "Content-Encoding: nonce\0", 12),
      );
      const cipher = createDecipheriv("aes-128-gcm", key, nonce);
      cipher.setAuthTag(body.subarray(-16));
      const plaintext = Buffer.concat([
        cipher.update(body.subarray(offset, -16)),
        cipher.final(),
      ]);
      return JSON.parse(
        plaintext.subarray(0, plaintext.lastIndexOf(2)).toString(),
      );
    },
  };
}

test("the first livestream pushes only to authenticated station admins", async ({
  page,
  request,
}) => {
  const label = `push-${randomBytes(6).toString("hex")}`;
  const handle = `${label}.${new URL(process.env.PDS_HTTPS_URL!).hostname}`;
  const account = await request.post(
    `${pds}/xrpc/com.atproto.server.createAccount`,
    {
      data: {
        handle,
        email: `${label}@example.com`,
        password: process.env.ACCOUNT_PASSWORD,
      },
    },
  );
  await expect(account).toBeOK();
  const { did, accessJwt } = await account.json();
  const latest = `${station}/api/livestream/${did}`;
  // Sync the empty repo before writing so the create arrives live, not in backfill.
  await expect(
    await request.get(`${station}/api/bluesky/resolve/${did}`),
  ).toBeOK();
  expect((await request.get(latest)).status()).toBe(404);

  const received: { path: string | undefined; body: Buffer }[] = [];
  const gateway = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(chunk);
    received.push({ path: req.url, body: Buffer.concat(chunks) });
    res.writeHead(201).end();
  });
  gateway.listen(0, "127.0.0.1");
  await once(gateway, "listening");
  const address = gateway.address();
  if (!address || typeof address === "string")
    throw new Error("missing push gateway port");
  const endpoint = `http://127.0.0.1:${address.port}`;
  const admin = subscription(`${endpoint}/admin`);
  const unsigned = subscription(`${endpoint}/unverified`);
  const rkeys: string[] = [];
  const repo = (method: string, data: object) =>
    request.post(`${pds}/xrpc/com.atproto.repo.${method}`, {
      headers: { Authorization: `Bearer ${accessJwt}` },
      data: { repo: did, collection, ...data },
    });
  const writeLivestream = async (title: string, createdAt: string) => {
    const response = await repo("createRecord", {
      record: { $type: collection, title, createdAt },
    });
    await expect(response).toBeOK();
    const { uri } = (await response.json()) as { uri: string };
    rkeys.push(uri.slice(uri.lastIndexOf("/") + 1));
    await expect
      .poll(async () => {
        const indexed = await request.get(latest);
        return indexed.ok() ? (await indexed.json()).uri : null;
      })
      .toBe(uri);
  };

  try {
    await loginThroughPds(page);
    await page.context().grantPermissions(["notifications"], { origin });
    // Substitute the external browser push provider, not OAuth or station registration.
    await page.addInitScript((token) => {
      Object.defineProperty(PushManager.prototype, "subscribe", {
        value: async () => token,
      });
    }, admin.token);
    await page.goto(`${origin}/settings/notifications`);
    const toggle = page.getByRole("switch", { name: "Push Notifications" });
    await expect(toggle).toHaveAttribute("aria-checked", "false");
    await Promise.all([
      page.waitForResponse(
        (response) =>
          response.url() === `${origin}${notificationPath}` &&
          response.request().method() === "POST" &&
          response.ok(),
      ),
      toggle.click(),
    ]);
    await expect(
      await request.post(`${station}${notificationPath}`, {
        data: {
          token: JSON.stringify(unsigned.token),
          type: "web",
          repoDID: process.env.ACCOUNT_DID,
        },
      }),
    ).toBeOK();

    const now = Date.now();
    const title = "The debut livestream";
    await writeLivestream(title, new Date(now).toISOString());
    await expect
      .poll(() => received.some(({ path }) => path === "/admin"))
      .toBe(true);
    expect(
      admin.decrypt(received.find(({ path }) => path === "/admin")!.body),
    ).toEqual({
      title: `First livestream from @${handle}`,
      body: title,
      data: { path: `/${handle}` },
    });
    await writeLivestream(
      "Another livestream",
      new Date(Math.max(Date.now(), now + 1)).toISOString(),
    );
    // Observe the second record in the index, then allow any erroneous async push to arrive.
    await setTimeout(1_000);
    expect(received.map(({ path }) => path)).toEqual(["/admin"]);
  } finally {
    await gateway[Symbol.asyncDispose]();
    for (const rkey of rkeys)
      await expect(await repo("deleteRecord", { rkey })).toBeOK();
    for (const { token } of [admin, unsigned]) {
      await expect(
        await request.delete(`${station}${notificationPath}`, {
          data: { token: JSON.stringify(token) },
        }),
      ).toBeOK();
    }
  }
});
