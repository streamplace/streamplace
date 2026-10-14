import { expect, request, test } from "@playwright/test";

// A chat row's badges sit on the same line as the handle and the colon. Both
// web chat layouts put them in a flex row, and the badge image used to pull
// itself up onto a text baseline there too: that pull was written for a row
// whose children were inline, and in a flex row, which centers its own items,
// it lifted the badge a dozen pixels above the line and made the handle and the
// colon read as a subscript of it. Badges are now centered like the handle.
//
// The harness account is the node's streamer, so a message from it carries the
// streamer badge. Chat is filled by writing records straight to the account's
// PDS, so this flow needs no browser login.
const SERVER_URL = process.env.SERVER_URL;
const PDS_URL = process.env.PDS_HTTPS_URL;
const HANDLE = process.env.ACCOUNT_HANDLE!;
const DID = process.env.ACCOUNT_DID!;
const PASSWORD = process.env.ACCOUNT_PASSWORD!;

test.skip(!PDS_URL, "harness started without its HTTPS hostnames");

// A 16px badge centered on the row's 22px line lands within a pixel of the
// handle's middle; the pull that broke it put the badge 4.3px above it.
const CENTER_TOLERANCE = 2;

test("chat-badge-alignment: a badge sits on the handle's line", async ({
  page,
}) => {
  const message = `badge line ${Date.now()}`;
  // the PDS's throwaway CA is trusted by the harness's browsers, not by Node
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  const session = await pds.post(
    `${PDS_URL}/xrpc/com.atproto.server.createSession`,
    { data: { identifier: HANDLE, password: PASSWORD } },
  );
  expect(session.ok(), `createSession: ${session.status()}`).toBe(true);
  const { accessJwt } = await session.json();
  const created = await pds.post(
    `${PDS_URL}/xrpc/com.atproto.repo.createRecord`,
    {
      headers: { Authorization: `Bearer ${accessJwt}` },
      data: {
        repo: DID,
        collection: "place.stream.chat.message",
        record: {
          $type: "place.stream.chat.message",
          text: message,
          createdAt: new Date().toISOString(),
          streamer: DID,
        },
      },
    },
  );
  expect(created.ok(), `createRecord: ${await created.text()}`).toBe(true);

  await page.goto(`${SERVER_URL}/`);
  await page.getByTestId("home-stream-card").first().click();
  await expect(page.getByText(message, { exact: true }).first()).toBeVisible({
    timeout: 30_000,
  });

  // Scope to this message's own row: the chat list carries every message
  // seeded earlier in the run.
  const list = page.getByTestId("chat-list").first();
  const trigger = list.getByTestId(`chat-profile-trigger-${HANDLE}`).first();
  const badge = trigger.getByTestId("chat-badge").first();
  const handle = trigger.getByText(`@${HANDLE}`, { exact: true });
  const colon = trigger.getByText(":", { exact: true });

  const [badgeBox, handleBox, colonBox] = await Promise.all([
    badge.boundingBox(),
    handle.boundingBox(),
    colon.boundingBox(),
  ]);
  expect(badgeBox).not.toBeNull();
  expect(handleBox).not.toBeNull();
  expect(colonBox).not.toBeNull();

  // The streamer badge is a 16px square on the handle's line.
  expect(badgeBox!.width).toBe(badgeBox!.height);
  const badgeMiddle = badgeBox!.y + badgeBox!.height / 2;
  const handleMiddle = handleBox!.y + handleBox!.height / 2;
  const colonMiddle = colonBox!.y + colonBox!.height / 2;
  expect(
    Math.abs(badgeMiddle - handleMiddle),
    "the badge is centered on the handle's line",
  ).toBeLessThanOrEqual(CENTER_TOLERANCE);
  expect(
    Math.abs(badgeMiddle - colonMiddle),
    "the badge is centered on the colon's line",
  ).toBeLessThanOrEqual(CENTER_TOLERANCE);

  // The colon still rides at the end of the handle, on the handle's line.
  expect(
    Math.abs(colonBox!.y - handleBox!.y),
    "colon shares the handle's line",
  ).toBeLessThan(handleBox!.height);
  expect(colonBox!.x).toBeGreaterThanOrEqual(
    handleBox!.x + handleBox!.width - 2,
  );

  // A badge is one more box in the row; it must not push the row past the
  // column, whose scroll container hides horizontal overflow.
  const { scrollWidth, clientWidth } = await list.evaluate((el) => ({
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
  }));
  expect(
    scrollWidth,
    "the row with a badge stays inside the column",
  ).toBeLessThanOrEqual(clientWidth + 1);

  await pds.dispose();
});
