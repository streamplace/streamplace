import {
  expect,
  request,
  test,
  type Locator,
  type Page,
} from "@playwright/test";

// Chat is an inverted list, which the web flips with scaleY(-1) and scrolls by
// hand on each wheel event. A trackpad sends many small, fractional deltas,
// and the browser snaps each scrollTop write to a device pixel: the list used
// to lose half-pixel steps in one direction, so in Chromium a two-finger
// scroll back down to the latest message stalled while scrolling up into
// history worked. Both directions must travel the full distance the wheel
// asked for. This flow also runs in Firefox, where the bug was reported (see
// playwright.config.ts).
//
// The chat is filled by writing records straight to the account's PDS, so
// this flow needs no browser login and runs the same in every browser.
const SERVER_URL = process.env.SERVER_URL;
const PDS_URL = process.env.PDS_HTTPS_URL;
const HANDLE = process.env.ACCOUNT_HANDLE!;
const DID = process.env.ACCOUNT_DID!;
const PASSWORD = process.env.ACCOUNT_PASSWORD!;

test.skip(!PDS_URL, "harness started without its HTTPS hostnames");

const MESSAGES = 30;
const STEPS = 40;
const STEP_PX = 0.5;

const scrollTop = (list: Locator) => list.evaluate((el) => el.scrollTop);

// Write chat messages to the harness account's PDS; the node picks them up
// from its firehose, as it would any viewer's.
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

async function wheelSteps(page: Page, deltaY: number, steps: number) {
  for (let i = 0; i < steps; i++) {
    await page.mouse.wheel(0, deltaY);
  }
}

test("chat-wheel-scroll: small wheel deltas scroll chat both ways", async ({
  page,
}) => {
  const run = Date.now();
  const texts = Array.from(
    { length: MESSAGES },
    (_, i) => `wheel scroll ${run} ${i}`,
  );
  await seedChat(texts);

  await page.goto(`${SERVER_URL}/`);
  await page.getByTestId("home-stream-card").first().click();
  await expect(
    page.getByText(texts[texts.length - 1], { exact: true }).first(),
  ).toBeVisible({ timeout: 30_000 });

  const list = page.getByTestId("chat-list").first();
  await expect
    .poll(() => list.evaluate((el) => el.scrollHeight - el.clientHeight))
    .toBeGreaterThan(200);

  const box = await list.boundingBox();
  expect(box).not.toBeNull();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);

  // Into history with a mouse-wheel-sized step. In an inverted list the
  // latest message sits at scrollTop 0 and history is further up.
  await page.mouse.wheel(0, -150);
  await expect.poll(() => scrollTop(list)).toBeGreaterThan(100);
  const inHistory = await scrollTop(list);
  const travel = STEPS * STEP_PX;

  // Back down toward the latest message in trackpad-sized steps.
  await wheelSteps(page, STEP_PX, STEPS);
  await expect
    .poll(async () => inHistory - (await scrollTop(list)))
    .toBeGreaterThanOrEqual(travel - 1);
  const afterDown = await scrollTop(list);
  expect(inHistory - afterDown).toBeLessThanOrEqual(travel + 1);

  // And up again by the same amount.
  await wheelSteps(page, -STEP_PX, STEPS);
  await expect
    .poll(async () => (await scrollTop(list)) - afterDown)
    .toBeGreaterThanOrEqual(travel - 1);
  expect((await scrollTop(list)) - afterDown).toBeLessThanOrEqual(travel + 1);
});
