import {
  expect,
  request,
  test,
  type Locator,
  type Page,
} from "@playwright/test";

// Viewers must be able to drag-select a chat message's text. The web chat row
// used to wrap every message in a Pressable (react-native-web renders it as a
// focusable, press-handling element) and render the message body inside a
// single <Text>. Both opt their DOM subtree out of native text selection, so a
// drag over a message produced no selection at all. The row is now a plain
// View marked userSelect: text, and the body is a wrapping row, so a drag
// selects the message like any other text on the page.
//
// Native is deliberately left alone: a selectable message <Text> engages text
// selection during the row's swipe-to-reply gesture and steals focus from the
// chat input, breaking `.maestro/logged-in/chat-reply.yaml`.
//
// Chat is filled by writing records straight to the account's PDS, so this
// flow needs no browser login.
const SERVER_URL = process.env.SERVER_URL;
const PDS_URL = process.env.PDS_HTTPS_URL;
const HANDLE = process.env.ACCOUNT_HANDLE!;
const DID = process.env.ACCOUNT_DID!;
const PASSWORD = process.env.ACCOUNT_PASSWORD!;

test.skip(!PDS_URL, "harness started without its HTTPS hostnames");

async function seedChat(texts: string[]) {
  // the PDS's throwaway CA is trusted by the harness's browsers, not by Node
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  try {
    const session = await pds.post(
      `${PDS_URL}/xrpc/com.atproto.server.createSession`,
      { data: { identifier: HANDLE, password: PASSWORD } },
    );
    expect(session.ok(), `createSession: ${session.status()}`).toBe(true);
    const { accessJwt } = await session.json();
    for (const text of texts) {
      const res = await pds.post(
        `${PDS_URL}/xrpc/com.atproto.repo.createRecord`,
        {
          headers: { Authorization: `Bearer ${accessJwt}` },
          data: {
            repo: DID,
            collection: "place.stream.chat.message",
            record: {
              $type: "place.stream.chat.message",
              text,
              createdAt: new Date().toISOString(),
              streamer: DID,
            },
          },
        },
      );
      expect(res.ok(), `createRecord: ${res.status()}`).toBe(true);
    }
  } finally {
    await pds.dispose();
  }
}

// Drag across a text element's first line and return the browser selection.
async function dragSelect(page: Page, target: Locator): Promise<string> {
  await target.scrollIntoViewIfNeeded();
  const box = await target.boundingBox();
  expect(box, "selection target has a layout box").not.toBeNull();
  const y = box!.y + box!.height / 2;
  await page.mouse.move(box!.x + 1, y);
  await page.mouse.down();
  await page.mouse.move(box!.x + box!.width - 1, y, { steps: 8 });
  await page.mouse.up();
  return page.evaluate(() => window.getSelection()?.toString() ?? "");
}

test("chat-select: a message can be drag-selected into the browser selection", async ({
  page,
}) => {
  const run = Date.now();
  const message = `select me ${run}`;
  const linkText = `selectlink${run}`;
  await seedChat([message, `${linkText} https://example.com/${run}`]);

  await page.goto(`${SERVER_URL}/`);
  await page.getByTestId("home-stream-card").first().click();

  const body = page.getByText(message, { exact: true }).first();
  await expect(body).toBeVisible({ timeout: 30_000 });

  // Dragging across the body selects the message text.
  const selected = await dragSelect(page, body);
  expect(selected).toContain(message);

  // Regression for the body wrapper: it used to be a single inline <Text>.
  // Replacing it with a default (column) View stacked the handle, colon and
  // message vertically; the row keeps them on one line.
  const list = page.getByTestId("chat-list").first();
  const handle = list.getByText(`@${HANDLE}`, { exact: true }).last();
  const bodyBox = await body.boundingBox();
  const handleBox = await handle.boundingBox();
  expect(bodyBox).not.toBeNull();
  expect(handleBox).not.toBeNull();
  expect(
    Math.abs(bodyBox!.y - handleBox!.y),
    "handle and body share one line",
  ).toBeLessThan(bodyBox!.height + 2);

  // A message with a link facet renders as several segments (the link text and
  // its URI); the whole line stays selectable, the way the old single-<Text>
  // body was. Select the row's contents through the browser Selection API and
  // check both the link text and its URI come back together.
  const acrossSegments = await page.evaluate(
    ({ m, h }) => {
      const chatList = document.querySelector('[data-testid="chat-list"]')!;
      const scope = Array.from(chatList.querySelectorAll("*")).find(
        (e) =>
          e.children.length > 0 &&
          (e.textContent ?? "").includes(m) &&
          (e.textContent ?? "").includes(h),
      ) as HTMLElement | undefined;
      if (!scope) return "";
      const range = document.createRange();
      range.selectNodeContents(scope);
      const sel = window.getSelection()!;
      sel.removeAllRanges();
      sel.addRange(range);
      return sel.toString();
    },
    { m: linkText, h: `https://example.com/${run}` },
  );
  expect(acrossSegments).toContain(linkText);
  expect(acrossSegments).toContain("example.com");

  // The selection lives inside the chat list, so the surrounding page chrome
  // is not swept into it.
  const inChat = await page.evaluate(() => {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) return false;
    const chatList = document.querySelector('[data-testid="chat-list"]');
    return !!chatList && chatList.contains(sel.getRangeAt(0).startContainer);
  });
  expect(inChat).toBe(true);
});
