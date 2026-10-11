import {
  expect,
  request,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";

// The web chat row renders the handle, the colon and the message as one inline
// flow. The trigger box around the handle could not shrink, so it stayed at its
// max-content width: a handle longer than the chat column stretched the row past
// the list, and the scroll container's overflow-x: hidden (react-native-web
// hides it for a vertical ScrollView) put the rest of the message, colon and
// all, out of reach with no way to scroll to it. The handle is now a single-line
// item that ellipsizes, and the body can shrink past its min-content width, so
// no row can exceed the column.
//
// Chat is filled by writing records straight to the account's PDS, so these
// flows need no browser login. The harness account's own handle is short, so the
// long handle needs its own author: the local PDS accepts a second account
// because handles under its service domain need no DNS.
const SERVER_URL = process.env.SERVER_URL;
const PDS_URL = process.env.PDS_HTTPS_URL;
const HANDLE = process.env.ACCOUNT_HANDLE!;
const DID = process.env.ACCOUNT_DID!;
const PASSWORD = process.env.ACCOUNT_PASSWORD!;

test.skip(!PDS_URL, "harness started without its HTTPS hostnames");

// The account's label under the PDS's service domain. It has to be long enough
// that the handle outgrows the chat column, and unique per attempt: a retry
// runs against the same PDS, where the account the previous attempt created is
// still registered. The PDS caps a service-domain label at 18 characters.
function longHandleLabel(): string {
  return `longhandle${Date.now().toString(36).slice(-6)}`;
}

async function postChatMessage(
  pds: APIRequestContext,
  jwt: string,
  repo: string,
  text: string,
) {
  const res = await pds.post(`${PDS_URL}/xrpc/com.atproto.repo.createRecord`, {
    headers: { Authorization: `Bearer ${jwt}` },
    data: {
      repo,
      collection: "place.stream.chat.message",
      record: {
        $type: "place.stream.chat.message",
        text,
        createdAt: new Date().toISOString(),
        streamer: DID,
      },
    },
  });
  expect(res.ok(), `createRecord: ${res.status()} ${await res.text()}`).toBe(
    true,
  );
  const { uri } = await res.json();
  return { rkey: String(uri).split("/").pop()! };
}

async function deleteChatMessage(
  pds: APIRequestContext,
  jwt: string,
  repo: string,
  rkey: string,
) {
  const res = await pds.post(`${PDS_URL}/xrpc/com.atproto.repo.deleteRecord`, {
    headers: { Authorization: `Bearer ${jwt}` },
    data: { repo, collection: "place.stream.chat.message", rkey },
  });
  expect(res.ok(), `deleteRecord: ${res.status()} ${await res.text()}`).toBe(
    true,
  );
}

// An account whose handle is one long, unbroken label, with one chat message
// posted from it.
//
// The harness node and its chat list last for the whole run, and the other chat
// flows locate rows by the harness account's handle, so a message from another
// author has to be deleted again when this flow is done.
async function seedLongHandleMessage(text: string) {
  // the PDS's throwaway CA is trusted by the harness's browsers, not by Node
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  const label = longHandleLabel();
  const handle = `${label}.${new URL(PDS_URL!).hostname}`;
  const created = await pds.post(
    `${PDS_URL}/xrpc/com.atproto.server.createAccount`,
    {
      data: { handle, email: `${label}@example.com`, password: PASSWORD },
    },
  );
  expect(
    created.ok(),
    `createAccount: ${created.status()} ${await created.text()}`,
  ).toBe(true);
  const { did, accessJwt } = await created.json();
  const { rkey } = await postChatMessage(pds, accessJwt, did, text);
  return {
    pds,
    handle,
    cleanup: () => deleteChatMessage(pds, accessJwt, did, rkey),
  };
}

async function seedOwnMessage(text: string) {
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  const session = await pds.post(
    `${PDS_URL}/xrpc/com.atproto.server.createSession`,
    { data: { identifier: HANDLE, password: PASSWORD } },
  );
  expect(session.ok(), `createSession: ${session.status()}`).toBe(true);
  const { accessJwt } = await session.json();
  await postChatMessage(pds, accessJwt, DID, text);
  return pds;
}

async function openStreamChat(page: Page, text: string) {
  await page.goto(`${SERVER_URL}/`);
  await page.getByTestId("home-stream-card").first().click();
  await expect(page.getByText(text, { exact: true }).first()).toBeVisible({
    timeout: 30_000,
  });
  return page.getByTestId("chat-list").first();
}

// The chat list clips horizontal overflow, so any row wider than it is
// unreachable. Every row must fit.
async function expectNoHorizontalOverflow(page: Page) {
  const { scrollWidth, clientWidth } = await page
    .getByTestId("chat-list")
    .first()
    .evaluate((el) => ({
      scrollWidth: el.scrollWidth,
      clientWidth: el.clientWidth,
    }));
  expect(
    scrollWidth,
    "chat rows stay inside the chat column",
  ).toBeLessThanOrEqual(clientWidth + 1);
}

test("chat-long-handle: a long handle ellipsizes instead of pushing the message out of view", async ({
  page,
}) => {
  const message = `long handle body ${Date.now()}`;
  const author = await seedLongHandleMessage(message);

  try {
    const list = await openStreamChat(page, message);
    await expectNoHorizontalOverflow(page);

    // The chat list carries every message seeded earlier in the run, so scope
    // to this author's own row.
    const trigger = list
      .getByTestId(`chat-profile-trigger-${author.handle}`)
      .first();
    const handle = trigger.getByText(`@${author.handle}`, { exact: true });
    const colon = trigger.getByText(":", { exact: true });
    const body = list.getByText(message, { exact: true }).first();
    const [handleBox, colonBox, bodyBox, listBox] = await Promise.all([
      handle.boundingBox(),
      colon.boundingBox(),
      body.boundingBox(),
      list.boundingBox(),
    ]);
    expect(handleBox).not.toBeNull();
    expect(colonBox).not.toBeNull();
    expect(bodyBox).not.toBeNull();
    expect(listBox).not.toBeNull();

    // The handle is clamped to one line with an ellipsis: it is wider than the
    // box it is drawn in, and that box stays inside the column.
    expect(
      await handle.evaluate((el) => getComputedStyle(el).textOverflow),
      "the handle ellipsizes",
    ).toBe("ellipsis");
    expect(
      await handle.evaluate((el) => el.scrollWidth > el.clientWidth),
      "the long handle is wider than its clamped box",
    ).toBe(true);
    expect(handleBox!.x + handleBox!.width).toBeLessThanOrEqual(
      listBox!.x + listBox!.width + 1,
    );

    // The colon still rides at the end of the handle, on the handle's line, and
    // the message body is still visible inside the column.
    expect(
      Math.abs(colonBox!.y - handleBox!.y),
      "colon shares the handle's line",
    ).toBeLessThan(handleBox!.height);
    expect(colonBox!.x).toBeGreaterThanOrEqual(
      handleBox!.x + handleBox!.width - 2,
    );
    expect(bodyBox!.x + bodyBox!.width).toBeLessThanOrEqual(
      listBox!.x + listBox!.width + 1,
    );
  } finally {
    await author.cleanup();
    await author.pds.dispose();
  }
});

test("chat-long-handle: an unbreakable word in the body stays inside the column", async ({
  page,
}) => {
  // Longer on its own than the chat column, like a long URL.
  const token = "supercalifragilisticexpialidociousandthensomemoreletters";
  const message = `body with ${token} inside`;
  const pds = await seedOwnMessage(message);

  const list = await openStreamChat(page, message);
  await expectNoHorizontalOverflow(page);

  // The word wraps mid-token rather than running past the column: every line
  // box of the message ends inside the list.
  const listBox = (await list.boundingBox())!;
  const listRight = listBox.x + listBox.width;
  const lineRights = await list
    .getByText(message, { exact: true })
    .first()
    .evaluate((el) =>
      Array.from(el.getClientRects()).map((rect) => rect.x + rect.width),
    );
  expect(lineRights.length).toBeGreaterThan(1);
  for (const right of lineRights) {
    expect(right).toBeLessThanOrEqual(listRight + 1);
  }

  await pds.dispose();
});
